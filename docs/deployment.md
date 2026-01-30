# Deployment Guide - Jobsity Chat

## Overview

This guide covers local development and production deployment options for the Jobsity Chat application.

## Prerequisites

- Docker 20.10+
- Docker Compose 2.0+
- Go 1.21+ (for local development)
- kubectl (for Kubernetes deployment)
- PostgreSQL 15+ (if running without Docker)
- RabbitMQ 3.12+ (if running without Docker)

## Environment Variables

### Chat Server

```bash
# Server Configuration
PORT=8080
HOST=0.0.0.0
ENV=development  # development | production

# Database Configuration
DATABASE_URL=postgres://user:password@localhost:5432/jobsity_chat?sslmode=disable
DB_MAX_CONNECTIONS=25
DB_MAX_IDLE_CONNECTIONS=5
DB_CONNECTION_MAX_LIFETIME=5m

# RabbitMQ Configuration
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE_COMMANDS=chat.commands
RABBITMQ_EXCHANGE_RESPONSES=chat.responses
RABBITMQ_QUEUE_STOCK_COMMANDS=stock.commands

# Session Configuration
SESSION_SECRET=your-secret-key-change-in-production
SESSION_MAX_AGE=86400  # 24 hours in seconds

# CORS Configuration
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080

# Logging
LOG_LEVEL=info  # debug | info | warn | error
LOG_FORMAT=json  # json | text
```

### Stock Bot

```bash
# RabbitMQ Configuration
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE_COMMANDS=chat.commands
RABBITMQ_EXCHANGE_RESPONSES=chat.responses
RABBITMQ_QUEUE_STOCK_COMMANDS=stock.commands

# Stooq API Configuration
STOOQ_API_URL=https://stooq.com
STOOQ_API_TIMEOUT=10s
STOOQ_API_MAX_RETRIES=3

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

## Local Development with Docker Compose

### 1. Create `.env` file

```bash
cp .env.example .env
# Edit .env with your local configuration
```

### 2. Start all services

```bash
docker-compose up -d
```

This will start:
- PostgreSQL (port 5432)
- RabbitMQ (ports 5672, 15672)
- Chat Server (port 8080)
- Stock Bot

### 3. Run database migrations

```bash
docker-compose exec chat-server make migrate-up
```

### 4. Access services

- Chat Application: http://localhost:8080
- RabbitMQ Management: http://localhost:15672 (guest/guest)
- PostgreSQL: localhost:5432

### 5. View logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f chat-server
docker-compose logs -f stock-bot
```

### 6. Stop services

```bash
docker-compose down

# With volumes (deletes data)
docker-compose down -v
```

## Local Development without Docker

### 1. Start PostgreSQL

```bash
# Using Homebrew (macOS)
brew install postgresql@15
brew services start postgresql@15

# Create database
createdb jobsity_chat
```

### 2. Start RabbitMQ

```bash
# Using Homebrew (macOS)
brew install rabbitmq
brew services start rabbitmq

# Enable management plugin
rabbitmq-plugins enable rabbitmq_management
```

### 3. Run database migrations

```bash
# Install golang-migrate
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations
migrate -path migrations -database "postgres://localhost:5432/jobsity_chat?sslmode=disable" up
```

### 4. Start Chat Server

```bash
cd cmd/chat-server
go run main.go
```

### 5. Start Stock Bot

```bash
cd cmd/stock-bot
go run main.go
```

## Docker Build

### Build Chat Server Image

```bash
docker build -t jobsity-chat:latest -f Dockerfile.chat-server .
```

### Build Stock Bot Image

```bash
docker build -t stock-bot:latest -f Dockerfile.stock-bot .
```

### Multi-stage Dockerfile (Chat Server)

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-s -w" \
    -o chat-server ./cmd/chat-server

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary
COPY --from=builder /app/chat-server .

# Copy migrations
COPY --from=builder /app/migrations ./migrations

# Copy static files (if any)
COPY --from=builder /app/static ./static

EXPOSE 8080

