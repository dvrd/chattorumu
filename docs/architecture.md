# Jobsity Chat - System Architecture

## Overview

Jobsity Chat is a real-time chat application with stock quote bot integration. The system consists of two main services: the Chat Server (handling user interactions) and the Stock Bot (fetching stock quotes), communicating via RabbitMQ message broker.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Browser Clients                          │
│  ┌──────────────┐              ┌──────────────┐                 │
│  │  User A      │              │  User B      │                 │
│  │  (Browser)   │              │  (Browser)   │                 │
│  └──────┬───────┘              └──────┬───────┘                 │
│         │                              │                          │
└─────────┼──────────────────────────────┼──────────────────────────┘
          │ HTTP/WS                      │ HTTP/WS
          │                              │
          ▼                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Chat Server (Go)                          │
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    HTTP Layer                             │  │
│  │  - Chi Router                                            │  │
│  │  - Middleware (Auth, CORS, Logging, Recovery)           │  │
│  │  - REST Handlers (Register, Login, Chatrooms)           │  │
│  └──────────────────┬───────────────────────────────────────┘  │
│                     │                                            │
│  ┌──────────────────┴───────────────────────────────────────┐  │
│  │                 WebSocket Layer                           │  │
│  │  - WebSocket Hub (Broadcast Pattern)                     │  │
│  │  - Client Connections (goroutines)                       │  │
│  │  - Message Broadcasting                                   │  │
│  │  - Command Detection (/stock=CODE)                       │  │
│  └──────────────────┬───────────────────────────────────────┘  │
│                     │                                            │
│  ┌──────────────────┴───────────────────────────────────────┐  │
│  │                  Service Layer                            │  │
│  │  - Auth Service (Login, Sessions)                        │  │
│  │  - Chat Service (Messages, Commands)                     │  │
│  │  - Chatroom Service (CRUD)                               │  │
│  └──────────────────┬───────────────────────────────────────┘  │
│                     │                                            │
│  ┌──────────────────┴───────────────────────────────────────┐  │
│  │                Repository Layer                           │  │
│  │  - User Repository                                        │  │
│  │  - Message Repository                                     │  │
│  │  - Chatroom Repository                                    │  │
│  │  - Session Repository                                     │  │
│  └──────────────────┬───────────────────────────────────────┘  │
│                     │                                            │
└─────────────────────┼────────────────────────────────────────────┘
                      │
                      │ SQL
                      ▼
              ┌──────────────┐
              │  PostgreSQL  │
              │              │
              │ - users      │
              │ - chatrooms  │
              │ - messages   │
              │ - sessions   │
              └──────────────┘

       ┌──────────────────┐
       │    RabbitMQ      │
       │                  │
       │ Exchanges:       │
       │ - chat.commands  │
       │ - chat.responses │
       │                  │
       │ Queues:          │
       │ - stock.commands │
       │ - stock.responses│
       └────┬──────▲──────┘
            │      │
   Publish  │      │ Consume
   /stock=  │      │ Quote
            │      │
            ▼      │
    ┌───────────────────────┐
    │   Stock Bot (Go)      │
    │                       │
    │ ┌─────────────────┐  │
    │ │ RabbitMQ        │  │
    │ │ Consumer        │  │
    │ └────────┬────────┘  │
    │          │            │
    │ ┌────────▼────────┐  │
    │ │ Stock Service   │  │
    │ │ - Parse command │  │
    │ │ - Validate code │  │
    │ └────────┬────────┘  │
    │          │            │
    │ ┌────────▼────────┐  │
    │ │ Stooq Client    │  │
    │ │ - HTTP Request  │  │
    │ │ - CSV Parser    │  │
    │ │ - Error Handler │  │
    │ └────────┬────────┘  │
    │          │            │
    │ ┌────────▼────────┐  │
    │ │ RabbitMQ        │  │
    │ │ Publisher       │  │
    │ └─────────────────┘  │
    └───────────────────────┘
            │
            │ HTTPS
            ▼
    ┌───────────────────┐
    │   Stooq API       │
    │ (External)        │
    │                   │
    │ /q/l/?s=aapl.us   │
    │ &f=sd2t2ohlcv     │
    │ &e=csv            │
    └───────────────────┘
