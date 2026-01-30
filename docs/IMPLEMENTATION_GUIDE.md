# Implementation Guide - Jobsity Chat

## Overview

This guide provides step-by-step instructions for implementing the Jobsity Chat application based on the complete specification in `spec.json`.

## Implementation Strategy

Follow the **9-phase approach** outlined in the specification:

1. ✅ Project Setup
2. Database Layer
3. Domain & Service Layer
4. HTTP & WebSocket Layer
5. RabbitMQ Integration
6. Stock Bot
7. Frontend (Minimal)
8. Testing & Quality
9. Deployment

## Phase 1: Project Setup ✅

**Status**: Complete (spec generation phase)

**Completed**:
- [x] Project directory structure
- [x] go.mod with dependencies
- [x] Makefile for build automation
- [x] Dockerfiles (chat-server, stock-bot)
- [x] docker-compose.yml
- [x] Environment configuration (.env.example)
- [x] Database migrations
- [x] .gitignore
- [x] README.md

**Next Steps**:
1. Initialize Git repository
2. Review and customize configurations
3. Test Docker Compose setup

## Phase 2: Database Layer

### 2.1 Repository Interfaces

Create interfaces in `internal/domain/`:

```go
// internal/domain/user.go
type User struct {
    ID           string    `json:"id"`
    Username     string    `json:"username"`
    Email        string    `json:"email"`
    PasswordHash string    `json:"-"`
    CreatedAt    time.Time `json:"created_at"`
}

type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id string) (*User, error)
    GetByUsername(ctx context.Context, username string) (*User, error)
    GetByEmail(ctx context.Context, email string) (*User, error)
}
```

```go
// internal/domain/message.go
type Message struct {
    ID         string    `json:"id"`
    ChatroomID string    `json:"chatroom_id"`
    UserID     string    `json:"user_id"`
    Username   string    `json:"username"`
    Content    string    `json:"content"`
    IsBot      bool      `json:"is_bot"`
    CreatedAt  time.Time `json:"created_at"`
}

type MessageRepository interface {
    Create(ctx context.Context, message *Message) error
    GetByChatroom(ctx context.Context, chatroomID string, limit int) ([]*Message, error)
}
```

```go
// internal/domain/chatroom.go
type Chatroom struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
    CreatedBy string    `json:"created_by"`
}

type ChatroomRepository interface {
    Create(ctx context.Context, chatroom *Chatroom) error
    GetByID(ctx context.Context, id string) (*Chatroom, error)
    List(ctx context.Context) ([]*Chatroom, error)
    AddMember(ctx context.Context, chatroomID, userID string) error
    IsMember(ctx context.Context, chatroomID, userID string) (bool, error)
}
```

```go
// internal/domain/session.go
type Session struct {
    ID        string    `json:"id"`
    UserID    string    `json:"user_id"`
    Token     string    `json:"token"`
    ExpiresAt time.Time `json:"expires_at"`
    CreatedAt time.Time `json:"created_at"`
}

type SessionRepository interface {
    Create(ctx context.Context, session *Session) error
    GetByToken(ctx context.Context, token string) (*Session, error)
    Delete(ctx context.Context, token string) error
    DeleteExpired(ctx context.Context) error
}
```

### 2.2 PostgreSQL Implementation

Implement repositories in `internal/repository/postgres/`:

```go
// internal/repository/postgres/user_repository.go
type UserRepository struct {
    db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
    return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
    query := `
        INSERT INTO users (username, email, password_hash)
        VALUES ($1, $2, $3)
        RETURNING id, created_at
    `
    return r.db.QueryRowContext(ctx, query,
        user.Username, user.Email, user.PasswordHash,
    ).Scan(&user.ID, &user.CreatedAt)
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
    query := `
        SELECT id, username, email, password_hash, created_at
        FROM users
        WHERE username = $1
    `
    user := &domain.User{}
    err := r.db.QueryRowContext(ctx, query, username).Scan(
        &user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.CreatedAt,
    )
    if err == sql.ErrNoRows {
        return nil, domain.ErrUserNotFound
    }
    return user, err
}
```

### 2.3 Database Connection

Create connection manager in `internal/config/database.go`:

