# Data Models

## Domain Structs (Go)

### User

```go
// /internal/domain/user.go
type User struct {
    ID              uuid.UUID
    GoogleSub       *string    // nil if email-only account
    Email           string
    DisplayName     string
    AvatarURL       *string
    IsEmailVerified bool
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

type OTPRecord struct {
    Email     string
    Code      string
    ExpiresAt time.Time
}
```

### Group

```go
// /internal/domain/group.go
type Group struct {
    ID          uuid.UUID
    Name        string
    Description *string
    AvatarURL   *string
    CreatedBy   uuid.UUID
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
    Amount      float64      // Dollars (converted from cents at repo layer)
    Description string
    SplitMethod SplitMethod
    CreatedAt   time.Time
}

type ExpenseSplit struct {
    ID         uuid.UUID
    ExpenseID  uuid.UUID
    UserID     uuid.UUID
    AmountOwed float64
}

type UserBalance struct {
    UserID      uuid.UUID
    DisplayName string
    NetBalance  float64  // Positive: owed to you. Negative: you owe others.
}

type DebtSummary struct {
    FromUserID   uuid.UUID
    FromUserName string
    ToUserID     uuid.UUID
    ToUserName   string
    Amount       float64
}
```

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

File: `migrations/001_schema.sql`

### users

```sql
CREATE TABLE users (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    google_sub       TEXT UNIQUE,            -- NULL for email-only accounts
    email            TEXT NOT NULL UNIQUE,
    display_name     TEXT NOT NULL,
    avatar_url       TEXT,
    is_email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Trigger: auto-updates updated_at on every UPDATE
```

### groups

```sql
CREATE TABLE groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT,
    avatar_url  TEXT,
    created_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### group_members

```sql
CREATE TYPE group_role AS ENUM ('admin', 'member');

CREATE TABLE group_members (
    group_id  UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role      group_role NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);
```

### expenses

```sql
CREATE TYPE split_method AS ENUM ('equal', 'exact', 'percentage');

CREATE TABLE expenses (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id      UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    payer_id      UUID NOT NULL REFERENCES users(id),
    amount_cents  BIGINT NOT NULL,          -- Stored as integer cents, no floats
    description   TEXT NOT NULL,
    split_method  split_method NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_expenses_group_created ON expenses(group_id, created_at);
CREATE INDEX idx_expenses_payer        ON expenses(payer_id);
```

### expense_splits

```sql
CREATE TABLE expense_splits (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id       UUID NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    user_id          UUID NOT NULL REFERENCES users(id),
    amount_owed_cents BIGINT NOT NULL,
    UNIQUE (expense_id, user_id)
);
```

### messages

```sql
CREATE TYPE message_type AS ENUM ('text', 'image', 'system');

CREATE TABLE messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id   UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id),
    content    TEXT NOT NULL,
    type       message_type NOT NULL DEFAULT 'text',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
  │                   groups
  │                     │
  ├──< expenses         │  (payer_id → users, group_id → groups)
  │       │
  │       └──< expense_splits  (user_id → users)
  │
  └──< messages  (group_id → groups)
```

---

## Amounts: Cents vs Dollars

Amounts are stored in the DB as integer cents (`BIGINT`) to avoid floating-point precision issues in aggregation queries.

The repository layer converts:
- **DB → Service**: multiply cents by 0.01 → `float64`
- **Service → DB**: multiply dollars by 100 → `int64`

Example: `$45.67` is stored as `4567` in `amount_cents`.

The balance aggregation SQL does all arithmetic in integer cents, then the repository converts the final result to float64 before returning to the service layer.
