APP      := spendbuddy
BINARY   := bin/server
MAIN     := ./cmd/server
MIGRATE  := migrations/001_schema.sql
DB_URL   ?= postgres://spendbuddy:secret@localhost:5432/spendbuddy_db?sslmode=disable

.PHONY: all build run dev test lint migrate migrate-down docker-up docker-down tidy sqlc

## ── Build & Run ──────────────────────────────────────────────────────────────

all: build

build:
	@mkdir -p bin
	go build -ldflags="-s -w" -o $(BINARY) $(MAIN)

run: build
	./$(BINARY)

dev:
	@which air > /dev/null || go install github.com/air-verse/air@latest
	air -c .air.toml

## ── Testing ──────────────────────────────────────────────────────────────────

test:
	go test -race -cover ./...

test-verbose:
	go test -race -v -cover ./...

## ── Lint ─────────────────────────────────────────────────────────────────────

lint:
	@which golangci-lint > /dev/null || (echo "install golangci-lint first" && exit 1)
	golangci-lint run ./...

## ── Database ─────────────────────────────────────────────────────────────────

migrate:
	psql "$(DB_URL)" -f $(MIGRATE)

migrate-down:
	psql "$(DB_URL)" -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"

## ── Docker ───────────────────────────────────────────────────────────────────

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app

## ── Go tooling ───────────────────────────────────────────────────────────────

tidy:
	go mod tidy

sqlc:
	@which sqlc > /dev/null || go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	sqlc generate

## ── Help ─────────────────────────────────────────────────────────────────────

help:
	@grep -E '^[a-zA-Z_-]+:' Makefile | sort | awk -F: '{printf "  %-20s\n", $$1}'
