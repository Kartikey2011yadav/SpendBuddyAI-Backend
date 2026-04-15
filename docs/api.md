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
  "otp": "483920"
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
  "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Errors:**
- `401` — expired or tampered refresh token

---

## Protected Endpoints

All routes below require a valid Bearer access token.

---

### GET /ws/groups/:group_id

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
      "content": "Dinner was $80 total",
      "type": "text",
      "created_at": "2024-01-15T19:30:00Z",
      "sender_name": "Jane Doe",
      "sender_avatar": "https://..."
    }
  ]
}
```

Messages are ordered newest-first (pagination cursor pattern).

---

### POST /api/v1/groups/:group_id/expenses

Create an expense with splits.

**Request (equal split):**
```json
{
  "amount": 90.00,
  "description": "Dinner at Mario's",
  "split_method": "equal"
}
```

**Request (exact split):**
```json
{
  "amount": 90.00,
  "description": "Groceries",
  "split_method": "exact",
  "splits": [
    { "user_id": "uuid-1", "amount": 50.00 },
    { "user_id": "uuid-2", "amount": 40.00 }
  ]
}
```

**Request (percentage split):**
```json
{
  "amount": 100.00,
  "description": "Utilities",
  "split_method": "percentage",
  "splits": [
    { "user_id": "uuid-1", "percentage": 60 },
    { "user_id": "uuid-2", "percentage": 40 }
  ]
}
```

**Response `201`:**
```json
{
  "expense": {
    "id": "uuid",
    "group_id": "uuid",
    "payer_id": "uuid",
    "amount": 90.00,
    "description": "Dinner at Mario's",
    "split_method": "equal",
    "created_at": "2024-01-15T19:30:00Z"
  },
  "splits": [
    { "user_id": "uuid-1", "amount_owed": 45.00 },
    { "user_id": "uuid-2", "amount_owed": 45.00 }
  ]
}
```

After creation, a `balance_update` WebSocket event is broadcast to all connected group members.

**Errors:**
- `400` — invalid split method, splits don't sum to amount/100%
- `403` — payer is not a group member

---

### GET /api/v1/groups/:group_id/balances

Get all member balances and simplified debt settlements for a group.

**Response `200`:**
```json
{
  "balances": [
    {
      "user_id": "uuid-1",
      "display_name": "Jane",
      "net_balance": 45.00
    },
    {
      "user_id": "uuid-2",
      "display_name": "Bob",
      "net_balance": -45.00
    }
  ],
  "settlements": [
    {
      "from_user_id": "uuid-2",
      "from_user_name": "Bob",
      "to_user_id": "uuid-1",
      "to_user_name": "Jane",
      "amount": 45.00
    }
  ]
}
```

`net_balance > 0` means others owe you. `net_balance < 0` means you owe others.

---

### GET /api/v1/groups/:group_id/balances/me

Get the current user's net balance in a group.

**Response `200`:**
```json
{
  "user_id": "uuid",
  "display_name": "Jane",
  "net_balance": 45.00
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
| 500 | Internal server error |