```

## Component Details

### 1. Chat Server

**Purpose**: Handle user authentication, WebSocket connections, and message management.

**Technologies**:
- **HTTP Router**: Chi (lightweight, idiomatic)
- **WebSocket**: gorilla/websocket
- **Database**: PostgreSQL with lib/pq driver
- **Message Queue**: RabbitMQ with amqp091-go

**Key Patterns**:
- **Hub Pattern**: Central hub manages all WebSocket connections
- **Goroutines**: One goroutine per WebSocket connection
- **Channels**: Go channels for message broadcasting
- **Context**: Context-based cancellation for graceful shutdown

**Responsibilities**:
1. User registration and authentication
2. Session management (cookie-based)
3. WebSocket connection lifecycle
4. Message persistence (PostgreSQL)
5. Real-time message broadcasting
6. Command detection (`/stock=CODE`)
7. RabbitMQ message publishing (stock commands)
8. RabbitMQ message consumption (stock responses)

### 2. Stock Bot

**Purpose**: Decoupled service that fetches stock quotes from external API.

**Technologies**:
- **HTTP Client**: net/http with timeouts
- **CSV Parser**: encoding/csv
- **Message Queue**: RabbitMQ with amqp091-go

**Key Patterns**:
- **Consumer Pattern**: Continuously consume from RabbitMQ queue
- **Retry Logic**: Exponential backoff for API failures
- **Circuit Breaker**: Optional (prevent cascading failures)

**Responsibilities**:
1. Consume stock command messages from RabbitMQ
2. Validate stock codes
3. Fetch CSV data from Stooq API
4. Parse CSV and extract closing price
5. Format message: "AAPL.US quote is $93.42 per share"
6. Publish response to RabbitMQ
7. Handle errors gracefully (invalid codes, API failures)

### 3. PostgreSQL Database

**Purpose**: Persistent storage for users, chatrooms, messages, and sessions.

**Schema**:
- `users`: User accounts
- `chatrooms`: Chat rooms
- `messages`: Chat messages (ordered by timestamp)
- `chatroom_members`: Many-to-many user-chatroom relationship
- `sessions`: User sessions

**Optimizations**:
- Index on `messages(chatroom_id, created_at DESC)` for fast last-50 query
- Connection pooling (25 connections, 5 idle)
- Prepared statements for common queries

### 4. RabbitMQ Message Broker

**Purpose**: Decouple chat server and stock bot communication.

**Exchanges**:
- `chat.commands` (topic): Stock command routing
- `chat.responses` (fanout): Broadcast stock responses

**Queues**:
- `stock.commands`: Stock bot consumes commands
- `stock.responses.{chatroom_id}`: Chat server consumes responses

**Message Flow**:
1. User sends `/stock=AAPL.US` via WebSocket
2. Chat server detects command (not saved to DB)
3. Chat server publishes to `chat.commands` exchange
4. Stock bot consumes from `stock.commands` queue
5. Stock bot fetches data from Stooq API
6. Stock bot publishes response to `chat.responses` exchange
7. Chat server consumes response
8. Chat server saves bot message to DB (is_bot=true)
9. Chat server broadcasts to all WebSocket clients

## Data Flow

### Regular Message Flow

```
Browser → WebSocket → Hub → Service → Repository → PostgreSQL
                       ↓
                    Broadcast → All Connected Clients
```

### Stock Command Flow

```
Browser → WebSocket → Hub → Detect /stock= command
                              ↓
                         RabbitMQ (publish)
                              ↓
                          Stock Bot
                              ↓
                         Stooq API (HTTP)
                              ↓
                         Parse CSV
                              ↓
                         RabbitMQ (publish)
                              ↓
                         Chat Server (consume)
                              ↓
                         Save to DB (is_bot=true)
                              ↓
                         Broadcast → All Clients
