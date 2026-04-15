# Architecture Overview

## What We Are Building

SpendBuddy AI is a real-time expense-sharing backend. Users join groups, add expenses with flexible split methods, and see live balance updates via WebSocket. Authentication supports Google OAuth2 and email OTP.

---

## Layer Structure (Clean Architecture)

```
HTTP Request
    │
    ▼
┌─────────────────────────────────────┐
│           Delivery Layer            │
│  Echo Router → Middleware → Handler │
│  /internal/delivery/http/           │
└──────────────────┬──────────────────┘
                   │
                   ▼
┌─────────────────────────────────────┐
│           Service Layer             │
│  Business logic, split computation  │
│  /internal/auth/, /internal/expense/│
└──────────────────┬──────────────────┘
                   │
                   ▼
┌─────────────────────────────────────┐
│         Repository Layer            │
│  SQL queries via pgx, no ORM        │
│  /internal/repository/postgres/     │
└──────────────────┬──────────────────┘
                   │
                   ▼
┌─────────────────────────────────────┐
│         Infrastructure              │
│  PostgreSQL 16 + Redis 7            │
│  /pkg/database/                     │
└─────────────────────────────────────┘
```

All layer boundaries are defined by interfaces in `/internal/domain/interfaces.go`. No layer imports a layer below it directly — only its interface.

---

## Directory Map

```
SpendBuddyAI-Backend/
├── cmd/server/
│   ├── main.go          # Dependency injection wiring + graceful shutdown
│   └── mailer.go        # SMTP OTP sender implementation
│
├── internal/
│   ├── domain/          # Models + interfaces (no dependencies)
│   │   ├── user.go
│   │   ├── expense.go
│   │   ├── group.go
│   │   ├── message.go
│   │   └── interfaces.go
│   │
│   ├── auth/            # Authentication services
│   │   ├── jwt.go       # JWT generate/validate (HS256)
│   │   ├── otp.go       # Email OTP with Redis storage
│   │   └── google.go    # Google ID token validation
│   │
│   ├── expense/         # Expense business logic
│   │   ├── service.go   # Split computation (equal/exact/percentage)
│   │   └── balance.go   # Net balance + debt simplification algorithm
│   │
│   ├── chat/            # WebSocket real-time layer
│   │   ├── hub.go       # Central hub: rooms per group, broadcast
│   │   └── client.go    # Per-connection read/write pumps + keepalive
│   │
│   ├── delivery/http/
│   │   ├── router.go            # Echo route registration
│   │   ├── handler/
│   │   │   ├── auth.go          # Auth endpoints
│   │   │   ├── expense.go       # Expense + balance endpoints
│   │   │   └── chat.go          # WS upgrade + message history
│   │   └── middleware/
│   │       └── auth.go          # JWT validation, injects user_id to context
│   │
│   └── repository/postgres/
│       ├── user.go
│       ├── group.go
│       ├── expense.go
│       └── message.go
│
├── pkg/
│   ├── config/config.go         # Env-based config (panics on missing required)
│   └── database/
│       ├── postgres.go          # pgxpool setup (20 max connections)
│       └── redis.go             # Redis client setup
│
├── migrations/
│   └── 001_schema.sql           # Full DB schema
│
├── Dockerfile                   # Multi-stage: builder → scratch
├── docker-compose.yaml          # App + PostgreSQL + Redis
├── Makefile
└── .env.example
```

---

## Dependency Injection

All wiring is in `cmd/server/main.go` — no global state, no init() magic:

```
Infrastructure  →  Repositories  →  Services  →  Handlers  →  Router
(db, redis)        (user, group,     (jwt, otp,    (auth,        (echo)
                    expense, msg)     google,        expense,
                                      expense)       chat)
```

The WebSocket hub is started as a goroutine before the server starts and shut down on SIGINT/SIGTERM via context cancellation.

---

## Concurrency Model

| Component | Goroutine Strategy |
|---|---|
| WebSocket Hub | Single event-loop goroutine with channel-based message routing |
| WS Client | Two goroutines: `ReadPump` + `WritePump` per connection |
| Balance Broadcast | Fire-and-forget goroutine after expense creation |
| DB Pool | pgxpool: 2 min / 20 max connections |
| Redis Pool | 10 pool size |

Hub uses an `RWMutex` to protect the room map. Client send buffer is 256 messages; if full, the client is dropped without blocking the hub.

---

## Key Design Decisions

| Decision | Rationale |
|---|---|
| Amounts stored as integer cents | Avoids floating-point rounding errors in aggregation |
| Interfaces at domain layer | All layers depend on abstractions, enabling easy testing |
| No ORM (raw pgx) | Full control over query plans; CTEs for balance aggregation |
| Redis for OTP | OTP is ephemeral with TTL; Redis is the right tool |
| Amounts in cents → float at service layer | DB stores cents; service exposes float64 for API ergonomics |
| Greedy min-cash-flow for debt simplification | Minimizes number of settlement transactions |
| Scratch Docker image | Minimal attack surface, ~10MB image |
| Fail-fast config | `mustEnv()` panics on missing required vars at startup |

---

## Related Docs

- [API Reference](api.md) — all endpoints, request/response shapes
- [Data Models](data-models.md) — domain structs and DB schema
- [Auth Flows](auth-flows.md) — Google OAuth2, Email OTP, JWT refresh
- [WebSocket Protocol](websocket.md) — real-time chat and balance updates
- [Expense Splitting](expense-splitting.md) — split methods and debt simplification