CMD ["./chat-server"]
```

## Docker Compose Configuration

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    container_name: jobsity-postgres
    environment:
      POSTGRES_USER: jobsity
      POSTGRES_PASSWORD: jobsity123
      POSTGRES_DB: jobsity_chat
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U jobsity"]
      interval: 10s
      timeout: 5s
      retries: 5

  rabbitmq:
    image: rabbitmq:3.12-management-alpine
    container_name: jobsity-rabbitmq
    environment:
      RABBITMQ_DEFAULT_USER: guest
      RABBITMQ_DEFAULT_PASS: guest
    ports:
      - "5672:5672"
      - "15672:15672"
    volumes:
      - rabbitmq_data:/var/lib/rabbitmq
    healthcheck:
      test: ["CMD", "rabbitmq-diagnostics", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  chat-server:
    build:
      context: .
      dockerfile: Dockerfile.chat-server
    container_name: jobsity-chat-server
    environment:
      PORT: 8080
      DATABASE_URL: postgres://jobsity:jobsity123@postgres:5432/jobsity_chat?sslmode=disable
      RABBITMQ_URL: amqp://guest:guest@rabbitmq:5672/
      SESSION_SECRET: ${SESSION_SECRET:-change-me-in-production}
      ENV: development
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      rabbitmq:
        condition: service_healthy
    restart: unless-stopped

  stock-bot:
    build:
      context: .
      dockerfile: Dockerfile.stock-bot
    container_name: jobsity-stock-bot
    environment:
      RABBITMQ_URL: amqp://guest:guest@rabbitmq:5672/
      STOOQ_API_URL: https://stooq.com
      ENV: development
    depends_on:
      rabbitmq:
        condition: service_healthy
    restart: unless-stopped

volumes:
  postgres_data:
  rabbitmq_data:
```

## Kubernetes Deployment

### Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: jobsity-chat
```

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: chat-config
  namespace: jobsity-chat
data:
  PORT: "8080"
  ENV: "production"
  RABBITMQ_EXCHANGE_COMMANDS: "chat.commands"
  RABBITMQ_EXCHANGE_RESPONSES: "chat.responses"
  RABBITMQ_QUEUE_STOCK_COMMANDS: "stock.commands"
  STOOQ_API_URL: "https://stooq.com"
  LOG_LEVEL: "info"
  LOG_FORMAT: "json"
```

### Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: chat-secrets
  namespace: jobsity-chat
type: Opaque
stringData:
  DATABASE_URL: "postgres://user:password@postgres:5432/jobsity_chat?sslmode=disable"
  RABBITMQ_URL: "amqp://user:password@rabbitmq:5672/"
  SESSION_SECRET: "your-production-secret-key"
```

### PostgreSQL StatefulSet

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: jobsity-chat
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15-alpine
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_DB
          value: jobsity_chat
        - name: POSTGRES_USER
          value: jobsity
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: chat-secrets
              key: DATABASE_PASSWORD
        volumeMounts:
        - name: postgres-storage
          mountPath: /var/lib/postgresql/data
  volumeClaimTemplates:
  - metadata:
      name: postgres-storage
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

### RabbitMQ StatefulSet

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rabbitmq
  namespace: jobsity-chat
spec:
  serviceName: rabbitmq
  replicas: 1
  selector:
    matchLabels:
      app: rabbitmq
  template:
    metadata:
      labels:
        app: rabbitmq
    spec:
      containers:
      - name: rabbitmq
        image: rabbitmq:3.12-management-alpine
        ports:
        - containerPort: 5672
        - containerPort: 15672
        env:
        - name: RABBITMQ_DEFAULT_USER
          value: guest
        - name: RABBITMQ_DEFAULT_PASS
          valueFrom:
            secretKeyRef:
              name: chat-secrets
              key: RABBITMQ_PASSWORD
        volumeMounts:
        - name: rabbitmq-storage
          mountPath: /var/lib/rabbitmq
  volumeClaimTemplates:
  - metadata:
      name: rabbitmq-storage
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 5Gi
```

