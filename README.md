# BudgetFlow Backend

BudgetFlow is a Go REST API for personal finance management. It gives users a single place to manage budgets, accounts, categories, and transactions, then exposes a summary of their financial activity.

The API uses Echo for HTTP routing, PostgreSQL for persistence, and JWT bearer tokens for authentication.

## What It Does

- Register users and log in with email and password.
- Create and manage budgets and accounts.
- Record income, expenses, and transfers between accounts.
- Keep account balances consistent with transaction ledger entries.
- Read a financial summary for the authenticated user.
- Explore the API through the bundled Swagger UI and OpenAPI specification.

## Requirements

- Go 1.22 or newer
- PostgreSQL
- The `migrate` CLI for applying database migrations

## Quick Start

1. Clone the repository and enter the project directory.
2. Create a `.env` file in the project root:

```dotenv
PORT=8080
DATABASE_URL=postgres://user:password@localhost:5432/budgetflow?sslmode=disable
JWT_SECRET=replace-with-a-long-random-secret
JWT_EXPIRY_HOURS=24
CORS_ORIGINS=http://localhost:5173
```

`DATABASE_URL` and `JWT_SECRET` are required. The other values have defaults, but setting them explicitly makes local setup easier to understand. URL-encode special characters in database usernames and passwords.

3. Create the PostgreSQL database named in `DATABASE_URL`.
4. Apply the schema:

```bash
make migrate-up
```

5. Start the API:

```bash
make run
```

The server starts on `http://localhost:8080` by default.

## Verify the Server

```bash
curl http://localhost:8080/healthz
```

Expected response:

```json
{"status":"ok"}
```

Interactive documentation is available at [http://localhost:8080/docs](http://localhost:8080/docs). The source specification is in [openapi.yaml](openapi.yaml), and the documentation details are in [API_DOCUMENTATION.md](API_DOCUMENTATION.md).

## Authentication

Register or log in first:

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"password123","name":"Example User"}'
```

Both registration and login return a JWT in the `token` field. Send it with protected requests:

```bash
curl http://localhost:8080/api/me \
  -H 'Authorization: Bearer YOUR_TOKEN'
```

## API Overview

Public endpoints:

- `POST /api/auth/register` - create an account
- `POST /api/auth/login` - receive a JWT
- `POST /api/forgot-password` - request a password reset
- `PUT /api/password/reset` - reset a password

Protected endpoints require `Authorization: Bearer <token>`:

- `GET /api/me` - current user
- `GET|POST|PUT|DELETE /api/budgets` - budget management
- `GET|POST|PUT|DELETE /api/accounts` - account management
- `GET /api/transactions` - list transactions
- `POST /api/transactions/income` - record income
- `POST /api/transactions/expense` - record an expense
- `POST /api/transactions/transfer` - transfer money between accounts
- `DELETE /api/transactions/:id` - delete a transaction and reverse its ledger effects
- `GET /api/summary` - financial summary

Transaction creation is intentionally split by type. Amounts are supplied as positive values; the API determines the ledger direction from the transaction type.

## Project Structure

```text
cmd/api/              Application entry point and dependency wiring
cmd/migrate-data/     One-time legacy SQLite-to-PostgreSQL migration
internal/config/      Environment configuration
internal/routes/      Public and protected route registration
internal/handler/     HTTP request and response handling
internal/service/     Business rules
internal/repository/  PostgreSQL data access and ledger operations
internal/models/      API and database models
migrations/           Versioned PostgreSQL schema changes
docs/                 Swagger UI assets
openapi.yaml          OpenAPI 3 specification
```

The application follows a straightforward flow: routes call handlers, handlers call services, and services use repositories. Transaction writes update the transaction, ledger entries, and cached account balances together.

## Useful Commands

```bash
make run            # start the API
make build          # build bin/api
make migrate-up    # apply all pending migrations
make migrate-down  # roll back the latest migration
make migrate-data  # import legacy budget.db data once
make tidy           # normalize Go dependencies
```

Run `migrate-data` only after applying the migrations to the target PostgreSQL database. It expects the legacy SQLite file at `./budget.db`.

## Database Migrations

Each migration has an `.up.sql` file and a matching `.down.sql` file. Migrations are numbered and should be applied in order. The current schema includes users, categories, budgets, accounts, transactions, ledger entries, and account balance caching.

## Related Documentation

- [API documentation guide](API_DOCUMENTATION.md)
- [Architecture refactor notes](ARCHITECTURE_REFACTOR.md)
- [OpenAPI specification](openapi.yaml)
