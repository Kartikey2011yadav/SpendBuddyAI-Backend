# SpendBuddy AI — Backend

A production-ready, real-time expense-sharing backend built with Go. Split bills, track balances, and chat with your group — all over WebSockets.

---

## Features

- **Authentication** — Google OAuth2 + email OTP login with JWT access/refresh token pairs
- **Expense Splitting** — Equal, exact, and percentage-based splits stored as integer cents
- **Debt Simplification** — Greedy min-cash-flow algorithm to minimize the number of settlements
- **Real-time Chat** — Per-group WebSocket rooms with persistent message history
- **Live Balance Updates** — Balance changes are broadcast to all connected group members instantly
- **Clean Architecture** — Domain → Repository → Service → Delivery layers, fully interface-driven

---

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.23 |
| HTTP Framework | Echo v4 |
| Database | PostgreSQL 16 (pgxpool) |
| Cache / OTP Store | Redis 7 |
| WebSockets | gorilla/websocket |
| Auth | JWT (HS256) · Google OAuth2 · SMTP OTP |
| Containerization | Docker · Docker Compose |

---

## Project Structure

```
.
├── cmd/
│   └── server/
│       ├── main.go          # Entry point, dependency wiring, graceful shutdown
│       └── mailer.go        # SMTP email service
├── internal/
│   ├── auth/
│   │   ├── jwt.go           # Token generation & validation
│   │   ├── otp.go           # OTP generation, Redis storage, verification
│   │   └── google.go        # Google ID token verification
│   ├── chat/
│   │   ├── hub.go           # WebSocket hub (per-group rooms, broadcast)
│   │   └── client.go        # Per-connection read/write pumps, keepalive
│   ├── delivery/
│   │   └── http/
│   │       ├── router.go    # Route registration
│   │       ├── handler/
│   │       │   ├── auth.go     # Auth endpoints
│   │       │   ├── expense.go  # Expense & balance endpoints
│   │       │   └── chat.go     # WebSocket upgrade & message history
│   │       └── middleware/
│   │           └── auth.go     # JWT validation middleware
│   ├── domain/              # Models & repository interfaces
│   ├── expense/
│   │   ├── service.go       # Split creation logic
│   │   └── balance.go       # Balance aggregation & debt simplification
│   └── repository/
│       └── postgres/        # PostgreSQL implementations
├── pkg/
│   ├── config/              # Environment-based configuration
│   └── database/            # PostgreSQL pool & Redis client setup
├── migrations/
│   └── 001_schema.sql       # Full database schema
├── Dockerfile               # Multi-stage build → scratch image
├── docker-compose.yaml      # App + Postgres + Redis
├── Makefile                 # Dev, build, test, lint, migrate targets
└── .env.example             # All required environment variables
```

---

## API Reference

### Public Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/auth/google` | Sign in with a Google ID token |
| `POST` | `/auth/otp/send` | Send a 6-digit OTP to an email address |
| `POST` | `/auth/otp/verify` | Verify OTP and receive JWT token pair |
| `POST` | `/auth/refresh` | Refresh access token using a refresh token |
| `GET` | `/health` | Health check |

### Protected Endpoints (Bearer JWT required)

| Method | Path | Description |
|---|---|---|
| `GET` | `/ws/groups/:group_id` | Upgrade to WebSocket — join group room |
| `GET` | `/api/v1/groups/:group_id/messages` | Fetch last 50 messages (paginated) |
| `POST` | `/api/v1/groups/:group_id/expenses` | Create an expense with splits |
| `GET` | `/api/v1/groups/:group_id/balances` | All member balances + simplified debt list |
| `GET` | `/api/v1/groups/:group_id/balances/me` | Current user's net balance |

### Expense Split Methods

```json
{ "split_method": "equal" }
{ "split_method": "exact",      "splits": [{ "user_id": "...", "amount": 1500 }] }
{ "split_method": "percentage", "splits": [{ "user_id": "...", "percent": 60 }] }
```

All monetary values are **integer cents** (e.g. `1500` = $15.00).

---

## Getting Started

### Prerequisites

- Go 1.23+
- Docker & Docker Compose

### Run with Docker (recommended)

```bash
# 1. Copy environment file and fill in secrets
cp .env.example .env

# 2. Start all services (app + postgres + redis)
make docker-up

# Server is available at http://localhost:8080
```

### Run locally

```bash
# Start only infrastructure
docker compose up postgres redis -d

# Install dependencies
go mod download

# Run the server (hot-reload with air)
make dev

# Or build and run the binary
make run
```

---

## Configuration

Copy `.env.example` to `.env` and set the following variables:

```bash
# Server
APP_PORT=8080
APP_ENV=development

# PostgreSQL
DB_HOST=localhost
DB_PORT=5432
DB_USER=spendbuddy
DB_PASSWORD=secret
DB_NAME=spendbuddy_db
DB_SSLMODE=disable

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT (minimum 32 characters each)
JWT_ACCESS_SECRET=your-access-secret-at-least-32-chars
JWT_REFRESH_SECRET=your-refresh-secret-at-least-32-chars
JWT_ACCESS_TTL_MINUTES=15
JWT_REFRESH_TTL_DAYS=30

# Google OAuth2
GOOGLE_CLIENT_ID=your-google-client-id.apps.googleusercontent.com

# Email OTP
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your@gmail.com
SMTP_PASSWORD=your-app-password
OTP_TTL_MINUTES=10
```

> The server **panics on startup** if any required variable is missing — fast fail by design.

---

## Database Schema

The schema is applied automatically by Docker Compose on first run, or manually:

```bash
make migrate
```

Key design decisions:
- **Amounts stored as integer cents** to avoid floating-point precision issues
- **Expense splits** are inserted atomically with the parent expense in a single transaction
- **Balance aggregation** uses a single SQL round-trip with `SUM` aggregates
- `updated_at` on `users` is maintained by a PostgreSQL trigger

---

## WebSocket Protocol

Connect to `/ws/groups/:group_id` with a valid JWT in the `Authorization` header or `token` query param.

**Incoming message (client → server)**:
```json
{ "type": "text", "content": "Who paid for dinner?" }
```

**Outgoing message (server → client)**:
```json
{ "type": "text", "sender_id": "uuid", "content": "...", "created_at": "..." }
```

When a new expense is created via the REST API, the server broadcasts a `balance_update` event to all connected members of that group in real time.

---

## Development

```bash
make build        # Compile binary to ./bin/server
make run          # Build and run
make dev          # Hot-reload with air
make test         # Run tests with coverage
make lint         # golangci-lint
make docker-up    # Start all Docker services
make docker-down  # Stop all Docker services
make migrate      # Apply migrations
make migrate-down # Revert migrations
```

---

## Architecture Notes

**Clean Architecture layers:**

```
Delivery (HTTP/WS)  →  Service  →  Repository  →  Database
        ↑                  ↑              ↑
    Handler            Business       Data Access
   Middleware           Logic         (pgx/Redis)
        ↓
     Domain (interfaces + models — no dependencies)
```

**Concurrency model:**
- The WebSocket hub runs in its own goroutine
- Each client connection has independent read and write goroutines
- Balance broadcasts on expense creation are fire-and-forget (non-blocking)
- PostgreSQL connection pool: 20 max / 2 min connections

---

## License

MIT
