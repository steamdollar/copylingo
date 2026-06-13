.PHONY: build run test clean docker-up docker-down migrate dev app-up app-logs restart-app restart-db restart-redis infra tunnel tmux tmux-stop

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

# Restart the local app process without touching tunnel, DB, or Redis
restart-app:
	@echo "[copylingo] restarting app..."
	@if ! tmux has-session -t copylingo 2>/dev/null; then \
		echo "[copylingo] tmux session 'copylingo' is not running. Start it with 'make tmux' first." >&2; \
		exit 1; \
	fi
	@pane=$$(tmux list-panes -t copylingo -F '#{pane_id} #{pane_index} #{pane_current_command} #{pane_title}' | awk '$$4=="copylingo-app"{print $$1; exit}'); \
	if [ -z "$$pane" ]; then \
		pane=$$(tmux list-panes -t copylingo -F '#{pane_id} #{pane_index} #{pane_current_command}' | awk '$$2!="0" && $$3=="make"{print $$1; exit}'); \
	fi; \
	if [ -n "$$pane" ]; then \
		tmux kill-pane -t "$$pane" 2>/dev/null || true; \
	fi; \
	docker stop copylingo-app 2>/dev/null || true; \
	pkill -f '^air$$' 2>/dev/null || true; \
	pkill -f '^go run ./cmd/server$$' 2>/dev/null || true; \
	if command -v fuser >/dev/null 2>&1; then fuser -k 8080/tcp 2>/dev/null || true; fi; \
	new_pane=$$(tmux split-window -h -P -F '#{pane_id}' -t copylingo:0.0 'make dev'); \
	tmux select-pane -T copylingo-app -t "$$new_pane"; \
	tmux select-layout -t copylingo tiled >/dev/null
	@for i in $$(seq 1 30); do \
		if curl -fsS http://localhost:8080/health >/dev/null 2>&1; then \
			echo "[copylingo] app is ready: http://localhost:8080/health"; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "[copylingo] app did not become ready within 30s" >&2; \
	exit 1

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
	tmux select-pane -T copylingo-tunnel -t copylingo:0.0; \
	app_pane=$$(tmux split-window -h -P -F '#{pane_id}' -t copylingo "make dev"); \
	tmux select-pane -T copylingo-app -t "$$app_pane"; \
	db_pane=$$(tmux split-window -v -P -F '#{pane_id}' -t copylingo "docker compose logs -f postgres"); \
	tmux select-pane -T copylingo-postgres -t "$$db_pane"; \
	redis_pane=$$(tmux split-window -v -P -F '#{pane_id}' -t "$$db_pane" "docker compose logs -f redis"); \
	tmux select-pane -T copylingo-redis -t "$$redis_pane"; \
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
# Applies every migrations/NNN_*.sql in filename order.
migrate:
	@for f in $(sort $(wildcard migrations/[0-9]*.sql)); do \
		echo "==> Applying $$f"; \
		psql -h localhost -U copylingo -d copylingo -v ON_ERROR_STOP=1 -f $$f || exit 1; \
	done

# Development: start infra only (DB + Redis)
infra:
	docker compose up -d postgres redis

# Development: start Cloudflare Quick Tunnel and update .env public base URL
tunnel:
	./scripts/start_quick_tunnel.sh

# Lint (진단만)
lint:
	golangci-lint run ./...

# Format: gofmt/goimports/golines 자동 적용 (긴 라인 자동 줄바꿈) + 자동수정 가능한 린트 fix
fmt:
	golangci-lint fmt ./...
	golangci-lint run --fix ./...

# golangci-lint v2 설치 (.golangci.yml 사용)
lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# git pre-commit hook 활성화 (commit 시 staged .go 자동 포맷/줄바꿈)
hooks:
	chmod +x scripts/git-hooks/*
	git config core.hooksPath scripts/git-hooks
	@echo "pre-commit hook 활성화됨 (core.hooksPath=scripts/git-hooks)"

# Download dependencies
deps:
	go mod tidy
	go mod download
