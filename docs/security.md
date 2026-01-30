# Security Model - Jobsity Chat

## Overview

This document outlines the security measures implemented in the Jobsity Chat application, covering authentication, authorization, input validation, and protection against common vulnerabilities.

## Authentication

### Session-Based Authentication

The application uses **session-based authentication** with secure cookies instead of JWT to keep the implementation simple while maintaining security.

#### Registration Flow

```
1. User submits username, email, password
2. Server validates input
3. Server checks for existing username/email
4. Server hashes password with bcrypt (cost 12)
5. Server creates user record in database
6. Server returns success response
```

#### Login Flow

```
1. User submits username and password
2. Server looks up user by username
3. Server compares password hash using bcrypt
4. If valid:
   - Create session record with random UUID token
   - Set expires_at (24 hours from now)
   - Set secure cookie (session_id)
   - Return success
5. If invalid:
   - Return 401 with generic error message
   - Log failed attempt (for rate limiting)
```

#### Session Validation

```
1. Client includes session_id cookie in request
2. Middleware extracts cookie value
3. Middleware queries sessions table
4. Check if session exists and not expired
5. If valid:
   - Load user data
   - Add user to request context
   - Continue to handler
6. If invalid:
   - Return 401 Unauthorized
   - Optionally delete expired session
```

#### Logout Flow

```
1. User requests logout
2. Server deletes session from database
3. Server clears session cookie
4. Return success
```

### Password Security

**Hashing Algorithm**: bcrypt with cost factor 12

```go
// Hash password
hashedPassword, err := bcrypt.GenerateFromPassword(
    []byte(password),
    12, // cost
)

// Verify password
err := bcrypt.CompareHashAndPassword(
    []byte(hashedPassword),
    []byte(password),
)
```

**Requirements**:
- Minimum 8 characters
- No maximum (database limit: 100 chars)
- No complexity requirements (to keep simple)
- Consider adding: uppercase, lowercase, number, special char (optional)

### Cookie Security

**Session Cookie Configuration**:

```go
cookie := &http.Cookie{
    Name:     "session_id",
    Value:    sessionToken,
    Path:     "/",
    HttpOnly: true,        // Prevent JavaScript access (XSS protection)
    Secure:   true,        // HTTPS only (set to false in dev)
    SameSite: http.SameSiteStrictMode,  // CSRF protection
    MaxAge:   86400,       // 24 hours
}
```

**Attributes**:
- **HttpOnly**: Prevents XSS attacks from stealing session
- **Secure**: Only sent over HTTPS
- **SameSite=Strict**: Prevents CSRF attacks
- **MaxAge**: Auto-expire after 24 hours

### Session Management

**Sessions Table Schema**:

```sql
CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    token VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

**Session Cleanup**:
- Periodic cleanup job (every hour)
- Delete sessions where `expires_at < CURRENT_TIMESTAMP`
- Can be implemented as cron job or background goroutine

```go
func CleanupExpiredSessions(ctx context.Context, repo SessionRepository) {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            err := repo.DeleteExpiredSessions(ctx)
            if err != nil {
                log.Error("Failed to cleanup sessions", "error", err)
            }
        case <-ctx.Done():
            return
        }
    }
}
```

## Authorization

### Access Control Rules

1. **Public Endpoints** (no authentication):
   - POST /api/v1/auth/register
   - POST /api/v1/auth/login
   - GET /health

2. **Authenticated Endpoints**:
   - POST /api/v1/auth/logout
   - GET /api/v1/chatrooms
   - POST /api/v1/chatrooms
   - GET /api/v1/chatrooms/{id}/messages
   - POST /api/v1/chatrooms/{id}/join
   - WS /ws/chat/{chatroom_id}

3. **Chatroom Access**:
   - Users must be members to access chatroom
   - Check `chatroom_members` table before allowing:
     - Message retrieval
     - WebSocket connection
     - Sending messages

### Middleware Chain

```
Request → Logger → Recoverer → CORS → RateLimiter → Auth → Handler
```

**Auth Middleware**:

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie("session_id")
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        session, err := sessionRepo.GetByToken(r.Context(), cookie.Value)
        if err != nil || session.ExpiresAt.Before(time.Now()) {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        user, err := userRepo.GetByID(r.Context(), session.UserID)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Add user to context
        ctx := context.WithValue(r.Context(), "user", user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## Input Validation

### User Input

**Username**:
- Regex: `^[a-zA-Z0-9_]{3,50}$`
- Min length: 3 characters
- Max length: 50 characters
- Allowed: alphanumeric and underscore

**Email**:
- Regex: RFC 5322 compliant
- Max length: 255 characters
- Must contain @ and valid domain

**Password**:
- Min length: 8 characters
- Max length: 100 characters
- No other requirements (simplicity)

**Chatroom Name**:
- Min length: 1 character
- Max length: 100 characters

**Message Content**:
- Min length: 1 character
- Max length: 1000 characters
- No HTML allowed (escaped on display)

**Stock Code**:
- Regex: `^[a-zA-Z0-9.]{1,20}$`
- Max length: 20 characters
- Allowed: alphanumeric and dots

### Validation Library

Use Go validator library:

```go
import "github.com/go-playground/validator/v10"