```go
func NewPostgresConnection(dbURL string) (*sql.DB, error) {
    db, err := sql.Open("postgres", dbURL)
    if err != nil {
        return nil, err
    }

    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)

    if err := db.Ping(); err != nil {
        return nil, err
    }

    return db, nil
}
```

### 2.4 Unit Tests

Write tests in `internal/repository/postgres/*_test.go`:

```go
func TestUserRepository_Create(t *testing.T) {
    // Use testcontainers-go for real PostgreSQL
    ctx := context.Background()

    container, err := postgres.RunContainer(ctx,
        testcontainers.WithImage("postgres:15-alpine"),
        // ... config
    )
    require.NoError(t, err)
    defer container.Terminate(ctx)

    // Get connection string
    connStr, err := container.ConnectionString(ctx)
    require.NoError(t, err)

    // Connect and test
    db, err := sql.Open("postgres", connStr)
    require.NoError(t, err)
    defer db.Close()

    repo := NewUserRepository(db)

    user := &domain.User{
        Username:     "test_user",
        Email:        "test@example.com",
        PasswordHash: "hashed_password",
    }

    err = repo.Create(ctx, user)
    assert.NoError(t, err)
    assert.NotEmpty(t, user.ID)
}
```

## Phase 3: Domain & Service Layer

### 3.1 Authentication Service

```go
// internal/service/auth_service.go
type AuthService struct {
    userRepo    domain.UserRepository
    sessionRepo domain.SessionRepository
}

func (s *AuthService) Register(ctx context.Context, username, email, password string) (*domain.User, error) {
    // Validate input
    if len(username) < 3 || len(password) < 8 {
        return nil, domain.ErrInvalidInput
    }

    // Check if username exists
    if _, err := s.userRepo.GetByUsername(ctx, username); err == nil {
        return nil, domain.ErrUsernameExists
    }

    // Hash password
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
    if err != nil {
        return nil, err
    }

    // Create user
    user := &domain.User{
        Username:     username,
        Email:        email,
        PasswordHash: string(hashedPassword),
    }

    if err := s.userRepo.Create(ctx, user); err != nil {
        return nil, err
    }

    return user, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (*domain.Session, error) {
    // Get user
    user, err := s.userRepo.GetByUsername(ctx, username)
    if err != nil {
        return nil, domain.ErrInvalidCredentials
    }

    // Verify password
    if err := bcrypt.CompareHashAndPassword(
        []byte(user.PasswordHash), []byte(password),
    ); err != nil {
        return nil, domain.ErrInvalidCredentials
    }

    // Create session
    session := &domain.Session{
        UserID:    user.ID,
        Token:     uuid.New().String(),
        ExpiresAt: time.Now().Add(24 * time.Hour),
    }

    if err := s.sessionRepo.Create(ctx, session); err != nil {
        return nil, err
    }

    return session, nil
}
```

### 3.2 Chat Service

```go
// internal/service/chat_service.go
type ChatService struct {
    messageRepo  domain.MessageRepository
    chatroomRepo domain.ChatroomRepository
}

func (s *ChatService) SendMessage(ctx context.Context, msg *domain.Message) error {
    // Validate chatroom membership
    isMember, err := s.chatroomRepo.IsMember(ctx, msg.ChatroomID, msg.UserID)
    if err != nil || !isMember {
        return domain.ErrNotMember
    }

    // Save message
    return s.messageRepo.Create(ctx, msg)
}

func (s *ChatService) GetMessages(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error) {
    return s.messageRepo.GetByChatroom(ctx, chatroomID, limit)
}
```

### 3.3 Command Parser

```go
// internal/service/command_parser.go
type Command struct {
    Type      string
    StockCode string
}

func ParseCommand(content string) (*Command, bool) {
    if !strings.HasPrefix(content, "/stock=") {
        return nil, false
    }

    stockCode := strings.TrimPrefix(content, "/stock=")
    stockCode = strings.TrimSpace(stockCode)

    if stockCode == "" || len(stockCode) > 20 {
        return nil, false
    }

    return &Command{
        Type:      "stock",
        StockCode: stockCode,
    }, true
}
```

