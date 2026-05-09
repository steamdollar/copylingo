.PHONY: build run test clean docker-up docker-down migrate dev app-up app-logs restart-db restart-redis infra tunnel tmux tmux-stop

# Build the application
build:
	go build -o bin/copylingo ./cmd/server

# Run locally
run:
	go run ./cmd/server

# Run with hot reload (requires air)
dev: infra
	docker stop copylingo-app || true
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "[copylingo] air not found; falling back to go run ./cmd/server"; \
		go run ./cmd/server; \
	fi

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
	@pkill -f '^cloudflared tunnel --url http://localhost:8080$$' 2>/dev/null || true
	@pkill -f '^go run ./cmd/server$$' 2>/dev/null || true
	@if command -v fuser >/dev/null 2>&1; then fuser -k 8080/tcp 2>/dev/null || true; fi
	@docker stop copylingo-app 2>/dev/null || true
	@stamp=$$(date +%s); \
	tmux new-session -d -s copylingo -n 'Dashboard' "make tunnel"; \
	echo "[copylingo] waiting for tunnel URL before starting app..."; \
	ready=0; \
	deadline=$$(( $$(date +%s) + 60 )); \
	while [ $$(date +%s) -le $$deadline ]; do \
		if [ -f .env ] && [ $$(stat -c %Y .env) -ge $$stamp ] && grep -Eq '^COPYLINGO_SERVER_PUBLIC_BASE_URL=https://[[:alnum:]-]+\.trycloudflare\.com/?$$' .env; then \
			ready=1; \
			break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ready" != "1" ]; then \
		echo "[copylingo] tunnel URL was not ready within 60s" >&2; \
		tmux kill-session -t copylingo 2>/dev/null || true; \
		exit 1; \
	fi; \
	tmux split-window -h -t copylingo "make dev"; \
	tmux split-window -v -t copylingo "docker compose logs -f postgres"; \
	tmux split-window -v -t copylingo:0.2 "docker compose logs -f redis"; \
	tmux select-layout -t copylingo tiled
	@tmux select-pane -t copylingo:0.0
	@echo "--------------------------------------------------------"
	@echo "🚀 All-in-One Dashboard started in session 'copylingo'"
	@echo "Monitor Tunnel, App, DB, and Redis in a single screen."
	@echo "--------------------------------------------------------"
	@echo "Use: tmux attach -t copylingo"

# Kill the tmux session
tmux-stop:
	tmux kill-session -t copylingo || true
	pkill -f '^cloudflared tunnel --url http://localhost:8080$$' 2>/dev/null || true
	pkill -f '^go run ./cmd/server$$' 2>/dev/null || true
	@if command -v fuser >/dev/null 2>&1; then fuser -k 8080/tcp 2>/dev/null || true; fi

# Run database migration (requires psql)
migrate:
	psql -h localhost -U copylingo -d copylingo -f migrations/001_init.up.sql

migrate-down:
	psql -h localhost -U copylingo -d copylingo -f migrations/001_init.down.sql

# Development: start infra only (DB + Redis)
infra:
	docker compose up -d postgres redis

# Development: start Cloudflare Quick Tunnel and update .env public base URL
tunnel:
	./scripts/start_quick_tunnel.sh

# Lint
lint:
	golangci-lint run ./...

# Download dependencies
deps:
	go mod tidy
	go mod download