### Chat Server Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: chat-server
  namespace: jobsity-chat
spec:
  replicas: 2
  selector:
    matchLabels:
      app: chat-server
  template:
    metadata:
      labels:
        app: chat-server
    spec:
      containers:
      - name: chat-server
        image: jobsity-chat:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: chat-secrets
              key: DATABASE_URL
        - name: RABBITMQ_URL
          valueFrom:
            secretKeyRef:
              name: chat-secrets
              key: RABBITMQ_URL
        - name: SESSION_SECRET
          valueFrom:
            secretKeyRef:
              name: chat-secrets
              key: SESSION_SECRET
        envFrom:
        - configMapRef:
            name: chat-config
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### Stock Bot Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: stock-bot
  namespace: jobsity-chat
spec:
  replicas: 1
  selector:
    matchLabels:
      app: stock-bot
  template:
    metadata:
      labels:
        app: stock-bot
    spec:
      containers:
      - name: stock-bot
        image: stock-bot:latest
        env:
        - name: RABBITMQ_URL
          valueFrom:
            secretKeyRef:
              name: chat-secrets
              key: RABBITMQ_URL
        envFrom:
        - configMapRef:
            name: chat-config
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 256Mi
```

### Services

```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: jobsity-chat
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
  clusterIP: None

---
apiVersion: v1
kind: Service
metadata:
  name: rabbitmq
  namespace: jobsity-chat
spec:
  selector:
    app: rabbitmq
  ports:
  - port: 5672
    targetPort: 5672
    name: amqp
  - port: 15672
    targetPort: 15672
    name: management
  clusterIP: None

---
apiVersion: v1
kind: Service
metadata:
  name: chat-server
  namespace: jobsity-chat
spec:
  type: LoadBalancer
  selector:
    app: chat-server
  ports:
  - port: 80
    targetPort: 8080
```

## CI/CD Pipeline (GitHub Actions)

```yaml
name: Build and Deploy

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: 1.21

      - name: Run tests
        run: |
          go test -v -race -coverprofile=coverage.out ./...
          go tool cover -html=coverage.out -o coverage.html

      - name: Run linter
        uses: golangci/golangci-lint-action@v3

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Build Docker images
        run: |
          docker build -t jobsity-chat:${{ github.sha }} -f Dockerfile.chat-server .
          docker build -t stock-bot:${{ github.sha }} -f Dockerfile.stock-bot .

      - name: Push to registry
        run: |
          # Push to your registry
          echo "Push Docker images"

  deploy:
    needs: build
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Deploy to Kubernetes
        run: |
          # kubectl apply commands
          echo "Deploy to K8s"
```

## Monitoring

### Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'chat-server'
    static_configs:
      - targets: ['chat-server:8080']
    metrics_path: '/metrics'
```

### Grafana Dashboard

Import dashboard for Go applications and customize with:
- HTTP request rate
- WebSocket connections
- Message throughput
- Stock request latency
- RabbitMQ message rate

## Troubleshooting

### Chat server not connecting to database

```bash
# Check database connection
docker-compose logs postgres
docker-compose exec chat-server nc -zv postgres 5432
```

### Stock bot not receiving messages

```bash
# Check RabbitMQ queues
docker-compose exec rabbitmq rabbitmqctl list_queues

# Check RabbitMQ logs
docker-compose logs rabbitmq
```

### WebSocket connections failing

```bash
# Check CORS configuration
# Check session cookie settings
# Verify authentication middleware
```

## Production Checklist

- [ ] Change SESSION_SECRET to strong random value
- [ ] Use proper DATABASE_URL with SSL
- [ ] Use proper RABBITMQ_URL with credentials
- [ ] Set ENV=production
- [ ] Configure CORS allowed origins
- [ ] Enable HTTPS/TLS
- [ ] Set up database backups
- [ ] Configure log aggregation
- [ ] Set up monitoring and alerts
- [ ] Configure auto-scaling (HPA)
- [ ] Perform load testing
- [ ] Set up CI/CD pipeline
- [ ] Create runbooks for incidents
