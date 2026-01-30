# Docker Configuration

This directory contains all Docker-related configuration files for the Jobsity Chat application.

## Files

- **docker-compose.yml** - Orchestrates all services (PostgreSQL, RabbitMQ, Chat Server, Stock Bot)
- **Dockerfile.chat-server** - Multi-stage build for the chat server
- **Dockerfile.stock-bot** - Multi-stage build for the stock bot service

## Quick Start

```bash
# Start all services
docker-compose -f containers/docker-compose.yml up -d

# Or use the Makefile shortcut from project root
make docker-run
```

## Services

### Chat Server
- Port: 8080
- WebSocket endpoint: `/ws/chat/{chatroom_id}`
- API endpoints: `/api/v1/*`

### Stock Bot
- Consumes commands from RabbitMQ
- Publishes responses to RabbitMQ
- No exposed ports

### PostgreSQL
- Port: 5432
- Database: `jobsity_chat`
- User: `jobsity`

### RabbitMQ
- Port: 5672 (AMQP)
- Management UI: 15672
- Default credentials: guest/guest

## Development

```bash
# View logs
docker-compose -f containers/docker-compose.yml logs -f

# Stop services
docker-compose -f containers/docker-compose.yml down

# Rebuild and restart
docker-compose -f containers/docker-compose.yml up -d --build

# Clean up (remove volumes)
docker-compose -f containers/docker-compose.yml down -v
```

## Environment Variables

Environment variables can be configured in `.env` file at the project root or passed directly in `docker-compose.yml`.

Required variables:
- `DATABASE_URL`
- `RABBITMQ_URL`
- `SESSION_SECRET`
- `STOOQ_API_URL`
