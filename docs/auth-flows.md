# Authentication Flows

## Overview

Three authentication methods are supported. All produce the same JWT token pair on success.

| Method | File | Use Case |
|---|---|---|
| Google OAuth2 | `/internal/auth/google.go` | Social login |
| Email OTP | `/internal/auth/otp.go` | Passwordless email login |
| JWT Refresh | `/internal/auth/jwt.go` | Extend session |

---

## 1. Google OAuth2

```
Client (mobile/web)
    │
    │  1. User taps "Sign in with Google"
    │  2. Google returns ID token to client
    │
    ▼
POST /auth/google
    { "id_token": "eyJ..." }
    │
    │  3. Validate ID token against Google's public keys
    │     - Verifies signature, expiry, audience (GOOGLE_CLIENT_ID)
    │
    │  4. Extract: sub (google_sub), email, name, picture
    │
    │  5. Upsert user:
    │     a. Find by google_sub → update display_name, avatar
    │     b. Not found by google_sub but email exists → link google_sub
    │     c. Neither → create new user (is_email_verified = true)
    │
    │  6. Generate JWT token pair
    │
    ▼
{ tokens: { access_token, refresh_token }, user: {...} }
```

**Implementation:** `internal/auth/google.go::GoogleAuthService.Authenticate()`

---

## 2. Email OTP

### Step 1 — Send OTP

```
POST /auth/otp/send
    { "email": "user@example.com" }
    │
    │  1. Generate 6-digit random code
    │  2. Store in Redis: key="otp:{email}", value=code, TTL=OTP_TTL_MINUTES
    │  3. Send email via SMTP (goroutine, non-blocking)
    │
    ▼
{ "message": "OTP sent successfully" }
```

**Redis key pattern:** `otp:{email}` with TTL (default 10 min)

Sending again overwrites the previous OTP — no "already sent" error.

### Step 2 — Verify OTP

```
POST /auth/otp/verify
    { "email": "user@example.com", "otp": "483920" }
    │
    │  1. GET otp:{email} from Redis
    │  2. Constant-time compare provided code vs stored code
    │  3. DELETE otp:{email} from Redis (single-use)
    │
    │  4. Upsert user:
    │     a. Email exists → mark is_email_verified = true
    │     b. Not found → create new user (is_email_verified = true)
    │
    │  5. Generate JWT token pair
    │
    ▼
{ tokens: { access_token, refresh_token }, user: {...} }
```

**Implementation:** `internal/auth/otp.go::OTPService.SendOTP()` / `VerifyOTP()`

---

## 3. JWT Token Structure

**Algorithm:** HS256 (HMAC-SHA256)

**Access Token Claims:**
```json
{
  "uid": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "iss": "spendbuddy",
  "exp": 1704067200,
  "iat": 1704066300
}
```

**Token TTLs:**
| Token | Default TTL | Env Var |
|---|---|---|
| Access | 15 minutes | `JWT_ACCESS_TTL_MINUTES` |
| Refresh | 30 days | `JWT_REFRESH_TTL_DAYS` |

**Secrets:**
- Access and refresh use separate secrets (`JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET`)
- This means a leaked access token cannot be used to forge a refresh token

---

## 4. Token Refresh

```
POST /auth/refresh
    { "refresh_token": "eyJ..." }
    │
    │  1. Validate refresh token against JWT_REFRESH_SECRET
    │  2. Check expiry
    │  3. Extract uid + email from claims
    │  4. Issue new access token (no DB lookup needed)
    │
    ▼
{ "access_token": "eyJ..." }
```

No new refresh token is issued — clients use the same refresh token until it expires (30 days).

**Implementation:** `internal/auth/jwt.go::JWTService.RefreshAccessToken()`

---

## 5. JWT Middleware

All `/api/v1/*` and `/ws/*` routes are protected by `internal/delivery/http/middleware/auth.go`.

```
Authorization: Bearer <access_token>
    │
    │  1. Extract token from header
    │  2. Parse + validate against JWT_ACCESS_SECRET
    │  3. Check expiry
    │  4. Inject to Echo context:
    │       ctx.Set("user_id", uid)
    │       ctx.Set("user_email", email)
    │
    ▼
Handler receives user_id and user_email from context
```

For WebSocket connections, the token can alternatively be passed as `?token=<access_token>` query parameter.

**Returns `401`** if token is missing, expired, or tampered.

---

## 6. Account Linking Logic

When a user authenticates via Google and an account with the same email already exists (created via OTP), the `google_sub` is linked to the existing account. The user now has both login methods.

```
Email account exists?  →  google_sub already set?
        │                         │
       YES                       YES  → just update, return existing user
        │                        NO   → link google_sub to existing user
       NO
        │
     Create new user (google_sub + is_email_verified = true)
```
