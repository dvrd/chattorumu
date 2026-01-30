# Jobsity Chat

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

```bash
# Start all services
docker-compose -f containers/docker-compose.yml up -d

# Run database migrations
task migrate-up

# Access the application
# Chat: http://localhost:8080
# RabbitMQ UI: http://localhost:15672 (guest/guest)
```

## Local Development

```bash
# Install dependencies
task deps

# Run chat server
task run-server

# Run stock bot (in another terminal)
task run-bot

# Run tests
task test

# Run integration tests
task test-integration
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

# Access Swagger UI
open http://localhost:8081
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
  -v $(pwd)/artifacts:/api \
  swaggerapi/swagger-ui
```

Then visit: http://localhost:8081

#### OpenAPI Specification

The complete OpenAPI spec is available at `artifacts/openapi.yaml` and includes:
- All 10 REST endpoints + WebSocket documentation
- Request/response schemas with examples
- Authentication requirements (session-based cookies)
- Error responses and status codes
- Full validation with automatic runtime checks (dev mode)

**Services URLs:**
- 🌐 Chat Application: http://localhost:8080
- 📖 Swagger UI: http://localhost:8081
- 🐰 RabbitMQ Management: http://localhost:15672 (guest/guest)

## Stock Command

Send `/stock=AAPL.US` in the chat to get stock quotes.

Bot responds with: `AAPL.US quote is $93.42 per share`

## Testing

```bash
# Unit tests
task test

# Integration tests with test containers
task test-integration

# Generate coverage report
task coverage

# Run linter
task lint
```

## Project Structure

```
.
├── cmd/
│   ├── chat-server/    # Chat server entry point
│   └── stock-bot/      # Stock bot entry point
├── internal/
│   ├── config/         # Configuration
│   ├── domain/         # Domain entities
│   ├── service/        # Business logic
│   ├── repository/     # Data access
│   ├── handler/        # HTTP handlers
│   ├── websocket/      # WebSocket hub and client
│   └── middleware/     # HTTP middleware
├── migrations/         # Database migrations
├── static/             # Frontend assets
├── containers/         # Docker configuration
│   ├── docker-compose.yml
│   ├── Dockerfile.chat-server
│   └── Dockerfile.stock-bot
├── artifacts/          # API specs and schemas
└── docs/               # Documentation
```

## Building

```bash
# Build both services
task build

# Build Docker images
task docker-build
```

## Deployment

### Docker Compose (Development)

```bash
docker-compose -f containers/docker-compose.yml up -d
```

## License

MIT
