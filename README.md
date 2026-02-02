# Chattorumu (just a chat)

Real-time browser-based chat application with stock quote bot integration.

## Features

- Real-time chat with WebSocket
- User authentication (session-based)
- Stock quote bot (`/stock=AAPL.US` command)
- Multiple chatrooms support
- Last 50 messages display (ordered by timestamp)
- Decoupled microservices architecture

## Architecture

- **Chat Server**: HTTP/WebSocket server for user interactions
- **Stock Bot**: Decoupled service for fetching stock quotes
- **PostgreSQL**: Database for users, messages, chatrooms
- **RabbitMQ**: Message broker for async communication

## Prerequisites

- Docker & Docker Compose
- Go 1.21+ (for local development)
- [Task](https://taskfile.dev) - Modern task runner (replaces Make)

### Installing Task

```bash
# macOS
brew install go-task/tap/go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin

# Windows (with Scoop)
scoop install task

# Or download binary from https://github.com/go-task/task/releases
```

## Quick Start

Prerequisites: Docker and Docker Compose must be installed and running.

```bash
# Start all services (Docker Compose)
task docker:run

# Wait for services to be ready (~30 seconds)
# Chat: http://localhost:8080
# Swagger UI: http://localhost:8081/swagger/
# RabbitMQ Management: http://localhost:15672 (guest/guest)
```

**Note:** Database migrations run automatically during service startup.

## Local Development

### Prerequisites
- PostgreSQL 13+
- RabbitMQ 3.12+
- Go 1.21+

### Setup

```bash
# Install and start PostgreSQL
brew install postgresql
brew services start postgresql

# Create database and user
psql -U postgres << EOF
CREATE USER jobsity WITH PASSWORD 'jobsity123';
CREATE DATABASE jobsity_chat OWNER jobsity;
GRANT ALL PRIVILEGES ON DATABASE jobsity_chat TO jobsity;
EOF

psql -U postgres -d jobsity_chat << EOF
GRANT ALL ON SCHEMA public TO jobsity;
GRANT CREATE ON SCHEMA public TO jobsity;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO jobsity;
EOF

# Install and start RabbitMQ
brew install rabbitmq
brew services start rabbitmq

# Download Go dependencies
go mod download
```

### Running Locally

```bash
# Run chat server (on port 8080)
task run:server

# Run stock bot in another terminal
task run:bot

# In another terminal, run tests
task test          # Run all tests (unit + E2E)
task test:unit     # Run unit tests only (~2 min, no Docker needed)
task test:e2e      # Run E2E tests only (~3 min, requires Docker)
```

## Environment Variables

Copy `.env.example` to `.env` and configure:

- `DATABASE_URL`: PostgreSQL connection string
- `RABBITMQ_URL`: RabbitMQ connection string
- `SESSION_SECRET`: Secret for session encryption
- `STOOQ_API_URL`: Stock API base URL

## API Endpoints

- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login user
- `GET /api/v1/auth/me` - Get current user info
- `POST /api/v1/auth/logout` - Logout user
- `GET /api/v1/chatrooms` - List chatrooms
- `POST /api/v1/chatrooms` - Create chatroom
- `POST /api/v1/chatrooms/{id}/join` - Join chatroom
- `GET /api/v1/chatrooms/{id}/messages` - Get last 50 messages
- `WS /ws/chat/{chatroom_id}` - WebSocket connection for real-time chat

## API Documentation

### Interactive Swagger UI

The API is fully documented with OpenAPI 3.0 specification. You can explore and test all endpoints using Swagger UI.

#### Using Docker Compose (Recommended)

```bash
# Start all services including Swagger UI
docker-compose -f containers/docker-compose.yml up -d

# Access Swagger UI (note the /swagger/ path)
open http://localhost:8081/swagger/
```

The Swagger UI will automatically load the OpenAPI specification from `artifacts/openapi.yaml` and provide:
- 📖 Complete API documentation with examples
- 🧪 Interactive endpoint testing (try it out!)
- 📋 Request/response schemas
- 🔐 Authentication testing with session cookies

#### Using Docker Standalone

If you only want to run Swagger UI:

```bash
docker run -p 8081:8080 \
  -e SWAGGER_JSON=/api/openapi.yaml \
  -e BASE_URL=/swagger \
  -v $(pwd)/artifacts:/api \
  swaggerapi/swagger-ui
```

Then visit: http://localhost:8081/swagger/

#### OpenAPI Specification

The complete OpenAPI spec is available at `artifacts/openapi.yaml` and includes:
- All 10 REST endpoints + WebSocket documentation
- Request/response schemas with examples
- Authentication requirements (session-based cookies)
- Error responses and status codes
- Full validation with automatic runtime checks (dev mode)

**Services URLs:**
- 🌐 Chat Application: http://localhost:8080
- 📖 Swagger UI: http://localhost:8081/swagger/
- 🐰 RabbitMQ Management: http://localhost:15672 (guest/guest)

## Stock Command

Send `/stock=AAPL.US` in the chat to get stock quotes.

Bot responds with: `AAPL.US quote is $93.42 per share`

### Stock Bot Flow

```
User sends /stock=AAPL.US
        ↓
Chat Server (WebSocket) receives message
        ↓
Publishes to RabbitMQ: "chat.commands" exchange
        ↓
Stock Bot consumes from "chat.commands" queue
        ↓
Fetches quote from Stooq API
        ↓
Publishes response to RabbitMQ: "chat.responses" exchange
        ↓
ResponseConsumer receives from "chat.responses" queue
        ↓
Broadcasts to WebSocket clients via Hub
        ↓
All users in chatroom see the stock quote
```

### Observability

The application includes comprehensive observability features:

- **Structured Logging**: JSON formatted logs with context (slog)
- **Prometheus Metrics**:
  - HTTP request duration and count (by method, path, status)
  - WebSocket active connections (by chatroom)
  - WebSocket messages sent (by chatroom)
- **Request Tracing**: Request IDs propagated through context

Access metrics at: `http://localhost:9090` (if Prometheus is configured)

## Testing

### Test Suite Overview

- **Unit Tests**: 630+ tests covering all packages (fast, no Docker)
- **E2E Tests**: 63+ tests with real services (PostgreSQL, RabbitMQ, WebSocket)
- **Messaging Tests**: 11 new E2E tests for RabbitMQ integration
- **Coverage**: ~77% across the codebase

### Running Tests

```bash
# Run all tests (unit + E2E)
task tests

# Run unit tests only (fast, ~2 minutes, no Docker required)
task test:unit

# Run E2E tests only (requires Docker, ~3 minutes)
task test:e2e

# Generate coverage report
task coverage

# Run linters
task lint

# Format code
task fmt
```

### Test Types Explained

| Type | Speed | Docker | Command | Use Case |
|------|-------|--------|---------|----------|
| **Unit** | ~2min | ❌ | `task test:unit` | Fast feedback during development |
| **E2E** | ~3min | ✅ | `task test:e2e` | Full integration testing |
| **All** | ~5min | ✅ | `task tests` | CI/CD pipeline |

### Test Organization

```
tests/e2e/
├── setup_test.go              # E2E infrastructure (Docker, DB, RabbitMQ)
├── auth_e2e_test.go           # Authentication tests
├── chatroom_e2e_test.go       # Chatroom functionality tests
├── websocket_e2e_test.go      # WebSocket communication tests
├── repository_e2e_test.go     # Database repository tests
├── messaging_e2e_test.go      # RabbitMQ integration tests (NEW)
└── helpers_test.go            # Test utilities and helpers

internal/*/
└── *_test.go                  # Unit tests for each package
```

## Project Structure

```
.
├── cmd/
│   ├── chat-server/              # Chat server entry point
│   └── stock-bot/                # Stock bot entry point
├── internal/
│   ├── config/                   # Configuration & database setup
│   ├── domain/                   # Domain entities (User, Message, etc)
│   ├── service/                  # Business logic (Auth, Chat)
│   ├── repository/postgres/      # PostgreSQL data access layer
│   ├── handler/                  # HTTP API handlers
│   ├── websocket/                # WebSocket hub & client
│   ├── middleware/               # HTTP middleware (Auth, CORS, Rate limit)
│   ├── messaging/                # RabbitMQ integration & consumer
│   ├── stock/                    # Stock quote service (Stooq API)
│   ├── observability/            # Logging & metrics (slog, Prometheus)
│   └── testutil/                 # Test utilities & mocks
├── tests/
│   └── e2e/                      # End-to-end integration tests
│       ├── setup_test.go         # Docker infrastructure & services
│       ├── auth_e2e_test.go      # Authentication flow tests
│       ├── chatroom_e2e_test.go  # Chatroom operations tests
│       ├── websocket_e2e_test.go # WebSocket communication tests
│       ├── repository_e2e_test.go # Database integration tests
│       ├── messaging_e2e_test.go # RabbitMQ integration tests
│       └── helpers_test.go       # Test utilities & helpers
├── migrations/                   # Database migrations (SQL)
├── static/                       # Frontend assets (HTML, CSS, JS)
├── containers/                   # Docker configuration
│   ├── docker-compose.yml        # Multi-service orchestration
│   ├── Dockerfile.chat-server    # Chat server image
│   └── Dockerfile.stock-bot      # Stock bot image
└── artifacts/                    # Generated files
    ├── openapi.yaml              # OpenAPI 3.0 specification
    └── schemas/                  # API schemas
```

## Building

```bash
# Build both services (binary in ./bin/)
task build

# Build Docker images
task docker:build
```

## Deployment

### Docker Compose (Development)

```bash
# Start all services
task docker:run

# View logs
task docker:logs

# Stop services
task docker:stop

# Stop and remove volumes
task docker:clean
```

## Troubleshooting

### Common Issues

**E2E Tests Timeout**
```bash
# Increase timeout if tests fail due to slow Docker startup
go test -tags=e2e -timeout=300s ./tests/e2e
```

**RabbitMQ Connection Failed**
```bash
# Ensure RabbitMQ is running
brew services start rabbitmq

# Or check Docker container
docker ps | grep rabbitmq
```

**Database Migration Issues**
```bash
# Check migration status
task migrate:status

# Rollback last migration if needed
task migrate:down

# Rerun migrations
task docker:migrate:up
```

**WebSocket Connection Refused**
```bash
# Ensure chat server is running on port 8080
lsof -i :8080

# Or restart the service
task run:server
```

**Swagger UI Not Loading (404 Error)**
```bash
# Verify swagger-ui container is running
docker ps | grep swagger-ui

# Check if openapi.yaml is mounted correctly
docker exec jobsity-swagger-ui ls -la /api/

# View swagger-ui logs
docker logs jobsity-swagger-ui
```


