# Environment & Configuration

## Running Locally

```bash
# Copy and fill in env vars
cp .env.example .env

# Start dependencies
make docker-up      # or: docker compose up postgres redis -d

# Run the server
make run            # builds + runs
make dev            # hot-reload with air (requires: go install github.com/air-verse/air@latest)

# Apply DB schema
make migrate
```

---

## Environment Variables

### App

| Variable | Default | Required | Description |
|---|---|---|---|
| `APP_PORT` | `8080` | No | HTTP server port |
| `APP_ENV` | `development` | No | `development` or `production` |

### PostgreSQL

| Variable | Default | Required | Description |
|---|---|---|---|
| `DB_HOST` | `localhost` | Yes | DB host |
| `DB_PORT` | `5432` | Yes | DB port |
| `DB_USER` | — | Yes | DB user |
| `DB_PASSWORD` | — | Yes | DB password |
| `DB_NAME` | — | Yes | Database name |
| `DB_SSLMODE` | `disable` | No | `disable`, `require`, etc. |

### Redis

| Variable | Default | Required | Description |
|---|---|---|---|
| `REDIS_ADDR` | `localhost:6379` | Yes | Redis address |
| `REDIS_PASSWORD` | `""` | No | Redis password |
| `REDIS_DB` | `0` | No | Redis DB index |

### JWT

| Variable | Default | Required | Description |
|---|---|---|---|
| `JWT_ACCESS_SECRET` | — | Yes | HS256 signing secret for access tokens |
| `JWT_REFRESH_SECRET` | — | Yes | HS256 signing secret for refresh tokens |
| `JWT_ACCESS_TTL_MINUTES` | `15` | No | Access token lifetime in minutes |
| `JWT_REFRESH_TTL_DAYS` | `30` | No | Refresh token lifetime in days |

Use distinct, high-entropy secrets for access and refresh (e.g., `openssl rand -hex 32`).

### Google OAuth2

| Variable | Default | Required | Description |
|---|---|---|---|
| `GOOGLE_CLIENT_ID` | — | Yes | OAuth2 client ID from Google Cloud Console |

### SMTP / OTP

| Variable | Default | Required | Description |
|---|---|---|---|
| `SMTP_HOST` | — | Yes | SMTP server host |
| `SMTP_PORT` | `587` | No | SMTP server port |
| `SMTP_USER` | — | Yes | SMTP login (usually the from-address) |
| `SMTP_PASSWORD` | — | Yes | SMTP password or app password |
| `OTP_TTL_MINUTES` | `10` | No | OTP expiry in minutes |

---

## Config Loading

**File:** `pkg/config/config.go`

- Loads `.env` via `godotenv` if present (no-op in production containers)
- `getEnv(key, fallback)` — returns fallback if env var is missing
- `mustEnv(key)` — panics at startup if required var is missing

This is a fail-fast design: misconfiguration is caught immediately at boot, not at first request.

---

## Docker Compose (Development)

```bash
make docker-up      # Start app + postgres + redis
make docker-down    # Stop all services
```

Services:
- `app` — Go server (port 8080)
- `postgres` — PostgreSQL 16 (port 5432)
- `redis` — Redis 7 (port 6379)

Volumes persist DB and Redis data across restarts.

---

## Dockerfile (Production)

Multi-stage build:

```
Stage 1: golang:1.23-alpine
  - Compile with CGO_ENABLED=0 GOOS=linux
  - Output: statically linked binary

Stage 2: scratch
  - Copy CA certificates (for HTTPS/Google API calls)
  - Copy binary
  - Result: ~10-15MB image
```

Build: `docker build -t spendbuddy-api .`

---

## Database Migrations

```bash
make migrate
# Equivalent to: psql $DATABASE_URL -f migrations/001_schema.sql
```

Migrations are plain SQL files in `/migrations/`. There is no migration framework currently — apply in order by filename.

---

## Makefile Commands

| Command | Description |
|---|---|
| `make build` | Compile binary to `bin/server` |
| `make run` | Build and run the server |
| `make dev` | Hot-reload with `air` |
| `make test` | Run tests with race detector |
| `make lint` | Run `golangci-lint` |
| `make migrate` | Apply DB schema |
| `make docker-up` | Start full stack via Docker Compose |
| `make docker-down` | Stop Docker Compose services |
