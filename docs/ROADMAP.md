# SpendBuddy AI — Development Roadmap

_Last updated: 2026-05-03_

This is the living development tracker for SpendBuddy AI. Each section records what is **done**, what is **in progress**, and what comes **next**. Update this file at the end of every development session.

---

## Legend

| Symbol | Meaning |
|---|---|
| ✅ | Implemented and documented |
| 🔧 | Partially implemented / needs polish |
| 🔲 | Planned but not started |
| ⚠️ | Known issue / tech debt |

---

## Phase 0 — Foundation ✅ COMPLETE

> Core infrastructure and project scaffolding.

| Item | Status | Notes |
|---|---|---|
| Go 1.23 project scaffold | ✅ | `cmd/server/main.go`, clean DI wiring |
| Echo v4 HTTP server | ✅ | Graceful shutdown on SIGINT/SIGTERM |
| PostgreSQL 16 connection pool | ✅ | pgxpool — 2 min / 20 max, health checks |
| Redis 7 client | ✅ | Pool size 10, dial/read/write timeouts |
| Environment-based config | ✅ | `pkg/config/config.go` — fail-fast on missing required vars |
| Multi-stage Docker build | ✅ | Builder (golang:1.23-alpine) → scratch, ~10-15MB image |
| docker-compose dev stack | ✅ | app + postgres + redis, named volumes, health checks |
| Makefile targets | ✅ | build, run, dev (air), test, lint, migrate, docker-up/down |
| Initial DB schema | ✅ | `migrations/001_schema.sql` |

---

## Phase 1 — Authentication ✅ COMPLETE

> Two login methods, JWT token pair, refresh flow, middleware.

| Item | Status | Notes |
|---|---|---|
| Google OAuth2 login | ✅ | `POST /auth/google` — validates ID token via Google API |
| Email OTP login | ✅ | `POST /auth/otp/send` + `POST /auth/otp/verify` |
| OTP Redis storage | ✅ | Key: `otp:{email}`, TTL from `OTP_TTL_MINUTES` |
| OTP single-use guarantee | ✅ | Deleted from Redis immediately after verify |
| SMTP mailer | ✅ | `cmd/server/mailer.go` — plain SMTP, goroutine send |
| JWT token pair (HS256) | ✅ | Separate access + refresh secrets, configurable TTLs |
| JWT access token (15 min) | ✅ | Claims: uid, email |
| JWT refresh token (30 days) | ✅ | `POST /auth/refresh` |
| JWT middleware | ✅ | `internal/delivery/http/middleware/auth.go` — injects user_id, user_email |
| Google account linking | ✅ | Links `google_sub` to existing email account |
| Auto user creation on first login | ✅ | Both Google and OTP flows create user if not found |

⚠️ **Tech Debt:**
- SMTP send is fire-and-forget with no retry — OTP delivery failures are silent
- No rate limiting on OTP send (open to abuse)
- `POST /auth/refresh` currently issues a new refresh token — spec says same one; confirm intent

---

## Phase 2 — Multi-Currency Support ✅ COMPLETE

> Currency registry, per-group currency, per-user preference, int64 monetary values.

| Item | Status | Notes |
|---|---|---|
| Currency domain registry | ✅ | `internal/domain/currency.go` — 15 ISO 4217 currencies |
| `decimal_places` per currency | ✅ | JPY=0, all others=2; drives `MinorUnitFactor()` |
| `int64` minor units throughout domain | ✅ | No float arithmetic on money in service/domain layer |
| `float64` ↔ `int64` at HTTP boundary | ✅ | `×factor` on input, `÷factor` on output |
| User `preferred_currency` field | ✅ | Defaults to `"USD"` |
| Group `currency` field | ✅ | Immutable, set at creation |
| Group currency defaults to creator's preference | ✅ | Falls back to `"USD"` if unset |
| `PUT /api/v1/users/me/currency` | ✅ | Validates against registry |
| `GET /api/v1/currencies` | ✅ | Public — returns all 15 currencies with metadata |
| DB migration `002_currencies.sql` | ✅ | Adds both columns with `DEFAULT 'USD'` |
| Currency in all balance/debt responses | ✅ | `currency` field on every amount-bearing response |

---

## Phase 3 — Group Management ✅ COMPLETE (partial)

> Groups with roles, member management.

