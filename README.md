# Market Assistant

AI-powered WhatsApp business assistant for Nigerian small businesses.

## Current status

Phase 1 — Foundation:

- Task 1: Go module, environment configuration, HTTP server, health endpoint, structured logging, timeouts, graceful shutdown, and configuration tests.
- Task 2: PostgreSQL foundation using `pgx/v5/pgxpool`, connection-pool configuration, startup connectivity check, context-aware ping, and clean pool shutdown.

## Requirements

- Go 1.22.x (matches the current restricted development environment)
- PostgreSQL
- `github.com/jackc/pgx/v5`
- Redis will be added when background processing requires it.

## Run

Copy the example environment file:

```bash
cp .env.example .env
```

Export the variables in your shell, or load them with your preferred environment-file tool. PostgreSQL must be reachable before the server starts.

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/market_assistant?sslmode=disable'
GOTOOLCHAIN=local go run ./cmd/server
```

The server creates the PostgreSQL pool and performs a bounded connectivity check before accepting HTTP traffic.

Health check:

```bash
curl http://localhost:8080/api/health
```

Expected response:

```json
{"status":"ok"}
```

## Test

```bash
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go vet ./...
GOTOOLCHAIN=local go build ./...
```