## Phase 4: HTTP & WebSocket Layer

### 4.1 HTTP Server

```go
// cmd/chat-server/main.go
func main() {
    // Load config
    cfg := config.Load()

    // Connect to database
    db, err := config.NewPostgresConnection(cfg.DatabaseURL)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Initialize repositories
    userRepo := postgres.NewUserRepository(db)
    sessionRepo := postgres.NewSessionRepository(db)
    messageRepo := postgres.NewMessageRepository(db)
    chatroomRepo := postgres.NewChatroomRepository(db)

    // Initialize services
    authService := service.NewAuthService(userRepo, sessionRepo)
    chatService := service.NewChatService(messageRepo, chatroomRepo)

    // Initialize hub
    hub := websocket.NewHub()
    go hub.Run()

    // Initialize handlers
    authHandler := handler.NewAuthHandler(authService)
    chatroomHandler := handler.NewChatroomHandler(chatroomRepo)
    wsHandler := handler.NewWebSocketHandler(hub, chatService)

    // Setup router
    r := chi.NewRouter()

    // Middleware
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{cfg.AllowedOrigins},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
        AllowedHeaders:   []string{"Content-Type"},
        AllowCredentials: true,
    }))

    // Public routes
    r.Post("/api/v1/auth/register", authHandler.Register)
    r.Post("/api/v1/auth/login", authHandler.Login)

    // Protected routes
    r.Group(func(r chi.Router) {
        r.Use(middleware.Auth(sessionRepo))

        r.Post("/api/v1/auth/logout", authHandler.Logout)
        r.Get("/api/v1/chatrooms", chatroomHandler.List)
        r.Post("/api/v1/chatrooms", chatroomHandler.Create)
        r.Get("/api/v1/chatrooms/{id}/messages", chatroomHandler.GetMessages)
        r.Get("/ws/chat/{chatroom_id}", wsHandler.HandleConnection)
    })

    // Health checks
    r.Get("/health", handler.Health)
    r.Get("/health/ready", handler.Ready(db))

    // Start server
    log.Printf("Starting server on :%s", cfg.Port)
    if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
        log.Fatal(err)
    }
}
```

### 4.2 WebSocket Hub Pattern

```go
// internal/websocket/hub.go
type Hub struct {
    clients    map[*Client]bool
    broadcast  chan *BroadcastMessage
    register   chan *Client
    unregister chan *Client
}

type BroadcastMessage struct {
    ChatroomID string
    Message    []byte
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.clients[client] = true

        case client := <-h.unregister:
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
            }

        case message := <-h.broadcast:
            for client := range h.clients {
                if client.chatroomID == message.ChatroomID {
                    select {
                    case client.send <- message.Message:
                    default:
                        close(client.send)
                        delete(h.clients, client)
                    }
                }
            }
        }
    }
}
```

### 4.3 WebSocket Client

```go
// internal/websocket/client.go
type Client struct {
    hub        *Hub
    conn       *websocket.Conn
    send       chan []byte
    userID     string
    username   string
    chatroomID string
}

func (c *Client) ReadPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()

    c.conn.SetReadLimit(1024)

    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            break
        }

        // Parse message
        var msg struct {
            Type    string `json:"type"`
            Content string `json:"content"`
        }
        if err := json.Unmarshal(message, &msg); err != nil {
            continue
        }

        // Check if command
        if cmd, isCommand := service.ParseCommand(msg.Content); isCommand {
            // Publish to RabbitMQ
            // ...
            continue
        }

        // Save and broadcast regular message
        // ...
    }
}

func (c *Client) WritePump() {
    ticker := time.NewTicker(54 * time.Second)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()

    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }

            if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
                return
            }

        case <-ticker.C:
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}
```

## Phase 5: RabbitMQ Integration

### 5.1 RabbitMQ Connection Manager

