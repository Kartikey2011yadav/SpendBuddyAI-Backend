# SpendBuddy AI — Documentation Index

## What We Are Building

A real-time expense-sharing backend in Go. Users form groups, add expenses with flexible split methods, and see live balance updates. Chat happens in real-time via WebSocket. Authentication supports Google OAuth2 and passwordless email OTP.

---

## Docs

| Doc | Contents |
|---|---|
| [Architecture](architecture.md) | Layer structure, directory map, DI wiring, concurrency model, key design decisions |
| [API Reference](api.md) | All endpoints, request/response shapes, error codes |
| [Data Models](data-models.md) | Go domain structs, PostgreSQL schema, ER diagram |
| [Auth Flows](auth-flows.md) | Google OAuth2, Email OTP, JWT refresh, middleware |
| [WebSocket Protocol](websocket.md) | Connection, message formats, balance update events, keepalive |
| [Expense Splitting](expense-splitting.md) | Equal/exact/percentage splits, balance calculation, debt simplification algorithm |
| [Environment & Config](environment.md) | All env vars, local setup, Docker, Makefile commands |

---

## Quick Summary

**Stack:** Go 1.23 · Echo v4 · PostgreSQL 16 · Redis 7 · gorilla/websocket · JWT HS256

**Auth:** Google ID token → upsert user → JWT pair. Email OTP → Redis TTL → JWT pair.

**Expenses:** Three split methods. Amounts stored as integer cents. Splits inserted atomically in a DB transaction.

**Balances:** Single CTE query aggregates net balance per member. Greedy min-cash-flow algorithm minimizes settlement transactions.

**Real-time:** Single hub goroutine manages per-group rooms. Two goroutines per WS client (read + write). Balance snapshots broadcast to all group members after each new expense.
