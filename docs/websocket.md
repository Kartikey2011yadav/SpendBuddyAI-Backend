# WebSocket Protocol

## Connection

```
GET /ws/groups/:group_id
Authorization: Bearer <access_token>
```

Or via query param (for environments where headers aren't available):
```
GET /ws/groups/:group_id?token=<access_token>
```

The server validates the JWT, then checks that the user is a member of the group. Non-members receive `403` before the upgrade.

---

## Architecture

```
                    Hub (single goroutine)
                   ┌──────────────────────┐
                   │  rooms: map[groupID]  │
                   │    map[*Client]{}     │
                   │                      │
  register ──────▶ │  register channel    │
  unregister ────▶ │  unregister channel  │
  broadcast ─────▶ │  broadcast channel   │
                   └──────────────────────┘
                          │         │
                   ┌──────┘         └──────┐
                   ▼                       ▼
              Client A                 Client B
          ┌──────────────┐         ┌──────────────┐
          │  ReadPump    │         │  ReadPump    │
          │  WritePump   │         │  WritePump   │
          │  send chan   │         │  send chan   │
          └──────────────┘         └──────────────┘
```

The hub runs a single `select` event loop. All operations (register, unregister, broadcast) are serialized through channels — no locks on the hot path for broadcasts.

Per-client read and write pumps are separate goroutines. The send channel has a buffer of 256 messages. If the buffer is full, the client is unregistered (connection dropped) rather than blocking the hub.

---

## Keepalive

The server sends a WebSocket `ping` frame every 54 seconds. Clients must respond with a `pong` frame (most WebSocket libraries do this automatically). If no pong is received within 60 seconds, the connection is closed.

---

## Message: Send Chat Message

Client → Server:

```json
{
  "type": "text",
  "content": "Dinner tonight?"
}
```

Supported `type` values: `"text"`, `"image"` (image content = URL)

The server:
1. Saves the message to PostgreSQL asynchronously
2. Broadcasts the message to all connected members of the group

Server → All clients in group:

```json
{
  "type": "new_message",
  "group_id": "550e8400-e29b-41d4-a716-446655440002",
  "payload": {
    "id": "uuid",
    "group_id": "uuid",
    "user_id": "uuid",
    "content": "Dinner tonight?",
    "type": "text",
    "created_at": "2024-01-15T19:30:00Z",
    "sender_name": "Jane Doe",
    "sender_avatar": "https://..."
  }
}
```

---

## Event: Balance Update

Triggered automatically after a new expense is created (not by the client).

Server → All clients in group:

```json
{
  "type": "balance_update",
  "group_id": "550e8400-e29b-41d4-a716-446655440002",
  "payload": [
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
  ]
}
```

This is a full snapshot of all member balances — the client should replace its local balance state with this payload, not merge it.

The broadcast is fire-and-forget: if a client is not connected, they will see the updated balance the next time they call `GET /api/v1/groups/:group_id/balances`.

---

## WSMessage Envelope

All messages from server to client share this envelope:

```go
type WSMessage struct {
    Type    string      `json:"type"`     // "new_message" | "balance_update"
    GroupID string      `json:"group_id"`
    Payload interface{} `json:"payload"`
}
```

---

## Connection Lifecycle

```
1. Client connects to /ws/groups/:group_id
2. Server validates JWT + membership
3. Hub registers client into group room
4. ReadPump + WritePump goroutines start
5. Client sends/receives messages
6. On disconnect (close frame / network error):
   → ReadPump exits
   → Hub unregisters client from room
   → WritePump drains and exits
```

---

## Error Cases

| Situation | Behavior |
|---|---|
| Invalid/expired JWT | HTTP 401 before WS upgrade |
| Not a group member | HTTP 403 before WS upgrade |
| Client send buffer full | Client is unregistered (dropped) |
| Pong timeout | Connection closed by server |
| DB error saving message | Message is lost (not retried); connection stays open |
