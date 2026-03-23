.PHONY: build run test clean docker-up docker-down migrate

# Build the application
build:
	go build -o bin/copylingo ./cmd/server

# Run locally
run:
	go run ./cmd/server

# Run tests
test:
	go test ./... -v

# Run tests with coverage
test-cover:
	go test ./... -v -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html

# Docker operations
docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-build:
	docker compose up -d --build

# Run database migration (requires psql)
migrate:
	psql -h localhost -U copylingo -d copylingo -f migrations/001_init.up.sql

migrate-down:
	psql -h localhost -U copylingo -d copylingo -f migrations/001_init.down.sql

# Development: start infra only (DB + Redis)
infra:
	docker compose up -d postgres redis

# Lint
lint:
	golangci-lint run ./...

# Download dependencies
deps:
	go mod tidy
	go mod download