```

## Concurrency Model

### Chat Server Goroutines

1. **Main HTTP Server**: Handles HTTP requests
2. **WebSocket Hub**: Central goroutine managing connections
3. **Client Connections**: One goroutine per WebSocket connection
   - Read messages from client
   - Write messages to client
4. **RabbitMQ Consumer**: Goroutine consuming stock responses
5. **Graceful Shutdown**: Context-based cancellation

```go
// Pseudo-code
func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.clients[client] = true
        case client := <-h.unregister:
            delete(h.clients, client)
            close(client.send)
        case message := <-h.broadcast:
            for client := range h.clients {
                select {
                case client.send <- message:
                default:
                    close(client.send)
                    delete(h.clients, client)
                }
            }
        case <-h.ctx.Done():
            return // Graceful shutdown
        }
    }
}
```

## Security Architecture

### Authentication Flow

1. User submits credentials to `/api/v1/auth/login`
2. Server validates password (bcrypt comparison)
3. Server creates session in database
4. Server sets secure cookie (`session_id`, HttpOnly, Secure, SameSite=Strict)
5. Client includes cookie in subsequent requests
6. Middleware validates session on each request

### WebSocket Authentication

1. Client upgrades HTTP connection to WebSocket
2. Middleware checks session cookie before upgrade
3. If valid, upgrade proceeds
4. If invalid, return 401 Unauthorized

### Input Validation

- Username: 3-50 chars, alphanumeric + underscore
- Password: Min 8 chars, hashed with bcrypt (cost 12)
- Message content: Max 1000 chars
- Stock code: Max 20 chars, alphanumeric + dots

### Rate Limiting

- Login attempts: 5 per 5 minutes per IP
- Messages: 60 per minute per user
- Stock commands: 10 per minute per user

## Scalability Considerations

### Horizontal Scaling

**Chat Server**:
- Stateless design (sessions in DB, not memory)
- Multiple instances behind load balancer
- RabbitMQ queue per chatroom for responses
- Challenge: WebSocket connections are stateful
  - Solution: Sticky sessions or Redis pub/sub for inter-instance communication

**Stock Bot**:
- Stateless workers
- Scale to N instances consuming from same queue
- RabbitMQ load-balances messages across workers

### Vertical Scaling

- PostgreSQL connection pooling (25 connections per instance)
- RabbitMQ channel pooling
- Efficient goroutine management (limit concurrent connections)

### Performance Targets

- Message latency: <100ms (WebSocket broadcast)
- Stock quote latency: <5s (end-to-end)
- Concurrent connections: 100+ per chat-server instance
- Messages per second: 1000+

## Error Handling

### Chat Server Errors

- WebSocket connection errors: Log and close connection
- Database errors: Return 500, log error
- RabbitMQ errors: Retry with exponential backoff
- Invalid commands: Send error message to client

### Stock Bot Errors

- Invalid stock code: Send error message via RabbitMQ
- API timeout: Retry 3 times, then send error message
- CSV parse error: Send error message
- RabbitMQ connection loss: Reconnect with backoff

## Deployment Architecture

### Docker Compose (Development)

```yaml
services:
  chat-server:
    image: jobsity-chat:latest
    ports: ["8080:8080"]
    depends_on: [postgres, rabbitmq]

  stock-bot:
    image: stock-bot:latest
    depends_on: [rabbitmq]

  postgres:
    image: postgres:15-alpine
    ports: ["5432:5432"]

  rabbitmq:
    image: rabbitmq:3.12-management-alpine
    ports: ["5672:5672", "15672:15672"]
```

### Kubernetes (Production)

- **Deployments**: chat-server (2 replicas), stock-bot (1 replica)
- **StatefulSets**: postgres, rabbitmq
- **Services**: chat-server (LoadBalancer), postgres (ClusterIP), rabbitmq (ClusterIP)
- **ConfigMaps**: Application configuration
- **Secrets**: Database credentials, RabbitMQ credentials

## Monitoring & Observability

### Metrics (Prometheus)

- `http_requests_total`: HTTP request counter
- `http_request_duration_seconds`: Request latency histogram
- `websocket_connections_active`: Active WebSocket connections
- `messages_sent_total`: Messages sent counter
- `stock_requests_total`: Stock requests counter
- `stock_request_duration_seconds`: Stock request latency

### Logging (slog)

- Structured JSON logs
- Log levels: DEBUG, INFO, WARN, ERROR
- Context: request_id, user_id, chatroom_id

### Health Checks

- `/health`: Basic liveness check
- `/health/ready`: Readiness check (database + RabbitMQ)

## Future Enhancements

1. **Multiple Chatrooms**: UI for chatroom selection (already supported in backend)
2. **Redis for Sessions**: Faster session lookups
3. **Redis Pub/Sub**: Inter-instance WebSocket message routing
4. **Message History**: Pagination for older messages
5. **User Presence**: Show online/offline status
6. **Typing Indicators**: Real-time typing notifications
7. **Message Reactions**: Emoji reactions
8. **File Uploads**: Image/file sharing
9. **Push Notifications**: Desktop/mobile notifications
10. **E2E Encryption**: Client-side encryption (optional)