```go
// internal/messaging/rabbitmq.go
type RabbitMQ struct {
    conn    *amqp.Connection
    channel *amqp.Channel
}

func NewRabbitMQ(url string) (*RabbitMQ, error) {
    conn, err := amqp.Dial(url)
    if err != nil {
        return nil, err
    }

    ch, err := conn.Channel()
    if err != nil {
        return nil, err
    }

    return &RabbitMQ{conn: conn, channel: ch}, nil
}

func (r *RabbitMQ) DeclareExchanges() error {
    // Declare commands exchange
    if err := r.channel.ExchangeDeclare(
        "chat.commands", // name
        "topic",         // type
        true,            // durable
        false,           // auto-deleted
        false,           // internal
        false,           // no-wait
        nil,             // arguments
    ); err != nil {
        return err
    }

    // Declare responses exchange
    return r.channel.ExchangeDeclare(
        "chat.responses",
        "fanout",
        true,
        false,
        false,
        false,
        nil,
    )
}

func (r *RabbitMQ) PublishStockCommand(ctx context.Context, cmd *StockCommand) error {
    body, err := json.Marshal(cmd)
    if err != nil {
        return err
    }

    return r.channel.PublishWithContext(
        ctx,
        "chat.commands",      // exchange
        "stock.request",      // routing key
        false,                // mandatory
        false,                // immediate
        amqp.Publishing{
            ContentType: "application/json",
            Body:        body,
        },
    )
}
```

## Phase 6: Stock Bot

### 6.1 Stooq API Client

```go
// internal/stock/stooq_client.go
type StooqClient struct {
    baseURL    string
    httpClient *http.Client
}

func (c *StooqClient) GetQuote(ctx context.Context, stockCode string) (*Quote, error) {
    url := fmt.Sprintf("%s/q/l/?s=%s&f=sd2t2ohlcv&h&e=csv", c.baseURL, stockCode)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    // Parse CSV
    reader := csv.NewReader(resp.Body)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, err
    }

    if len(records) < 2 {
        return nil, errors.New("invalid CSV response")
    }

    // Extract closing price (column 6)
    symbol := records[1][0]
    closePrice := records[1][6]

    if closePrice == "N/D" {
        return nil, errors.New("stock not found")
    }

    price, err := strconv.ParseFloat(closePrice, 64)
    if err != nil {
        return nil, err
    }

    return &Quote{
        Symbol: symbol,
        Price:  price,
    }, nil
}
```

### 6.2 Stock Bot Main

```go
// cmd/stock-bot/main.go
func main() {
    cfg := config.Load()

    // Connect to RabbitMQ
    rmq, err := messaging.NewRabbitMQ(cfg.RabbitMQURL)
    if err != nil {
        log.Fatal(err)
    }
    defer rmq.Close()

    // Initialize Stooq client
    stooqClient := stock.NewStooqClient(cfg.StooqAPIURL)

    // Consume messages
    msgs, err := rmq.Consume("stock.commands")
    if err != nil {
        log.Fatal(err)
    }

    for msg := range msgs {
        var cmd messaging.StockCommand
        if err := json.Unmarshal(msg.Body, &cmd); err != nil {
            log.Printf("Error parsing message: %v", err)
            msg.Ack(false)
            continue
        }

        // Fetch quote
        quote, err := stooqClient.GetQuote(context.Background(), cmd.StockCode)
        if err != nil {
            // Send error response
            rmq.PublishStockResponse(&messaging.StockResponse{
                ChatroomID: cmd.ChatroomID,
                Error:      err.Error(),
            })
            msg.Ack(false)
            continue
        }

        // Send success response
        response := &messaging.StockResponse{
            ChatroomID:       cmd.ChatroomID,
            Symbol:           quote.Symbol,
            Price:            quote.Price,
            FormattedMessage: fmt.Sprintf("%s quote is $%.2f per share", quote.Symbol, quote.Price),
        }

        if err := rmq.PublishStockResponse(response); err != nil {
            log.Printf("Error publishing response: %v", err)
        }

        msg.Ack(false)
    }
}
```

## Phase 7: Frontend (Minimal)

Create simple HTML/JS in `static/index.html`:

