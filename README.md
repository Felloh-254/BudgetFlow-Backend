# Budget App API

A modern, production-ready budgeting API built with Go, featuring a clean layered architecture and PostgreSQL for robust data persistence.

---

## Overview

The Budget App API provides a secure RESTful interface for managing personal finances. It enables users to create budgets, track transactions, and categorize expenses with ease. Built with maintainability and scalability in mind, this implementation replaces a legacy SQLite-based version with a modular, dependency-injected architecture.

---

## Architecture

The application follows a **clean layered architecture** with dependency injection, ensuring separation of concerns and testability.

cmd/
├── api/main.go # Composition root — wires dependencies, owns routing
└── migrate-data/ # One-time SQLite → PostgreSQL data migration tool

internal/
├── config/ # Loads and validates environment variables
├── database/ # PostgreSQL connection pool (pgxpool)
├── auth/ # JWT issuance/parsing, bcrypt password hashing
├── apperr/ # Sentinel errors shared across layers
├── models/ # Plain structs — data shape with no behavior
├── repository/ # All SQL queries — returns models or errors
├── service/ # Business logic + validation; calls repositories
├── middleware/ # Echo middleware (JWT authentication)
└── handler/ # HTTP layer: bind requests → call services → JSON responses

migrations/ # Versioned SQL migrations (up/down pairs)

### Request Flow

HTTP Request → Handler → Service → Repository → Database
↓ ↓ ↓ ↓
JSON Bind Validation SQL Return Data
↓ ↓ ↓ ↓
←←←←←←←←←← JSON Response ←←←←←←←←←←←

- **Handler**: Binds request body, calls service methods, maps errors to HTTP status codes
- **Service**: Contains business logic, validates input, orchestrates repository calls
- **Repository**: Executes SQL queries, returns models or domain errors
- **Errors**: Propagate upward as `apperr` sentinels and map to appropriate HTTP status codes

This architecture replaces the legacy flat structure (one `handlers.go` with mixed SQL, validation, and HTTP) to ensure:

- SQL changes don't affect HTTP code and vice versa
- Business rules (e.g., "amount must be > 0") are testable without a live database
- The free-text `category` field is normalized into a dedicated `categories` table

---

## Features

- **Secure Authentication**: JWT-based auth with bcrypt password hashing
- **User registration and login**: register, login, and fetch current user profile
- **Budget Management**: create, update, and delete budgets with category assignment
- **Custom Categories**: create and use custom categories for budgets and transactions
- **Transaction Tracking**: log income and expense transactions with category and date
- **Summary Reporting**: total income, total expenses, balance, budget stats, and recent monthly trends
- **PostgreSQL Backend**: robust, concurrent-writer support with proper migrations
- **Graceful Shutdown**: handles SIGTERM/SIGINT by draining in-flight requests
- **Environment Configuration**: all settings via environment variables, validated at startup
- **Modular Design**: clean separation between HTTP, business logic, and data layers

### Current limitations

- Password reset / forgot password flows are not implemented
- Notification scheduling (end-of-day or start-of-month reminders) is not implemented
- Family/invite user sharing is not implemented
- There is no explicit previous-month budget reuse feature in the backend
- Transaction listing does not currently support date-range or month filters

---

## Setup

### Prerequisites

