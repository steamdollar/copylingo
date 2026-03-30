.PHONY: build run test clean docker-up docker-down migrate dev app-up app-logs restart-db restart-redis infra tmux tmux-stop

# Build the application
build:
	go build -o bin/copylingo ./cmd/server

# Run locally
run:
	go run ./cmd/server

# Run with hot reload (requires air)
dev: infra
	docker stop copylingo-app || true
	air

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

# Fast app-only rebuild and restart
app-up:
	docker compose up -d --build app

# Tail app logs
app-logs:
	docker compose logs -f app

# Restart specific instances
restart-db:
	docker compose restart postgres

restart-redis:
	docker compose restart redis

# Run everything in a detached tmux session as an All-in-One dashboard
tmux: infra
	@tmux kill-session -t copylingo 2>/dev/null || true
	@docker stop copylingo-app 2>/dev/null || true
	@tmux new-session -d -s copylingo -n 'Dashboard' "make dev"
	@tmux split-window -h -t copylingo "docker compose logs -f postgres"
	@tmux split-window -v -t copylingo "docker compose logs -f redis"
	@tmux select-pane -t copylingo:0.0
	@echo "--------------------------------------------------------"
	@echo "🚀 All-in-One Dashboard started in session 'copylingo'"
	@echo "Monitor App, DB, and Redis in a single screen."
	@echo "--------------------------------------------------------"
	@echo "Use: tmux attach -t copylingo"

# Kill the tmux session
tmux-stop:
	tmux kill-session -t copylingo || true

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
