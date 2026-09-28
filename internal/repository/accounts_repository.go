package repository

import (
	"context"
	"database/sql"
	"log"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountsRepository struct {
	db *pgxpool.Pool
}

func NewAccountsRepository(db *pgxpool.Pool) *AccountsRepository {
	log.Println("[repo.accounts] NewAccountsRepository: created")
	return &AccountsRepository{db: db}
}

// CreateAccount creates a new account and initializes its balance record
func (r *AccountsRepository) CreateAccount(ctx context.Context, userID int, name string, accountType string, accountNumber *string, initialBalance float64, currency string) (*models.Account, error) {
	log.Printf("[repo.accounts] CreateAccount: user_id=%d name=%q type=%q currency=%q initial_balance=%.2f",
		userID, name, accountType, currency, initialBalance)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		log.Printf("[repo.accounts] CreateAccount: begin tx failed user_id=%d error=%v", userID, err)
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
		log.Printf("[repo.accounts] CreateAccount: insert failed user_id=%d error=%v", userID, err)
		return nil, err
	}

	log.Printf("[repo.accounts] CreateAccount: account row inserted account_id=%d", a.ID)

	err = tx.QueryRow(ctx,
		`INSERT INTO account_balances (account_id, balance, version)
		 VALUES ($1, $2, 1)
		 RETURNING balance`,
		a.ID, initialBalance,
	).Scan(&a.Balance)
	if err != nil {
		log.Printf("[repo.accounts] CreateAccount: balance insert failed account_id=%d error=%v", a.ID, err)
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		log.Printf("[repo.accounts] CreateAccount: commit failed account_id=%d error=%v", a.ID, err)
		return nil, err
	}

	log.Printf("[repo.accounts] CreateAccount: OK account_id=%d user_id=%d balance=%.2f", a.ID, a.UserID, a.Balance)
	return &a, nil
}

// ListAccountsByUser retrieves all accounts with their current balance
func (r *AccountsRepository) ListAccountsByUser(ctx context.Context, userID int) ([]models.Account, error) {
	log.Printf("[repo.accounts] ListAccountsByUser: user_id=%d", userID)

	rows, err := r.db.Query(ctx,
		`SELECT a.id, a.user_id, a.name, a.type, a.account_number, ab.balance, a.created_at, a.updated_at, a.currency
		 FROM accounts a
		 LEFT JOIN account_balances ab ON ab.account_id = a.id
		 WHERE a.user_id = $1
		 ORDER BY a.created_at DESC`,
		userID,
	)
	if err != nil {
		log.Printf("[repo.accounts] ListAccountsByUser: query failed user_id=%d error=%v", userID, err)
		return nil, err
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var a models.Account
		var balance sql.NullFloat64
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.AccountNumber, &balance, &a.CreatedAt, &a.UpdatedAt, &a.Currency); err != nil {
			log.Printf("[repo.accounts] ListAccountsByUser: scan failed user_id=%d error=%v", userID, err)
			return nil, err
		}
		if balance.Valid {
			a.Balance = balance.Float64
		}
		accounts = append(accounts, a)
	}

	log.Printf("[repo.accounts] ListAccountsByUser: OK user_id=%d count=%d", userID, len(accounts))
	return accounts, rows.Err()
}

// GetAccountByID retrieves a single account by ID
func (r *AccountsRepository) GetAccountByID(ctx context.Context, accountID, userID int) (*models.Account, error) {
	log.Printf("[repo.accounts] GetAccountByID: account_id=%d user_id=%d", accountID, userID)

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
			log.Printf("[repo.accounts] GetAccountByID: not found account_id=%d user_id=%d", accountID, userID)
			return nil, nil
		}
		log.Printf("[repo.accounts] GetAccountByID: FAILED account_id=%d user_id=%d error=%v", accountID, userID, err)
		return nil, err
	}

	if balance.Valid {
		a.Balance = balance.Float64
	}

	log.Printf("[repo.accounts] GetAccountByID: OK account_id=%d name=%q balance=%.2f", a.ID, a.Name, a.Balance)
	return &a, nil
}

// ExistsForUser checks if an account belongs to a user
func (r *AccountsRepository) ExistsForUser(ctx context.Context, accountID, userID int) (bool, error) {
	log.Printf("[repo.accounts] ExistsForUser: account_id=%d user_id=%d", accountID, userID)

	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1 AND user_id = $2)`,
		accountID, userID,
	).Scan(&exists)

	if err != nil {
		log.Printf("[repo.accounts] ExistsForUser: FAILED account_id=%d user_id=%d error=%v", accountID, userID, err)
		return false, err
	}

	log.Printf("[repo.accounts] ExistsForUser: account_id=%d user_id=%d exists=%v", accountID, userID, exists)
	return exists, nil
}

// UpdateAccount updates account metadata (name, type, account_number, currency)
func (r *AccountsRepository) UpdateAccount(ctx context.Context, accountID, userID int, name string, accountType string, accountNumber *string, currency string) (*models.Account, error) {
	log.Printf("[repo.accounts] UpdateAccount: account_id=%d user_id=%d name=%q type=%q currency=%q",
		accountID, userID, name, accountType, currency)

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
			log.Printf("[repo.accounts] UpdateAccount: not found account_id=%d user_id=%d", accountID, userID)
			return nil, nil
		}
		log.Printf("[repo.accounts] UpdateAccount: FAILED account_id=%d user_id=%d error=%v", accountID, userID, err)
		return nil, err
	}

	err = r.db.QueryRow(ctx,
		`SELECT balance FROM account_balances WHERE account_id = $1`,
		a.ID,
	).Scan(&balance)
	if err == nil && balance.Valid {
		a.Balance = balance.Float64
	}

	log.Printf("[repo.accounts] UpdateAccount: OK account_id=%d name=%q balance=%.2f", a.ID, a.Name, a.Balance)
	return &a, nil
}

// DeleteAccount removes an account and its balance record
func (r *AccountsRepository) DeleteAccount(ctx context.Context, userID int, accountID int) error {
	log.Printf("[repo.accounts] DeleteAccount: user_id=%d account_id=%d", userID, accountID)

	result, err := r.db.Exec(ctx,
		`DELETE FROM accounts WHERE id = $1 AND user_id = $2`,
		accountID, userID,
	)
	if err != nil {
		log.Printf("[repo.accounts] DeleteAccount: FAILED user_id=%d account_id=%d error=%v", userID, accountID, err)
		return err
	}

	rowsAffected := result.RowsAffected()
	log.Printf("[repo.accounts] DeleteAccount: OK user_id=%d account_id=%d rows_affected=%d", userID, accountID, rowsAffected)
	return nil
}
