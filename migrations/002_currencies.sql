-- Migration 002: Add multi-currency support
-- Run with: psql $DATABASE_URL -f migrations/002_currencies.sql

-- User's preferred currency — used as default when they create a group.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS preferred_currency CHAR(3) NOT NULL DEFAULT 'USD';

-- Group settlement currency — set at creation, immutable.
-- All expenses and balances in a group are denominated in this currency.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS currency CHAR(3) NOT NULL DEFAULT 'USD';
