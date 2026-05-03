# Data Models

## Domain Structs (Go)

### Currency

```go
// /internal/domain/currency.go
type Currency struct {
    Code          string `json:"code"`
    Symbol        string `json:"symbol"`
    Name          string `json:"name"`
    DecimalPlaces int    `json:"decimal_places"`
}
```

The registry of 15 supported currencies lives in `domain.Currencies` (a `map[string]Currency`). Use `domain.IsSupported(code)` for validation and `domain.MinorUnitFactor(code)` to get `10^DecimalPlaces` (e.g., `100` for USD, `1` for JPY).

---

### User

```go
// /internal/domain/user.go
type User struct {
    ID                uuid.UUID
    GoogleSub         *string    // nil if email-only account
    Email             string
    DisplayName       string
    AvatarURL         *string
    IsEmailVerified   bool
    PreferredCurrency string     // ISO 4217 code, default "USD"
    CreatedAt         time.Time
    UpdatedAt         time.Time
}

type OTPRecord struct {
    Email     string
    Code      string
    ExpiresAt time.Time
}
```

`PreferredCurrency` is used as the default currency when the user creates a new group.

---

### Group

```go
// /internal/domain/group.go
type Group struct {
    ID          uuid.UUID
    Name        string
    Description *string
    AvatarURL   *string
    CreatedBy   uuid.UUID
    Currency    string    // ISO 4217 code — set at creation, immutable
    CreatedAt   time.Time
}

type GroupRole string
const (
    RoleAdmin  GroupRole = "admin"
    RoleMember GroupRole = "member"
)

type GroupMember struct {
    GroupID  uuid.UUID
    UserID   uuid.UUID
    Role     GroupRole
    JoinedAt time.Time
}
```

`Currency` is the single settlement currency for all expenses in the group. It is set at group creation and never changes.

---

### Expense

```go
// /internal/domain/expense.go
type SplitMethod string
const (
    SplitEqual      SplitMethod = "equal"
    SplitExact      SplitMethod = "exact"
    SplitPercentage SplitMethod = "percentage"
)

type Expense struct {
    ID          uuid.UUID
    GroupID     uuid.UUID
    PayerID     uuid.UUID
    Amount      int64       // integer minor units (cents for USD/EUR, whole units for JPY)
    Description string
    SplitMethod SplitMethod
    CreatedAt   time.Time
}

type ExpenseSplit struct {
    ID         uuid.UUID
    ExpenseID  uuid.UUID
    UserID     uuid.UUID
    AmountOwed int64     // integer minor units
}

type UserBalance struct {
    UserID      uuid.UUID
    DisplayName string
    NetBalance  int64  // integer minor units. Positive: owed to you. Negative: you owe others.
}

type DebtSummary struct {
    FromUserID   uuid.UUID
    FromUserName string
    ToUserID     uuid.UUID
    ToUserName   string
    Amount       int64  // integer minor units
}
```

All monetary fields are `int64` minor units throughout the domain and service layers. Conversion to/from human-readable `float64` happens **only** at the HTTP handler boundary, using `domain.MinorUnitFactor(groupCurrency)`.

---

### Message

```go
// /internal/domain/message.go
type MessageType string
const (
    MessageText   MessageType = "text"
    MessageImage  MessageType = "image"
    MessageSystem MessageType = "system"
)

type Message struct {
    ID           uuid.UUID
    GroupID      uuid.UUID
    UserID       uuid.UUID
    Content      string
    Type         MessageType
    CreatedAt    time.Time
    SenderName   string   // Populated on read (JOIN)
    SenderAvatar *string  // Populated on read (JOIN)
}

type WSMessage struct {
    Type    string      // "new_message" | "balance_update"
    GroupID string
    Payload interface{}
}
```

---

## Database Schema

### Migration files

| File | Description |
|---|---|
| `migrations/001_schema.sql` | Initial schema: users, groups, group_members, expenses, expense_splits, messages |
| `migrations/002_currencies.sql` | Adds `preferred_currency` to `users`, `currency` to `groups` |

