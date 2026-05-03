# API Reference

Base URL: `http://localhost:8080` (configurable via `APP_PORT`)

All protected routes require:
```
Authorization: Bearer <access_token>
```

---

## Public Endpoints

### Health Check

```
GET /health
```

Response `200`:
```json
{ "status": "ok" }
```

---

### GET /api/v1/currencies

List all 15 supported currencies. No authentication required — useful for building currency pickers in the client.

**Response `200`:**
```json
{
  "currencies": [
    { "code": "USD", "symbol": "$",   "name": "US Dollar",        "decimal_places": 2 },
    { "code": "EUR", "symbol": "€",   "name": "Euro",             "decimal_places": 2 },
    { "code": "GBP", "symbol": "£",   "name": "British Pound",    "decimal_places": 2 },
    { "code": "JPY", "symbol": "¥",   "name": "Japanese Yen",     "decimal_places": 0 },
    { "code": "CAD", "symbol": "CA$", "name": "Canadian Dollar",  "decimal_places": 2 },
    { "code": "AUD", "symbol": "A$",  "name": "Australian Dollar","decimal_places": 2 },
    { "code": "CHF", "symbol": "Fr",  "name": "Swiss Franc",      "decimal_places": 2 },
    { "code": "CNY", "symbol": "¥",   "name": "Chinese Yuan",     "decimal_places": 2 },
    { "code": "INR", "symbol": "₹",   "name": "Indian Rupee",     "decimal_places": 2 },
    { "code": "SGD", "symbol": "S$",  "name": "Singapore Dollar", "decimal_places": 2 },
    { "code": "HKD", "symbol": "HK$", "name": "Hong Kong Dollar", "decimal_places": 2 },
    { "code": "NOK", "symbol": "kr",  "name": "Norwegian Krone",  "decimal_places": 2 },
    { "code": "SEK", "symbol": "kr",  "name": "Swedish Krona",    "decimal_places": 2 },
    { "code": "MXN", "symbol": "MX$", "name": "Mexican Peso",     "decimal_places": 2 },
    { "code": "BRL", "symbol": "R$",  "name": "Brazilian Real",   "decimal_places": 2 }
  ]
}
```

---

### POST /auth/google

Login or register with a Google ID token.

**Request:**
```json
{
  "id_token": "eyJhbGciOiJSUzI1NiIsImtpZCI6..."
}
```

**Response `200`:**
```json
{
  "tokens": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "display_name": "Jane Doe",
    "avatar_url": "https://lh3.googleusercontent.com/...",
    "is_email_verified": true,
    "preferred_currency": "USD",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

**Errors:**
- `400` — missing or malformed id_token
- `401` — invalid Google ID token (bad signature, expired, wrong audience)

---

### POST /auth/otp/send

Send a 6-digit OTP to the user's email.

**Request:**
```json
{
  "email": "user@example.com"
}
```

**Response `200`:**
```json
{
  "message": "OTP sent successfully"
}
```

**Notes:**
- OTP expires after `OTP_TTL_MINUTES` (default 10 min)
- If the email has no account, one is created on verify
- Sending again replaces the previous OTP in Redis

---

### POST /auth/otp/verify

Verify OTP and receive JWT tokens.

**Request:**
```json
{
  "email": "user@example.com",
  "code": "483920"
}
```

**Response `200`:** Same shape as `/auth/google` response.

**Errors:**
- `400` — missing fields
- `401` — incorrect or expired OTP

---

### POST /auth/refresh

Exchange a refresh token for a new access token.

**Request:**
```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response `200`:**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Errors:**
- `401` — expired or tampered refresh token

---

## Protected Endpoints

All routes below require a valid Bearer access token.

---

## Users

### GET /api/v1/users/me

Get the authenticated user's profile.

**Response `200`:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "display_name": "Jane Doe",
  "avatar_url": "https://...",
  "is_email_verified": true,
  "preferred_currency": "USD",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

---

### PUT /api/v1/users/me/currency

Update the user's preferred currency. This preference is used as the default currency when the user creates a new group.

**Request:**
```json
{
  "currency": "JPY"
}
```

**Response `200`:**
```json
{
  "preferred_currency": "JPY"
}
```

**Errors:**
- `400` — missing currency or unsupported code (not in the 15 supported list)

---

## Groups

### POST /api/v1/groups

Create a new group. The currency defaults to the creator's `preferred_currency` if not provided.

**Request:**
```json
{
  "name": "Tokyo Trip",
  "description": "March holiday",
  "currency": "JPY"
}
```

`currency` is optional — omit it to use the creator's `preferred_currency`.

