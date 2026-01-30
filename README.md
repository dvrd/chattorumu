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

## Quick Start

```bash
# Start all services
docker-compose -f containers/docker-compose.yml up -d

# Run database migrations
make migrate-up

# Access the application
# Chat: http://localhost:8080
# RabbitMQ UI: http://localhost:15672 (guest/guest)
```

## Local Development

```bash
# Install dependencies
make deps

# Run chat server
make run-server

# Run stock bot (in another terminal)
make run-bot

# Run tests
make test

# Run integration tests
make test-integration
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
- `POST /api/v1/auth/logout` - Logout user
- `GET /api/v1/chatrooms` - List chatrooms
- `POST /api/v1/chatrooms` - Create chatroom
- `GET /api/v1/chatrooms/{id}/messages` - Get last 50 messages
- `WS /ws/chat/{chatroom_id}` - WebSocket connection for real-time chat

## Stock Command

Send `/stock=AAPL.US` in the chat to get stock quotes.

Bot responds with: `AAPL.US quote is $93.42 per share`

## Testing

```bash
# Unit tests
make test

# Integration tests with test containers
make test-integration

# Generate coverage report
make coverage

# Run linter
make lint
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
make build

# Build Docker images
make docker-build
```

## Deployment

### Docker Compose (Development)

```bash
docker-compose -f containers/docker-compose.yml up -d
```

## License

MIT