| Item | Status | Notes |
|---|---|---|
| Create group | ✅ | `POST /api/v1/groups` — creator auto-added as admin |
| List user's groups | ✅ | `GET /api/v1/groups` — with currency visible |
| Group roles (admin/member) | ✅ | Stored in `group_members.role` |
| Add member (repo-level) | ✅ | `AddMember` with `ON CONFLICT DO NOTHING` |
| Remove member (repo-level) | ✅ | `RemoveMember` implemented |
| Is-member check (repo-level) | ✅ | Used by WebSocket auth gate |

**Missing HTTP endpoints:**

| Item | Status | Notes |
|---|---|---|
| Get group by ID | 🔲 | `GET /api/v1/groups/:group_id` |
| Update group (name/description/avatar) | 🔲 | `PATCH /api/v1/groups/:group_id` — admin only |
| Delete group | 🔲 | `DELETE /api/v1/groups/:group_id` — admin only, cascades |
| Invite member by user ID | 🔲 | `POST /api/v1/groups/:group_id/members` |
| Remove member | 🔲 | `DELETE /api/v1/groups/:group_id/members/:user_id` — admin only |
| List group members | 🔲 | `GET /api/v1/groups/:group_id/members` — with roles |
| Leave group | 🔲 | `DELETE /api/v1/groups/:group_id/members/me` |
| Transfer admin role | 🔲 | `PATCH /api/v1/groups/:group_id/members/:user_id` |

---

## Phase 4 — Expense Management ✅ COMPLETE (partial)

> Create, split, balance calculation, debt simplification.

| Item | Status | Notes |
|---|---|---|
| Create expense — equal split | ✅ | Integer division, last member absorbs remainder |
| Create expense — exact split | ✅ | Map format `{userID: amount}`, exact sum validation |
| Create expense — percentage split | ✅ | Must sum to 100; rounding to largest shareholder |
| Atomic expense + splits transaction | ✅ | pgx `SendBatch` inside a single DB transaction |
| Get group balances | ✅ | CTE aggregation SQL, returns `int64` minor units |
| Get my balance | ✅ | Single-user net balance query |
| Greedy min-cash-flow settlement engine | ✅ | `SimplifyDebts()` — exact integer comparison, no epsilon |
| Balance broadcast after expense creation | ✅ | Fire-and-forget goroutine → WebSocket hub |

**Missing endpoints:**

| Item | Status | Notes |
|---|---|---|
| List expenses in group | 🔲 | `GET /api/v1/groups/:group_id/expenses` — paginated, newest-first |
| Get expense by ID (with splits) | 🔲 | `GET /api/v1/groups/:group_id/expenses/:expense_id` |
| Delete expense | 🔲 | `DELETE /api/v1/groups/:group_id/expenses/:expense_id` — payer or admin only |
| Record a settlement payment | 🔲 | `POST /api/v1/groups/:group_id/settlements` — marks debt as paid |
| List settlement history | 🔲 | `GET /api/v1/groups/:group_id/settlements` |

⚠️ **Tech Debt:**
- No authorization check on `CreateExpense` — any authenticated user can create an expense for any group (should check group membership)
- `ListByGroup` exists in the repo but is never called from an HTTP handler

---

## Phase 5 — Real-Time Chat ✅ COMPLETE

> WebSocket hub, per-group rooms, message persistence, balance event broadcasting.

| Item | Status | Notes |
|---|---|---|
| WebSocket upgrade | ✅ | `GET /api/v1/ws/groups/:group_id` — JWT via header or `?token=` |
| Group membership gate | ✅ | Checks `IsMember` before upgrade |
| Hub — channel-based event loop | ✅ | Single goroutine, no locks on hot path |
| Client — ReadPump + WritePump goroutines | ✅ | Per-connection, 256-msg send buffer |
| Ping/pong keepalive | ✅ | Ping every 54s, pong must arrive within 60s |
| Send text/image chat message | ✅ | Client → server → all group members |
| Persist messages to PostgreSQL | ✅ | Async within ReadPump |
| `new_message` broadcast | ✅ | Envelope: `{type, group_id, payload}` |
| `balance_update` broadcast | ✅ | Triggered after expense creation |
| Message history endpoint | ✅ | `GET /api/v1/groups/:group_id/messages` — limit/offset |

⚠️ **Tech Debt:**
- WebSocket origin check returns `true` unconditionally — tighten in production
- Message persistence failure is silent (no retry, no error surfaced)
- WS endpoint is at `/ws/groups/:id` but all API routes are under `/api/v1/` — inconsistent; should be `/api/v1/ws/groups/:id`