**Response `201`:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440002",
  "name": "Tokyo Trip",
  "description": "March holiday",
  "created_by": "550e8400-e29b-41d4-a716-446655440000",
  "currency": "JPY",
  "created_at": "2024-01-15T10:00:00Z"
}
```

The creator is automatically added as an `admin` member.

**Errors:**
- `400` — missing name or unsupported currency code

---

### GET /api/v1/groups

List all groups the authenticated user belongs to.

**Response `200`:**
```json
{
  "groups": [
    {
      "id": "uuid",
      "name": "Tokyo Trip",
      "currency": "JPY",
      "created_by": "uuid",
      "created_at": "2024-01-15T10:00:00Z"
    }
  ]
}
```

Groups are returned newest-first.

---

## Chat

### GET /api/v1/ws/groups/:group_id

Upgrade to WebSocket for real-time group chat and balance updates.

**Query param alternative:** `?token=<access_token>` (when headers are unavailable)

**Protocol:** See [WebSocket Protocol](websocket.md)

**Errors:**
- `401` — invalid/missing token
- `403` — user is not a member of the group
- `400` — WebSocket upgrade failed

---

### GET /api/v1/groups/:group_id/messages

Fetch message history for a group.

**Query params:**
| Param | Type | Default | Description |
|---|---|---|---|
| `limit` | int | 50 | Max messages to return |
| `before` | RFC3339 | now | Return messages before this timestamp |

**Response `200`:**
```json
{
  "messages": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440001",
      "group_id": "550e8400-e29b-41d4-a716-446655440002",
      "user_id": "550e8400-e29b-41d4-a716-446655440000",
      "content": "Dinner was 8000 yen",
      "type": "text",
      "created_at": "2024-01-15T19:30:00Z",
      "sender_name": "Jane Doe",
      "sender_avatar": "https://..."
    }
  ]
}
```

Messages are ordered newest-first.

---

## Expenses

### POST /api/v1/groups/:group_id/expenses

Create an expense with splits. All amounts are in the **group's currency unit** (e.g., dollars for USD, yen for JPY — no sub-units for JPY).

**Request — equal split:**
```json
{
  "amount": 9000,
  "description": "Ramen dinner",
  "split_method": "equal"
}
```

**Request — exact split:**

The `splits` field is a map of `user_id → amount`. Amounts must sum exactly to the expense amount.
```json
{
  "amount": 90.00,
  "description": "Groceries",
  "split_method": "exact",
  "splits": {
    "uuid-jane": 50.00,
    "uuid-bob":  40.00
  }
}
```

**Request — percentage split:**

The `splits` field is a map of `user_id → percentage`. Percentages must sum exactly to 100.
```json
{
  "amount": 200.00,
  "description": "Utilities",
  "split_method": "percentage",
  "splits": {
    "uuid-jane": 60,
    "uuid-bob":  40
  }
}
```

**Response `201`:**
```json
{
  "id": "uuid",
  "group_id": "uuid",
  "payer_id": "uuid",
  "amount": 9000,
  "currency": "JPY",
  "description": "Ramen dinner",
  "split_method": "equal",
  "created_at": "2024-01-15T19:30:00Z"
}
```

After creation, a `balance_update` WebSocket event is broadcast to all connected group members.

**Errors:**
- `400` — invalid group_id
- `404` — group not found
- `422` — splits don't sum to amount (exact), percentages don't sum to 100, unknown split method

---

### GET /api/v1/groups/:group_id/balances

Get all member balances and simplified settlement transactions for a group.

**Response `200`:**
```json
{
  "balances": [
    {
      "user_id": "uuid-1",
      "display_name": "Jane",
      "net_balance": 4500,
      "currency": "JPY"
    },
    {
      "user_id": "uuid-2",
      "display_name": "Bob",
      "net_balance": -4500,
      "currency": "JPY"
    }
  ],
  "simplified_debts": [
    {
      "from_user_id": "uuid-2",
      "from_user_name": "Bob",
      "to_user_id": "uuid-1",
      "to_user_name": "Jane",
      "amount": 4500,
      "currency": "JPY"
    }
  ]
}
```

`net_balance > 0` means others owe you. `net_balance < 0` means you owe others.
Amounts are in the group's currency human unit (e.g., whole yen for JPY, dollars with decimals for USD).

---

### GET /api/v1/groups/:group_id/balances/me

Get the current user's net balance in a group.

**Response `200`:**
```json
{
  "net_balance": -45.00,
  "currency": "USD"
}
```

---

## Error Response Format

All errors follow this shape:

```json
{
  "message": "human-readable error description"
}
```

| Status | Meaning |
|---|---|
| 400 | Bad request / validation failure |
| 401 | Missing, expired, or invalid token |
| 403 | Authenticated but not authorized (e.g., not a group member) |
| 404 | Resource not found |
| 422 | Business logic validation failure (e.g., splits don't add up) |
| 500 | Internal server error |
