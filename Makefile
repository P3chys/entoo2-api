.PHONY: help dev build run test clean migrate-up migrate-down migrate-create docker-build docker-run

help:
	@echo "Available targets:"
	@echo "  dev            - Run development server with hot reload"
	@echo "  build          - Build the application"
	@echo "  run            - Run the application"
	@echo "  test           - Run tests"
	@echo "  migrate-up     - Run database migrations"
	@echo "  migrate-down   - Rollback last migration"
	@echo "  migrate-create - Create new migration (name=migration_name)"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"

dev:
	@echo "Starting development server..."
	@air || go run cmd/server/main.go

build:
	@echo "Building application..."
	@go build -o bin/entoo2-api cmd/server/main.go

run:
	@./bin/entoo2-api

test:
	@echo "Running tests..."
	@go test -v ./...

test-coverage:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out

clean:
	@rm -rf bin/
	@rm -f coverage.out

migrate-up:
	@echo "Running migrations..."
	@go run cmd/server/main.go migrate up

migrate-down:
	@echo "Rolling back migration..."
	@go run cmd/server/main.go migrate down

migrate-create:
	@echo "Creating migration: $(name)"
	@migrate create -ext sql -dir internal/database/migrations -seq $(name)

docker-build:
	@docker build -t entoo2-api:latest .

docker-run:
	@docker run -p 8000:8000 --env-file .env entoo2-api:latest

lint:
	@golangci-lint run

fmt:
	@go fmt ./...
	@goimports -w .
