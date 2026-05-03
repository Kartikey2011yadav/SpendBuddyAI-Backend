# Architecture Overview

## What We Are Building

SpendBuddy AI is a real-time expense-sharing backend. Users join groups, add expenses with flexible split methods, and see live balance updates via WebSocket. Authentication supports Google OAuth2 and email OTP. Groups each have their own settlement currency chosen from 15 supported ISO 4217 currencies.

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
│   │   ├── currency.go  # Static registry of 15 supported currencies
│   │   └── interfaces.go
│   │
│   ├── auth/            # Authentication services
│   │   ├── jwt.go       # JWT generate/validate (HS256)
│   │   ├── otp.go       # Email OTP with Redis storage
│   │   └── google.go    # Google ID token validation
│   │
│   ├── expense/         # Expense business logic
│   │   ├── service.go   # Split computation (equal/exact/percentage) — int64 throughout
│   │   └── balance.go   # Net balance + greedy min-cash-flow debt simplification
│   │
│   ├── chat/            # WebSocket real-time layer
│   │   ├── hub.go       # Central hub: rooms per group, broadcast
│   │   └── client.go    # Per-connection read/write pumps + keepalive
│   │
│   ├── delivery/http/
│   │   ├── router.go            # Echo route registration
│   │   ├── handler/
│   │   │   ├── auth.go          # Auth endpoints
│   │   │   ├── user.go          # GET /users/me, PUT /users/me/currency
│   │   │   ├── group.go         # POST /groups, GET /groups
│   │   │   ├── expense.go       # Expense + balance endpoints (currency-aware conversion)
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
│   ├── 001_schema.sql           # Initial DB schema
│   └── 002_currencies.sql       # Adds preferred_currency + group currency columns
│
├── Dockerfile                   # Multi-stage: builder → scratch
├── docker-compose.yaml          # App + PostgreSQL + Redis
├── Makefile
└── .env.example
```

---

## Dependency Injection

All wiring is in `cmd/server/main.go` — no global state, no `init()` magic:

```
Infrastructure  →  Repositories  →  Services  →  Handlers         →  Router
(db, redis)        (user, group,     (jwt, otp,    (auth, user,        (echo)
                    expense, msg)     google,        group, expense,
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
| `int64` minor units everywhere in domain + service | No floating-point arithmetic on money — eliminates rounding errors |
| `float64` ↔ `int64` conversion only at HTTP handler | Single boundary using `MinorUnitFactor(currency)` — `×factor` on input, `÷factor` on output |
| Currency per-group, not per-expense | Avoids FX conversion complexity; all balances in one currency per group |
| Static currency registry in domain layer | 15 currencies with `DecimalPlaces` drives correct conversion (JPY=0, USD=2) |
| User `preferred_currency` → group default | Convenience; creator's preference pre-fills group currency at creation |
| Interfaces at domain layer | All layers depend on abstractions, enabling easy testing/mocking |
| No ORM (raw pgx) | Full control over query plans; CTEs for balance aggregation |
| Redis for OTP | OTP is ephemeral with TTL; Redis is the right tool |
| Greedy min-cash-flow for debt simplification | Minimises number of settlement transactions |
| Scratch Docker image | Minimal attack surface, ~10MB image |
| Fail-fast config | `mustEnv()` panics on missing required vars at startup |

---

## Related Docs

- [API Reference](api.md) — all endpoints, request/response shapes
- [Data Models](data-models.md) — domain structs and DB schema
- [Auth Flows](auth-flows.md) — Google OAuth2, Email OTP, JWT refresh
- [WebSocket Protocol](websocket.md) — real-time chat and balance updates
- [Expense Splitting](expense-splitting.md) — split methods and debt simplification