---

## Phase 6 — User Profile 🔧 PARTIAL

> User self-service endpoints.

| Item | Status | Notes |
|---|---|---|
| Get own profile | ✅ | `GET /api/v1/users/me` |
| Update preferred currency | ✅ | `PUT /api/v1/users/me/currency` |
| Update display name | 🔲 | `PATCH /api/v1/users/me` — `{display_name}` |
| Update avatar URL | 🔲 | `PATCH /api/v1/users/me` — `{avatar_url}` |
| Search / look up user by email | 🔲 | `GET /api/v1/users?email=...` — for inviting to groups |
| Delete account | 🔲 | `DELETE /api/v1/users/me` — cascades all data |

---

## Phase 7 — Settlements 🔲 NOT STARTED

> Close the loop on debt resolution: record payments and mark debts as paid.

**Concept:** A settlement is a payment event where one user pays another. Recording it adjusts the running balance without creating an "expense".

**Schema additions:**
```sql
CREATE TABLE settlements (
    id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id     UUID        NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    payer_id     UUID        NOT NULL REFERENCES users(id),
    payee_id     UUID        NOT NULL REFERENCES users(id),
    amount_minor BIGINT      NOT NULL CHECK (amount_minor > 0),
    note         TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Endpoints needed:**
- `POST /api/v1/groups/:group_id/settlements` — record a payment
- `GET  /api/v1/groups/:group_id/settlements` — list settlement history
- Balance queries must factor in settlements (adjust CTE in `GetGroupBalances`)

**Impact:** Both `GetNetBalance` and `GetGroupBalances` SQL must be updated to subtract settled amounts from outstanding debts.

---

## Phase 8 — Expense Categories & Tags 🔲 NOT STARTED

> Classify expenses for analytics and smart suggestions.

**Proposed categories:** Food, Transport, Accommodation, Entertainment, Utilities, Shopping, Health, Travel, Other

**Schema additions:**
```sql
ALTER TABLE expenses ADD COLUMN category TEXT; -- nullable, free-form or from enum
```

Or a richer approach:
```sql
CREATE TABLE expense_categories (
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);
ALTER TABLE expenses ADD COLUMN category_id INT REFERENCES expense_categories(id);
```

**Endpoints needed:**
- `GET  /api/v1/categories` — list supported categories
- Include `category` in expense create/read payloads
- `GET  /api/v1/groups/:group_id/expenses?category=food` — filter by category

---

## Phase 9 — AI Features 🔲 NOT STARTED

> The core differentiator: on-device SLM + backend processing.

### 9a. Receipt OCR Endpoint

The mobile client extracts text via ML Kit (on-device). The backend receives the raw text and parses it into an expense suggestion.

**Endpoint:**
```
POST /api/v1/ai/parse-receipt
Body: { "raw_text": "...", "group_id": "uuid" }
Response: { "suggested_amount": 12.50, "currency": "USD", "description": "Starbucks", "category": "Food" }
```

**Implementation notes:**
- Can use a rules-based parser first (regex for amounts, keyword matching for category)
- Later: call Gemini API or run a small on-device model via the KMP client

### 9b. Smart Expense Suggestions from Chat

Parse group chat messages for expense-like mentions ("dinner was ¥8000", "I paid $45 for Uber").

**Endpoint:**
```
POST /api/v1/ai/suggest-expense
Body: { "message_id": "uuid", "group_id": "uuid" }
Response: { "suggested_amount": 45.00, "currency": "USD", "description": "Uber", "confidence": 0.87 }
```

### 9c. Auto-Categorization

When creating an expense, if no category is provided, infer it from the description.

**Implementation:** Keyword map → category lookup (e.g., "restaurant", "dinner", "ramen" → "Food"). Can be local logic or a lightweight model call.

---

## Phase 10 — Push Notifications 🔲 NOT STARTED

> Notify users of expense creation, settlement requests, and balance changes.

**Schema additions:**
```sql
CREATE TABLE device_tokens (
    id         UUID    PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT    NOT NULL UNIQUE,
    platform   TEXT    NOT NULL, -- 'android' | 'ios'
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Endpoints needed:**
- `POST /api/v1/users/me/device-token` — register FCM/APNs token
- `DELETE /api/v1/users/me/device-token` — deregister on logout

**Notification triggers:**
- New expense in a group the user is a member of
- Someone records a settlement directed at you
- Balance changes above a threshold

**Implementation approach:** Async worker goroutine reads from a channel or queue, sends via Firebase Cloud Messaging (FCM) for both Android and iOS.

---

## Phase 11 — Production Hardening 🔲 NOT STARTED

> Make the service safe to run under real load.

| Item | Priority | Notes |
|---|---|---|
| Rate limiting | High | OTP send endpoint especially; use Redis token bucket |
| Input validation middleware | High | Global validator (e.g., go-playground/validator) |
| Group membership check on expense create | High | Currently any authed user can post to any group |
| WebSocket origin validation | High | Currently returns `true` for all origins |
| CORS tighten | Medium | Replace `AllowOrigins: ["*"]` with explicit list |
| Structured request logging | Medium | Already using `slog`; add request_id, user_id to log fields |
| Prometheus metrics | Medium | Request duration, active WS connections, expense count |
| `/health` deep check | Medium | Currently returns `ok` regardless of DB/Redis state; add actual pings |
| Migration framework | Medium | Replace raw psql with `golang-migrate` for ordered, idempotent migrations |
| Retry on DB connection loss | Medium | pgxpool reconnects automatically, but startup should retry |
| Graceful WebSocket drain on shutdown | Low | Currently closes hub context; clients receive no close frame |
| Token revocation | Low | Refresh tokens are stateless — add a Redis blocklist for logout |
| SMTP retry with backoff | Low | OTP delivery can silently fail today |

---

## Phase 12 — Testing 🔲 NOT STARTED

> Confidence before shipping.

| Layer | Approach | Priority |
|---|---|---|
| Domain logic (split math, SimplifyDebts) | Table-driven unit tests (`testing` package) | High |
| Service layer | Unit tests with mock repositories (interfaces make this easy) | High |
| HTTP handlers | Integration tests using `httptest.NewRecorder` + real in-memory test DB | Medium |
| WebSocket | Test with `gorilla/websocket` client + in-memory hub | Medium |
| Currency conversion | Property tests — all 15 currencies × 3 split methods | Medium |
| End-to-end | Docker Compose spin-up + HTTP test suite | Low |

---

## Phase 13 — KMP Mobile Client Integration 🔲 NOT STARTED

> API contracts and sync behaviour for the Kotlin Multiplatform app.

| Item | Notes |
|---|---|
| Offline-first architecture | Client stores expenses locally; syncs on reconnect |
| Optimistic UI | Client adds expense to local state before server confirms |
| Conflict resolution | Server is source of truth; timestamps decide winner on conflict |
| API pagination contract | Cursor-based for expenses (`before` timestamp), offset for messages |
| Device token registration | Phase 10 prerequisite |
| Gemini Nano on-device parsing | ML Kit OCR → SLM → `POST /api/v1/ai/parse-receipt` |

---

## Backlog (Unscheduled)

| Idea | Notes |
|---|---|
| Expense edit history / audit log | Immutable append-only log for all financial changes |
| Group spending analytics | Totals by category, by member, over time |
| Recurring expenses | Weekly rent, monthly subscriptions |
| CSV / PDF export | Group expense report |
| Group invitation via shareable link | Join via UUID token, no email required |
| Multiple group admin roles | Admin, moderator, member hierarchy |
| Expense splitting by shares | "Alice gets 2 shares, Bob gets 1" — generalizes equal split |
| Budget caps per group | Alert when group crosses a threshold |
| Multi-group user dashboard | Aggregate "you owe X across all groups" |

---

## Iteration Log

| Date | Phase | What was done |
|---|---|---|
| 2026-05-02 | Foundation → Auth | Initial scaffold: Echo router, JWT middleware, Google OAuth, Email OTP, WebSocket Hub, group/expense/message repos, Clean Architecture structure |
| 2026-05-02 | Phase 2 (prep) | Migrated all monetary domain fields from `float64` to `int64` minor units; removed `roundCents()`; rewrote `computeSplits()` with integer arithmetic; updated balance engine and all repositories |
| 2026-05-03 | Phase 2 | Multi-currency support: 15-currency registry, per-group currency, user preferred currency, currency-aware handler conversion, new user/group handlers, 5 new endpoints, DB migration 002 |
| 2026-05-03 | Docs | Full doc refresh: api.md, data-models.md, architecture.md, expense-splitting.md, websocket.md, environment.md updated to reflect all changes; pushed to `main` (commit `a4f0299`) |
