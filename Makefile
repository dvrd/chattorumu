.PHONY: help build run test lint docker-build docker-run clean migrate-up migrate-down migrate-create

# Database URL (can be overridden with environment variable)
DATABASE_URL ?= postgres://jobsity:jobsity123@localhost:5432/jobsity_chat?sslmode=disable

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the applications
	@echo "Building chat-server..."
	@go build -o bin/chat-server ./cmd/chat-server
	@echo "Building stock-bot..."
	@go build -o bin/stock-bot ./cmd/stock-bot
	@echo "Build complete!"

run-server: ## Run the chat server
	@go run ./cmd/chat-server

run-bot: ## Run the stock bot
	@go run ./cmd/stock-bot

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	@go test -v -tags=integration ./test/...

test-short: ## Run short tests only
	@go test -short ./...

coverage: test ## Generate coverage report
	@go tool cover -func=coverage.out

lint: ## Run linters
	@echo "Running golangci-lint..."
	@golangci-lint run
	@echo "Running go vet..."
	@go vet ./...
	@echo "Running go fmt..."
	@go fmt ./...

fmt: ## Format code
	@go fmt ./...
	@gofmt -s -w .

docker-build: ## Build Docker images
	@echo "Building chat-server image..."
	@docker build -t jobsity-chat:latest -f containers/Dockerfile.chat-server .
	@echo "Building stock-bot image..."
	@docker build -t stock-bot:latest -f containers/Dockerfile.stock-bot .
	@echo "Docker images built successfully!"

docker-run: ## Run with Docker Compose
	@docker-compose -f containers/docker-compose.yml up -d
	@echo "Services started. Access chat at http://localhost:8080"
	@echo "RabbitMQ Management UI: http://localhost:15672 (guest/guest)"

docker-stop: ## Stop Docker Compose services
	@docker-compose -f containers/docker-compose.yml down

docker-logs: ## Show Docker Compose logs
	@docker-compose -f containers/docker-compose.yml logs -f

docker-clean: ## Stop and remove Docker volumes
	@docker-compose -f containers/docker-compose.yml down -v
	@echo "Docker containers and volumes removed"

migrate-up: ## Run database migrations up
	@echo "Running migrations..."
	@docker run --rm -v $(PWD)/migrations:/migrations --network host \
		migrate/migrate:v4.17.0 \
		-path=/migrations \
		-database "$(DATABASE_URL)" up
	@echo "Migrations complete!"

migrate-down: ## Rollback last migration
	@echo "Rolling back migration..."
	@docker run --rm -v $(PWD)/migrations:/migrations --network host \
		migrate/migrate:v4.17.0 \
		-path=/migrations \
		-database "$(DATABASE_URL)" down 1
	@echo "Rollback complete!"

migrate-create: ## Create new migration (use: make migrate-create NAME=create_users_table)
ifndef NAME
	@echo "Error: NAME is required. Usage: make migrate-create NAME=create_users_table"
	@exit 1
endif
	@echo "Creating migration: $(NAME)"
	@docker run --rm -v $(PWD)/migrations:/migrations \
		migrate/migrate:v4.17.0 \
		create -ext sql -dir /migrations -seq $(NAME)
	@echo "Migration files created in migrations/"

migrate-force: ## Force migration version (use: make migrate-force VERSION=1)
ifndef VERSION
	@echo "Error: VERSION is required. Usage: make migrate-force VERSION=1"
	@exit 1
endif
	@echo "Forcing migration to version $(VERSION)..."
	@docker run --rm -v $(PWD)/migrations:/migrations --network host \
		migrate/migrate:v4.17.0 \
		-path=/migrations \
		-database "$(DATABASE_URL)" force $(VERSION)
	@echo "Migration forced to version $(VERSION)!"

migrate-status: ## Check migration status
	@echo "Checking migration status..."
	@docker run --rm -v $(PWD)/migrations:/migrations --network host \
		migrate/migrate:v4.17.0 \
		-path=/migrations \
		-database "$(DATABASE_URL)" version

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated!"

install-tools: ## Install development tools
	@echo "Installing tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@echo "Tools installed!"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Clean complete!"

dev: ## Run development environment
	@docker-compose -f containers/docker-compose.yml up -d postgres rabbitmq
	@echo "Waiting for services to be ready..."
	@sleep 5
	@make migrate-up
	@echo "Starting chat server..."
	@make run-server

init: deps install-tools ## Initialize project (install dependencies and tools)
	@echo "Project initialized!"

.DEFAULT_GOAL := help
