package repository

import (
	"budgetapp/internal/models"
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountsRepository struct {
	db *pgxpool.Pool
}

func NewAccountsRepository(db *pgxpool.Pool) *AccountsRepository {
	return &AccountsRepository{db: db}
}

func (r *AccountsRepository) CreateAccount(ctx context.Context, userID int, name string, accountType string, accountNumber *string, banlance float64, currency string) (*models.Account, error) {
	var a models.Account
	err := r.db.QueryRow(ctx,
		`INSERT INTO accounts (user_id, name, type, account_number, balance, currency)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, name, type, account_number, balance, created_at, updated_at, currency`,
		userID, name, accountType, accountNumber, banlance, currency,
	).Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.AccountNumber, &a.Balance, &a.CreatedAt, &a.UpdatedAt, &a.Currency)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AccountsRepository) ListAccountsByUser(ctx context.Context, userID int) ([]models.Account, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, name, type, account_number, balance, created_at, updated_at, currency
		 FROM accounts WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var a models.Account
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.AccountNumber, &a.Balance, &a.CreatedAt, &a.UpdatedAt, &a.Currency); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (r *AccountsRepository) UpdateAccount(ctx context.Context, accountID int, name string, accountType string, accountNumber *string, balance float64, currency string) (*models.Account, error) {
	var a models.Account
	err := r.db.QueryRow(ctx,
		`UPDATE accounts SET name = $1, type = $2, account_number = $3, balance = $4, currency = $5, updated_at = NOW()
		 WHERE id = $6
		 RETURNING id, user_id, name, type, account_number, balance, created_at, updated_at, currency`,
		name, accountType, accountNumber, balance, currency, accountID,
	).Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.AccountNumber, &a.Balance, &a.CreatedAt, &a.UpdatedAt, &a.Currency)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AccountsRepository) DeleteAccount(ctx context.Context, userID int, accountID int) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM accounts WHERE id = $1 AND user_id = $2`,
		accountID, userID,
	)
	return err
}