type RegisterRequest struct {
    Username string `json:"username" validate:"required,alphanum,min=3,max=50"`
    Email    string `json:"email" validate:"required,email,max=255"`
    Password string `json:"password" validate:"required,min=8,max=100"`
}
```

### SQL Injection Protection

**Use Parameterized Queries**:

```go
// SAFE - using parameterized query
query := `SELECT * FROM users WHERE username = $1`
row := db.QueryRow(query, username)

// UNSAFE - DO NOT DO THIS
query := fmt.Sprintf("SELECT * FROM users WHERE username = '%s'", username)
```

**Database Driver**: Use `lib/pq` or `pgx` which support prepared statements.

## Rate Limiting

### Limits

1. **Login Attempts**:
   - 5 attempts per 5 minutes per IP
   - Prevents brute force attacks

2. **Message Sending**:
   - 60 messages per minute per user
   - Prevents spam

3. **Stock Commands**:
   - 10 stock commands per minute per user
   - Prevents API abuse

### Implementation

Use `golang.org/x/time/rate` package:

```go
import "golang.org/x/time/rate"

// Per-user rate limiter
type RateLimiter struct {
    limiters sync.Map // map[userID]*rate.Limiter
}

func (rl *RateLimiter) GetLimiter(userID string) *rate.Limiter {
    if limiter, ok := rl.limiters.Load(userID); ok {
        return limiter.(*rate.Limiter)
    }

    limiter := rate.NewLimiter(rate.Every(time.Minute), 60) // 60 per minute
    rl.limiters.Store(userID, limiter)
    return limiter
}

func (rl *RateLimiter) Allow(userID string) bool {
    limiter := rl.GetLimiter(userID)
    return limiter.Allow()
}
```

### Rate Limit Response

When rate limit exceeded:

```json
{
  "error": "Rate limit exceeded. Please try again later.",
  "retry_after": 30
}
```

HTTP Status: **429 Too Many Requests**

## CORS (Cross-Origin Resource Sharing)

### Configuration

```go
cors.Options{
    AllowedOrigins:   []string{"http://localhost:3000"},
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Content-Type", "Authorization"},
    AllowCredentials: true,  // Required for cookies
    MaxAge:           300,   // Cache preflight for 5 minutes
}
```

### Production Settings

- Restrict `AllowedOrigins` to actual domain
- Enable `AllowCredentials` for session cookies
- Use HTTPS only

## Security Headers

### Response Headers

```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("X-XSS-Protection", "1; mode=block")
w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
w.Header().Set("Content-Security-Policy", "default-src 'self'")
```

**Explanation**:
- **X-Content-Type-Options**: Prevent MIME sniffing
- **X-Frame-Options**: Prevent clickjacking
- **X-XSS-Protection**: Enable browser XSS filter
- **Referrer-Policy**: Control referrer information
- **CSP**: Restrict resource loading

## WebSocket Security

### Authentication

WebSocket upgrade requires valid session:

```go
func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
    // Get user from context (set by auth middleware)
    user, ok := r.Context().Value("user").(*User)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    // Check chatroom membership
    chatroomID := chi.URLParam(r, "chatroom_id")
    isMember, err := h.chatroomService.IsMember(r.Context(), user.ID, chatroomID)
    if err != nil || !isMember {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Upgrade connection
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }

    // Create client
    client := &Client{
        hub:        h.hub,
        conn:       conn,
        send:       make(chan []byte, 256),
        userID:     user.ID,
        username:   user.Username,
        chatroomID: chatroomID,
    }

    // Register client
    h.hub.register <- client
}
```

### Message Validation

Validate all incoming WebSocket messages:

```go
func (c *Client) ReadPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()

    c.conn.SetReadLimit(maxMessageSize)  // 1024 bytes
    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })

    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            break
        }

        // Validate message length
        if len(message) > 1000 {
            continue
        }

        // Parse and validate message
        var msg Message
        if err := json.Unmarshal(message, &msg); err != nil {
            continue
        }

        // Process message
        c.hub.broadcast <- &BroadcastMessage{
            ChatroomID: c.chatroomID,
            Message:    msg,
        }
    }
}
```

## Protection Against Common Vulnerabilities

### 1. SQL Injection

**Prevention**:
- Use parameterized queries
- Use ORM with prepared statements
- Never concatenate user input into SQL

### 2. XSS (Cross-Site Scripting)

**Prevention**:
- Escape all user input on display
- Set HttpOnly flag on cookies
- Use Content-Security-Policy header
- Sanitize HTML if allowing rich text (not needed for this app)

### 3. CSRF (Cross-Site Request Forgery)

**Prevention**:
- Use SameSite=Strict cookie attribute
- Validate Origin/Referer headers
- Use CSRF tokens (optional, SameSite is sufficient)

### 4. Command Injection

**Prevention**:
- Don't execute shell commands with user input
- If needed, use allowlist and escape properly

### 5. Path Traversal

**Prevention**:
- Validate file paths
- Don't serve arbitrary files based on user input

### 6. DoS (Denial of Service)

**Prevention**:
- Rate limiting
- Request timeout
- Connection limits
- Message size limits

### 7. Sensitive Data Exposure

**Prevention**:
- Never log passwords
- Don't return password hashes in API responses
- Use HTTPS in production
- Secure environment variables

## Secrets Management

### Development

Use `.env` file (not committed to Git):

```bash
SESSION_SECRET=dev-secret-change-me
DATABASE_URL=postgres://...
RABBITMQ_URL=amqp://...
```

### Production

**Options**:

1. **Environment Variables** (simplest):
   - Set in deployment configuration
   - Use Kubernetes Secrets

2. **HashiCorp Vault**:
   - Centralized secrets management
   - Automatic rotation

3. **AWS Secrets Manager** / **GCP Secret Manager**:
   - Cloud-native secrets storage
   - IAM-based access control

### Loading Secrets

```go
import "github.com/joho/godotenv"

