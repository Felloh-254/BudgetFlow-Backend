.PHONY: run build migrate-up migrate-down migrate-data tidy

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

tidy:
	go mod tidy

# Requires the golang-migrate CLI: https://github.com/golang-migrate/migrate
migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down 1

# One-time: copy data from the old SQLite budget.db into Postgres.
# Run migrate-up against the target DB first.
migrate-data:
	go run ./cmd/migrate-data ./budget.db
