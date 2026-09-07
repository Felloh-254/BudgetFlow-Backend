package repository

import (
	"budgetapp/internal/models"
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountsRepository struct {
	db *pgxpool.Pool
}

func NewAccountsRepository(db *pgxpool.Pool) *AccountsRepository {
	return &AccountsRepository{db: db}
}

// CreateAccount creates a new account and initializes its balance record
func (r *AccountsRepository) CreateAccount(ctx context.Context, userID int, name string, accountType string, accountNumber *string, initialBalance float64, currency string) (*models.Account, error) {
	// Use a transaction to ensure account and balance are created together
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var a models.Account
	err = tx.QueryRow(ctx,
		`INSERT INTO accounts (user_id, name, type, account_number, currency)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, user_id, name, type, account_number, created_at, updated_at, currency`,
		userID, name, accountType, accountNumber, currency,
	).Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.AccountNumber, &a.CreatedAt, &a.UpdatedAt, &a.Currency)
	if err != nil {
		return nil, err
	}

	// Create the account balance record
	err = tx.QueryRow(ctx,
		`INSERT INTO account_balances (account_id, balance, version)
		 VALUES ($1, $2, 1)
		 RETURNING balance`,
		a.ID, initialBalance,
	).Scan(&a.Balance)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &a, nil
}

// ListAccountsByUser retrieves all accounts with their current balance
func (r *AccountsRepository) ListAccountsByUser(ctx context.Context, userID int) ([]models.Account, error) {
	rows, err := r.db.Query(ctx,
		`SELECT a.id, a.user_id, a.name, a.type, a.account_number, ab.balance, a.created_at, a.updated_at, a.currency
		 FROM accounts a
		 LEFT JOIN account_balances ab ON ab.account_id = a.id
		 WHERE a.user_id = $1
		 ORDER BY a.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var a models.Account
		var balance sql.NullFloat64
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.AccountNumber, &balance, &a.CreatedAt, &a.UpdatedAt, &a.Currency); err != nil {
			return nil, err
		}
		if balance.Valid {
			a.Balance = balance.Float64
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// GetAccountByID retrieves a single account by ID
func (r *AccountsRepository) GetAccountByID(ctx context.Context, accountID, userID int) (*models.Account, error) {
	var a models.Account
	var balance sql.NullFloat64
	err := r.db.QueryRow(ctx,
		`SELECT a.id, a.user_id, a.name, a.type, a.account_number, ab.balance, a.created_at, a.updated_at, a.currency
		 FROM accounts a
		 LEFT JOIN account_balances ab ON ab.account_id = a.id
		 WHERE a.id = $1 AND a.user_id = $2`,
		accountID, userID,
	).Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.AccountNumber, &balance, &a.CreatedAt, &a.UpdatedAt, &a.Currency)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if balance.Valid {
		a.Balance = balance.Float64
	}
	return &a, nil
}

// ExistsForUser checks if an account belongs to a user
func (r *AccountsRepository) ExistsForUser(ctx context.Context, accountID, userID int) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1 AND user_id = $2)`,
		accountID, userID,
	).Scan(&exists)
	return exists, err
}

// UpdateAccount updates account metadata (name, type, account_number, currency)
// Note: Balance is managed via account_balances table, not directly updated here
func (r *AccountsRepository) UpdateAccount(ctx context.Context, accountID, userID int, name string, accountType string, accountNumber *string, currency string) (*models.Account, error) {
	var a models.Account
	var balance sql.NullFloat64
	err := r.db.QueryRow(ctx,
		`UPDATE accounts SET name = $1, type = $2, account_number = $3, currency = $4, updated_at = NOW()
		 WHERE id = $5 AND user_id = $6
		 RETURNING id, user_id, name, type, account_number, created_at, updated_at, currency`,
		name, accountType, accountNumber, currency, accountID, userID,
	).Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.AccountNumber, &a.CreatedAt, &a.UpdatedAt, &a.Currency)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Fetch the balance separately
	err = r.db.QueryRow(ctx,
		`SELECT balance FROM account_balances WHERE account_id = $1`,
		a.ID,
	).Scan(&balance)
	if err == nil && balance.Valid {
		a.Balance = balance.Float64
	}

	return &a, nil
}

// DeleteAccount removes an account and its balance record
func (r *AccountsRepository) DeleteAccount(ctx context.Context, userID int, accountID int) error {
	// Cascade delete will handle account_balances due to ON DELETE CASCADE
	_, err := r.db.Exec(ctx,
		`DELETE FROM accounts WHERE id = $1 AND user_id = $2`,
		accountID, userID,
	)
	return err
}