```html
<!DOCTYPE html>
<html>
<head>
    <title>Jobsity Chat</title>
    <style>
        #messages { height: 400px; overflow-y: scroll; border: 1px solid #ccc; }
        .message { padding: 5px; }
        .bot-message { background-color: #f0f0f0; }
    </style>
</head>
<body>
    <h1>Jobsity Chat</h1>
    <div id="messages"></div>
    <input type="text" id="messageInput" placeholder="Type a message or /stock=AAPL.US" />
    <button onclick="sendMessage()">Send</button>

    <script>
        const chatroomId = 'YOUR_CHATROOM_ID';
        const ws = new WebSocket(`ws://localhost:8080/ws/chat/${chatroomId}`);

        ws.onmessage = (event) => {
            const msg = JSON.parse(event.data);
            const div = document.createElement('div');
            div.className = msg.is_bot ? 'message bot-message' : 'message';
            div.textContent = `${msg.username}: ${msg.content}`;
            document.getElementById('messages').appendChild(div);
        };

        function sendMessage() {
            const input = document.getElementById('messageInput');
            ws.send(JSON.stringify({
                type: 'chat_message',
                content: input.value
            }));
            input.value = '';
        }
    </script>
</body>
</html>
```

## Phase 8: Testing & Quality

### 8.1 Unit Tests

Run tests:
```bash
make test
```

### 8.2 Integration Tests

```bash
make test-integration
```

### 8.3 E2E Testing

Test with 2 browsers:
1. Open browser 1: Register and login as user1
2. Open browser 2: Register and login as user2
3. Both join same chatroom
4. Send messages from both
5. Send `/stock=AAPL.US` from user1
6. Verify bot response appears in both browsers

### 8.4 Linting

```bash
make lint
```

## Phase 9: Deployment

### 9.1 Build Docker Images

```bash
make docker-build
```

### 9.2 Deploy with Docker Compose

```bash
make docker-run
```

### 9.3 Deploy to Kubernetes

```bash
kubectl apply -f deployments/kubernetes/
```

## Testing Checklist

- [ ] User registration works
- [ ] User login works
- [ ] WebSocket connection establishes
- [ ] Messages broadcast to all connected users
- [ ] Messages ordered by timestamp
- [ ] Only last 50 messages displayed
- [ ] `/stock=CODE` command detected
- [ ] Stock command not saved to database
- [ ] Bot receives command via RabbitMQ
- [ ] Bot fetches from Stooq API
- [ ] Bot parses CSV correctly
- [ ] Bot publishes response
- [ ] Bot message appears in chat
- [ ] 2 users can chat simultaneously
- [ ] Invalid stock codes handled
- [ ] Unit tests pass
- [ ] Integration tests pass

## Performance Optimization

1. **Database**:
   - Add index on `messages(chatroom_id, created_at DESC)`
   - Use connection pooling
   - Use prepared statements

2. **WebSocket**:
   - Limit message size (1000 chars)
   - Use goroutines efficiently
   - Implement ping/pong heartbeat

3. **RabbitMQ**:
   - Use channel pooling
   - Implement prefetch count
   - Use durable queues

4. **Caching** (Optional):
   - Cache user sessions in Redis
   - Cache chatroom membership

## Security Hardening

1. **Production**:
   - Change SESSION_SECRET
   - Enable HTTPS/TLS
   - Set Secure flag on cookies
   - Configure CORS properly

2. **Database**:
   - Use SSL connection
   - Least privilege access
   - Regular backups

3. **Monitoring**:
   - Set up logging
   - Configure alerts
   - Monitor error rates

## Troubleshooting

### Database Connection Issues

```bash
# Check connection
docker-compose exec chat-server nc -zv postgres 5432
```

### RabbitMQ Issues

```bash
# Check queues
docker-compose exec rabbitmq rabbitmqctl list_queues
```

### WebSocket Connection Issues

- Verify session cookie is set
- Check CORS configuration
- Verify chatroom membership

## Resources

- Go Documentation: https://go.dev/doc/
- Chi Router: https://github.com/go-chi/chi
- Gorilla WebSocket: https://github.com/gorilla/websocket
- RabbitMQ Go Client: https://github.com/rabbitmq/amqp091-go
- PostgreSQL: https://www.postgresql.org/docs/

## Support

For questions, refer to:
- `spec.json` - Complete specification
- `artifacts/architecture.md` - Architecture details
- `artifacts/security.md` - Security guidelines
- `artifacts/deployment.md` - Deployment guide