Apply in order:
```bash
psql $DATABASE_URL -f migrations/001_schema.sql
psql $DATABASE_URL -f migrations/002_currencies.sql
```

---

### users

```sql
CREATE TABLE users (
    id                 UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    google_sub         TEXT        UNIQUE,            -- NULL for email-only accounts
    email              TEXT        NOT NULL UNIQUE,
    display_name       TEXT        NOT NULL,
    avatar_url         TEXT,
    is_email_verified  BOOLEAN     NOT NULL DEFAULT FALSE,
    preferred_currency CHAR(3)     NOT NULL DEFAULT 'USD',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Trigger: auto-updates updated_at on every UPDATE
```

---

### groups

```sql
CREATE TABLE groups (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT        NOT NULL,
    description TEXT,
    avatar_url  TEXT,
    created_by  UUID        NOT NULL REFERENCES users(id),
    currency    CHAR(3)     NOT NULL DEFAULT 'USD',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

### group_members

```sql
CREATE TYPE group_role AS ENUM ('admin', 'member');

CREATE TABLE group_members (
    group_id  UUID       NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   UUID       NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    role      group_role NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);
```

---

### expenses

```sql
CREATE TYPE split_method AS ENUM ('equal', 'exact', 'percentage');

CREATE TABLE expenses (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id      UUID         NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    payer_id      UUID         NOT NULL REFERENCES users(id),
    amount_cents  BIGINT       NOT NULL CHECK (amount_cents > 0),
    description   TEXT         NOT NULL,
    split_method  split_method NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_expenses_group_created ON expenses(group_id, created_at DESC);
CREATE INDEX idx_expenses_payer         ON expenses(payer_id);
```

The column is named `amount_cents` for all currencies. For JPY (0 decimal places), the value stored IS the whole-yen amount (e.g., ¥3000 → `3000`). The name is a historical artifact; the value is always "minor units" of the group's currency.

---

### expense_splits

```sql
CREATE TABLE expense_splits (
    id                UUID   PRIMARY KEY DEFAULT uuid_generate_v4(),
    expense_id        UUID   NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    user_id           UUID   NOT NULL REFERENCES users(id),
    amount_owed_cents BIGINT NOT NULL CHECK (amount_owed_cents >= 0),
    UNIQUE (expense_id, user_id)
);
```

---

### messages

```sql
CREATE TYPE message_type AS ENUM ('text', 'image', 'system');

CREATE TABLE messages (
    id         UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id   UUID         NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id    UUID         NOT NULL REFERENCES users(id),
    content    TEXT         NOT NULL,
    type       message_type NOT NULL DEFAULT 'text',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_group_created ON messages(group_id, created_at DESC);
```

---

## Entity Relationship Diagram

```
users
  │
  ├──< group_members >──┐
  │                     │
  │                   groups  (currency → ISO 4217)
  │                     │
  ├──< expenses         │  (payer_id → users, group_id → groups)
  │       │
  │       └──< expense_splits  (user_id → users)
  │
  └──< messages  (group_id → groups)
```

---

## Monetary Value Contract

All monetary values are `int64` **minor units** throughout the domain and service layers. There is no `float64` arithmetic on money.

| Layer | Type | Example (USD $15.00) | Example (JPY ¥3000) |
|---|---|---|---|
| Database (`amount_cents`) | `BIGINT` | `1500` | `3000` |
| Domain / Service | `int64` | `1500` | `3000` |
| HTTP handler (input) | `float64` → `int64` via `×factor` | `15.00 × 100 = 1500` | `3000 × 1 = 3000` |
| HTTP handler (output) | `int64` → `float64` via `÷factor` | `1500 ÷ 100 = 15.00` | `3000 ÷ 1 = 3000` |
| WebSocket broadcast | `int64` (raw minor units) | `1500` | `3000` |

`factor = domain.MinorUnitFactor(currencyCode)` = `10^DecimalPlaces` (100 for most currencies, 1 for JPY).

WebSocket `balance_update` payloads send raw `int64` minor units. Clients should use the group's `currency` (and its `decimal_places` from `GET /api/v1/currencies`) to format for display.