func LoadConfig() (*Config, error) {
    // Load .env in development
    if os.Getenv("ENV") != "production" {
        godotenv.Load()
    }

    return &Config{
        Port:          os.Getenv("PORT"),
        DatabaseURL:   os.Getenv("DATABASE_URL"),
        RabbitMQURL:   os.Getenv("RABBITMQ_URL"),
        SessionSecret: os.Getenv("SESSION_SECRET"),
    }, nil
}
```

## Logging and Auditing

### Security Events to Log

- Failed login attempts (with IP)
- Successful logins
- Session creation/deletion
- Rate limit violations
- Authorization failures
- Invalid input attempts
- WebSocket connection/disconnection
- Stock command execution

### Log Format

```json
{
  "timestamp": "2026-01-28T10:30:00Z",
  "level": "warn",
  "event": "failed_login",
  "username": "john_doe",
  "ip": "192.168.1.1",
  "user_agent": "Mozilla/5.0..."
}
```

### What NOT to Log

- Passwords (plain or hashed)
- Session tokens
- Full request bodies with sensitive data
- Credit card numbers (N/A for this app)

## Security Checklist

### Development

- [x] Use parameterized queries
- [x] Hash passwords with bcrypt
- [x] Validate all user input
- [x] Set secure cookie flags
- [x] Implement rate limiting
- [x] Use HTTPS (in production)
- [x] Set security headers
- [x] Sanitize/escape output
- [x] Use CORS properly
- [x] Implement session expiration

### Deployment

- [ ] Change SESSION_SECRET to strong random value
- [ ] Use HTTPS/TLS certificates
- [ ] Restrict CORS to actual domains
- [ ] Enable security headers
- [ ] Set up secrets management
- [ ] Configure firewall rules
- [ ] Enable database SSL
- [ ] Set up intrusion detection
- [ ] Configure log aggregation
- [ ] Set up security monitoring

### Testing

- [ ] Test authentication flows
- [ ] Test authorization rules
- [ ] Test rate limiting
- [ ] Test input validation
- [ ] Test CORS configuration
- [ ] Test session expiration
- [ ] Penetration testing (optional)
- [ ] Security audit (optional)

## Incident Response

### If Session Leak Detected

1. Invalidate all sessions: `DELETE FROM sessions`
2. Force all users to login again
3. Rotate SESSION_SECRET
4. Investigate root cause

### If Database Breach

1. Immediately take database offline
2. Notify users of potential breach
3. Force password reset for all users
4. Review access logs
5. Patch vulnerability
6. Restore from backup if needed

### If XSS Vulnerability Found

1. Deploy patch immediately
2. Clear affected data if stored
3. Notify users if needed
4. Review code for similar issues

## Resources

- [OWASP Top 10](https://owasp.org/Top10/)
- [Go Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Go_Security_Cheat_Sheet.html)
- [WebSocket Security](https://owasp.org/www-community/websocket-security)
