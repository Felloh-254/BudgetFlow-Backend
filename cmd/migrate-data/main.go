// Command migrate-data is a one-time tool to copy data out of the old
// budget.db (SQLite) into the new normalized Postgres schema. It's a
// separate main package (not part of the API binary) since it's only ever
// run once, by hand, during the cutover.
//
// Usage:
//
//	DATABASE_URL=postgres://user:pass@host:5432/budgetapp go run ./cmd/migrate-data ./budget.db
//
// Run the schema migration (migrations/0001_init.up.sql) against the
// target Postgres database FIRST — this tool only inserts rows, it doesn't
// create tables.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate-data <path-to-budget.db>")
	}
	sqlitePath := os.Args[1]

	pgURL := os.Getenv("DATABASE_URL")
	if pgURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	sqliteDB, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer sqliteDB.Close()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer pool.Close()

	userIDMap := migrateUsers(ctx, sqliteDB, pool)
	categoryCache := map[string]int{} // key: "userID:name:type"
	budgetIDMap := migrateBudgets(ctx, sqliteDB, pool, userIDMap, categoryCache)
	migrateTransactions(ctx, sqliteDB, pool, userIDMap, budgetIDMap, categoryCache)

	log.Println("migration complete")
}

func migrateUsers(ctx context.Context, src *sql.DB, dst *pgxpool.Pool) map[int]int {
	rows, err := src.Query(`SELECT id, email, password, name, created_at FROM users`)
	if err != nil {
		log.Fatalf("read users: %v", err)
	}
	defer rows.Close()

	idMap := map[int]int{}
	for rows.Next() {
		var oldID int
		var email, password, name, createdAt string
		if err := rows.Scan(&oldID, &email, &password, &name, &createdAt); err != nil {
			log.Fatalf("scan user: %v", err)
		}

		// NOTE: the old password field is a salted SHA-256 hash, not
		// bcrypt. It's carried over as-is so logins don't break, but the
		// new CheckPassword (bcrypt) won't recognize it. Prompt affected
		// users through a "reset password" flow after cutover, or add a
		// one-time legacy-hash fallback check if you need seamless login.
		var newID int
		err := dst.QueryRow(ctx,
			`INSERT INTO users (email, password_hash, name, created_at) VALUES ($1, $2, $3, $4) RETURNING id`,
			email, password, name, createdAt,
		).Scan(&newID)
		if err != nil {
			log.Fatalf("insert user %s: %v", email, err)
		}
		idMap[oldID] = newID
		fmt.Printf("migrated user %s (old id %d -> new id %d)\n", email, oldID, newID)
	}
	return idMap
}

func getOrCreateCategory(ctx context.Context, dst *pgxpool.Pool, cache map[string]int, userID int, name, catType string) int {
	key := fmt.Sprintf("%d:%s:%s", userID, name, catType)
	if id, ok := cache[key]; ok {
		return id
	}

	var id int
	err := dst.QueryRow(ctx,
		`SELECT id FROM categories WHERE user_id = $1 AND name = $2 AND type = $3`,
		userID, name, catType,
	).Scan(&id)
	if err == nil {
		cache[key] = id
		return id
	}

	err = dst.QueryRow(ctx,
		`INSERT INTO categories (user_id, name, type) VALUES ($1, $2, $3) RETURNING id`,
		userID, name, catType,
	).Scan(&id)
	if err != nil {
		log.Fatalf("insert category %s: %v", name, err)
	}
	cache[key] = id
	return id
}

func migrateBudgets(ctx context.Context, src *sql.DB, dst *pgxpool.Pool, userIDMap map[int]int, catCache map[string]int) map[int]int {
	rows, err := src.Query(`SELECT id, user_id, name, amount, category, color, created_at FROM budgets`)
	if err != nil {
		log.Fatalf("read budgets: %v", err)
	}
	defer rows.Close()

	idMap := map[int]int{}
	for rows.Next() {
		var oldID, oldUserID int
		var name, category, color, createdAt string
		var amount float64
		if err := rows.Scan(&oldID, &oldUserID, &name, &amount, &category, &color, &createdAt); err != nil {
			log.Fatalf("scan budget: %v", err)
		}

		newUserID := userIDMap[oldUserID]
		catID := getOrCreateCategory(ctx, dst, catCache, newUserID, category, "expense")

		var newID int
		err := dst.QueryRow(ctx,
			`INSERT INTO budgets (user_id, category_id, name, amount, color, created_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
			newUserID, catID, name, amount, color, createdAt,
		).Scan(&newID)
		if err != nil {
			log.Fatalf("insert budget %s: %v", name, err)
		}
		idMap[oldID] = newID
	}
	fmt.Printf("migrated %d budgets\n", len(idMap))
	return idMap
}

func migrateTransactions(ctx context.Context, src *sql.DB, dst *pgxpool.Pool, userIDMap, budgetIDMap map[int]int, catCache map[string]int) {
	rows, err := src.Query(`SELECT id, user_id, budget_id, title, amount, type, category, date, note, created_at FROM transactions`)
	if err != nil {
		log.Fatalf("read transactions: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var oldID, oldUserID int
		var oldBudgetID sql.NullInt64
		var title, txnType, category, date, note, createdAt string
		var amount float64
		if err := rows.Scan(&oldID, &oldUserID, &oldBudgetID, &title, &amount, &txnType, &category, &date, &note, &createdAt); err != nil {
			log.Fatalf("scan transaction: %v", err)
		}

		newUserID := userIDMap[oldUserID]
		catID := getOrCreateCategory(ctx, dst, catCache, newUserID, category, txnType)

		var newBudgetID *int
		if oldBudgetID.Valid {
			if bID, ok := budgetIDMap[int(oldBudgetID.Int64)]; ok {
				newBudgetID = &bID
			}
		}

		_, err := dst.Exec(ctx,
			`INSERT INTO transactions (user_id, budget_id, category_id, title, amount, type, date, note, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			newUserID, newBudgetID, catID, title, amount, txnType, date, note, createdAt,
		)
		if err != nil {
			log.Fatalf("insert transaction %s: %v", title, err)
		}
		count++
	}
	fmt.Printf("migrated %d transactions\n", count)
}
