# Budget App API

## Architecture

Layered, dependency-injected, no global state:

```
cmd/api/main.go        composition root — wires everything together, owns routing
cmd/migrate-data/       one-time SQLite -> Postgres data migration tool

internal/
  config/               loads and validates environment variables
  database/             Postgres connection pool (pgxpool)
  auth/                 JWT issuing/parsing, bcrypt password hashing
  apperr/                sentinel errors shared by service + handler layers
  models/               plain structs — the shape of data, no behavior
  repository/           all SQL lives here; returns models or errors
  service/               business logic + validation; calls repositories
  middleware/            echo middleware (JWT auth)
  handler/               HTTP layer only: bind request -> call service -> JSON response

migrations/             versioned SQL migrations (up/down pairs)
```

Request flow: `handler` binds the JSON body, calls a `service` method,
which validates input and calls one or more `repository` methods, which
run the actual SQL. Errors flow back up as `apperr` sentinels and get
mapped to HTTP status codes in one place (`handler/handler.go`).

This replaces the old flat structure (one `handlers.go` with everything —
SQL, validation, and HTTP mixed together) so that:
- SQL changes don't risk touching HTTP code and vice versa
- business rules (e.g. "amount must be > 0") are testable without a live DB
- the free-text `category` field is now a normalized `categories` table

## Setup

1. **Provision Postgres.** Any of these work well for a Go app:
   - [Neon](https://neon.tech) or [Supabase](https://supabase.com) — free tier, serverless
   - [Railway](https://railway.app) or [Render](https://render.com) — simple managed Postgres

2. **Copy the env file and fill it in:**
   ```bash
   cp .env.example .env
   ```

3. **Resolve dependencies** (this repo's `go.mod` was written by hand in a
   sandbox without network access to the Go module proxy — run this once
   to fetch exact versions and generate `go.sum`):
   ```bash
   go mod tidy
   ```

4. **Run the schema migration.** Install the [golang-migrate CLI](https://github.com/golang-migrate/migrate#installation), then:
   ```bash
   export DATABASE_URL="postgres://user:pass@host:5432/budgetapp?sslmode=require"
   make migrate-up
   ```

5. **(Optional) Migrate existing data** from your old `budget.db`:
   ```bash
   make migrate-data
   ```
   Note: old passwords were salted SHA-256, not bcrypt. Migrated users'
   existing password hashes won't validate against the new `CheckPassword`
   — either force a password reset for migrated accounts, or add a
   temporary legacy-hash fallback in `auth_service.go` if you need
   seamless login continuity.

6. **Run it:**
   ```bash
   make run
   ```

## Key changes from the previous version

- **Postgres instead of SQLite file** — works with any hosting provider,
  supports concurrent writers.
- **bcrypt instead of hand-rolled SHA-256** — industry standard, no custom
  crypto code to maintain or get subtly wrong.
- **Normalized `categories` table** — `budgets.category` and
  `transactions.category` were free-text strings; both now reference
  `categories.id`, so category names are consistent and can carry a
  color/icon.
- **Config via environment variables**, validated at startup — no more
  hardcoded JWT secret in source.
- **Layered architecture** — SQL, business logic, and HTTP handling are in
  separate packages instead of one `handlers.go`.
- **Graceful shutdown** — the server drains in-flight requests on
  SIGTERM/SIGINT instead of dying mid-request.
