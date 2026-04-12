-- SpendBuddy AI — Initial Schema
-- Run with: psql $DATABASE_URL -f migrations/001_schema.sql

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─────────────────────────────────────────
-- Users
-- ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
    id                UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    google_sub        TEXT        UNIQUE,                    -- NULL for OTP-only users
    email             TEXT        NOT NULL UNIQUE,
    display_name      TEXT        NOT NULL DEFAULT '',
    avatar_url        TEXT,
    is_email_verified BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email      ON users (email);
CREATE INDEX IF NOT EXISTS idx_users_google_sub ON users (google_sub) WHERE google_sub IS NOT NULL;

-- ─────────────────────────────────────────
-- Groups
-- ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS groups (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT        NOT NULL,
    description TEXT,
    avatar_url  TEXT,
    created_by  UUID        NOT NULL REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─────────────────────────────────────────
-- Group Members  (roles: admin | member)
-- ─────────────────────────────────────────
CREATE TYPE group_role AS ENUM ('admin', 'member');

CREATE TABLE IF NOT EXISTS group_members (
    group_id  UUID       NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    user_id   UUID       NOT NULL REFERENCES users  (id) ON DELETE CASCADE,
    role      group_role NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (group_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_group_members_user ON group_members (user_id);

-- ─────────────────────────────────────────
-- Messages
-- ─────────────────────────────────────────
CREATE TYPE message_type AS ENUM ('text', 'image', 'system');

CREATE TABLE IF NOT EXISTS messages (
    id         UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id   UUID         NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    user_id    UUID         NOT NULL REFERENCES users  (id) ON DELETE SET NULL,
    content    TEXT         NOT NULL,
    type       message_type NOT NULL DEFAULT 'text',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Cursor-style pagination: fetch latest N messages per group efficiently
CREATE INDEX IF NOT EXISTS idx_messages_group_created ON messages (group_id, created_at DESC);

-- ─────────────────────────────────────────
-- Expenses
-- ─────────────────────────────────────────
CREATE TYPE split_method AS ENUM ('equal', 'exact', 'percentage');

CREATE TABLE IF NOT EXISTS expenses (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    group_id      UUID         NOT NULL REFERENCES groups (id) ON DELETE CASCADE,
    payer_id      UUID         NOT NULL REFERENCES users  (id) ON DELETE SET NULL,
    -- Store as integer cents to avoid floating-point rounding issues
    amount_cents  BIGINT       NOT NULL CHECK (amount_cents > 0),
    description   TEXT         NOT NULL DEFAULT '',
    split_method  split_method NOT NULL DEFAULT 'equal',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_expenses_group ON expenses (group_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_expenses_payer ON expenses (payer_id);

-- ─────────────────────────────────────────
-- Expense Splits
-- ─────────────────────────────────────────
CREATE TABLE IF NOT EXISTS expense_splits (
    id                UUID   PRIMARY KEY DEFAULT uuid_generate_v4(),
    expense_id        UUID   NOT NULL REFERENCES expenses (id) ON DELETE CASCADE,
    user_id           UUID   NOT NULL REFERENCES users    (id) ON DELETE CASCADE,
    amount_owed_cents BIGINT NOT NULL CHECK (amount_owed_cents >= 0),

    UNIQUE (expense_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_splits_expense ON expense_splits (expense_id);
CREATE INDEX IF NOT EXISTS idx_splits_user    ON expense_splits (user_id);

-- ─────────────────────────────────────────
-- Trigger: keep users.updated_at fresh
-- ─────────────────────────────────────────
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