- Go 1.21 or higher
- PostgreSQL database (local or cloud-hosted)
- [golang-migrate CLI](https://github.com/golang-migrate/migrate#installation) for schema migrations

### 1. Provision a PostgreSQL Database

Choose any of these excellent options:

- [Neon](https://neon.tech) — Free tier, serverless Postgres with branching
- [Supabase](https://supabase.com) — Full-featured Postgres with auth and storage
- [Railway](https://railway.app) — Simple managed Postgres with generous free tier
- [Render](https://render.com) — Easy-to-use managed Postgres
- Local installation — For development purposes

### 2. Configure Environment Variables

Copy the example environment file and fill in your values:

```bash
cp .env.example .env
```

Edit `.env` with your database credentials, JWT secret, and other settings:

```bash
DATABASE_URL=postgres://user:password@host:5432/budgetapp?sslmode=require
JWT_SECRET=your-strong-secret-key-here
PORT=8080
```

### 3. Resolve Dependencies

The `go.mod` file is included but may need to fetch dependencies:

```bash
go mod tidy
```

### 4. Run Schema Migrations

Install the `golang-migrate` CLI if you haven't already, then:

```bash
export DATABASE_URL="postgres://user:password@host:5432/budgetapp?sslmode=require"
make migrate-up
```

Or using the migrate CLI directly:

```bash
migrate -database "$DATABASE_URL" -path migrations up
```

### 5. (Optional) Migrate Existing Data

If you have an existing SQLite `budget.db` from the legacy version, migrate it:

```bash
make migrate-data
```


### 6. Run the Application

```bash
make run
```

The server will start on the port specified in your environment (default: `8080`).

## Key Improvements from the Legacy Version

| Aspect           | Legacy Version             | New Version                                  |
| ---------------- | -------------------------- | -------------------------------------------- |
| Database         | SQLite (file-based)        | PostgreSQL (production-ready, concurrent)    |
| Password Hashing | Custom SHA-256             | bcrypt (industry standard)                   |
| Categories       | Free-text strings          | Normalized categories table                  |
| Configuration    | Hardcoded in source        | Environment variables with validation        |
| Architecture     | Monolithic handlers.go     | Layered: handler → service → repository      |
| Error Handling   | Inconsistent               | Structured sentinel errors with HTTP mapping |
| Shutdown         | Abrupt termination         | Graceful shutdown with request draining      |
| Testing          | Difficult (mixed concerns) | Isolated layers for unit testing             |

## Current API Endpoints

### Authentication

| Method | Endpoint             | Description                            |
| ------ | -------------------- | -------------------------------------- |
| POST   | `/api/auth/register` | Register a new user and receive JWT    |
| POST   | `/api/auth/login`    | Authenticate and receive JWT           |
| GET    | `/api/me`            | Get current authenticated user profile |

### Budgets

| Method | Endpoint           | Description                   |
| ------ | ------------------ | ----------------------------- |
| GET    | `/api/budgets`     | List all budgets for the user |
| POST   | `/api/budgets`     | Create a new budget           |
| PUT    | `/api/budgets/:id` | Update an existing budget     |
| DELETE | `/api/budgets/:id` | Delete a budget               |

### Transactions

| Method | Endpoint                | Description            |
| ------ | ----------------------- | ---------------------- |
| GET    | `/api/transactions`     | List user transactions |
| POST   | `/api/transactions`     | Create a transaction   |
| PUT    | `/api/transactions/:id` | Update a transaction   |
| DELETE | `/api/transactions/:id` | Delete a transaction   |

### Summary

| Method | Endpoint       | Description                                  |
| ------ | -------------- | -------------------------------------------- |
| GET    | `/api/summary` | Get totals, budget stats, and monthly trends |

> Note: All endpoints except `/api/auth/register` and `/api/auth/login` require a valid `Authorization: Bearer <token>` header.

## Development

Making Changes

- Add a new endpoint: Handler → Service → Repository
- Database changes: create migration files in `migrations/`
- Environment variables: update `config/config.go` and `.env.example`

### Running Tests

```bash
go test ./...
```

Building for Production
bash
go build -o budget-api ./cmd/api
Deployment
The application is designed for easy deployment to any platform that supports Go:

Fly.io: Deploy Go apps with Fly

Render: Deploy Go on Render

Railway: Go on Railway

AWS/GCP/Azure: Deploy as a containerized application

License
This project is licensed under the MIT License - see the LICENSE file for details.

Contributing
Contributions are welcome! Please ensure:

Code follows the established layering pattern

Tests are included for new functionality

Environment variables are documented in .env.example

Database migrations are reversible

Acknowledgments
Built with Echo web framework

PostgreSQL driver: pgx

Migrations: golang-migrate

