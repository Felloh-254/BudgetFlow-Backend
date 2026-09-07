package repository

import (
	"context"
	"database/sql"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountBalanceRepository struct {
	db *pgxpool.Pool
}

func NewAccountBalanceRepository(db *pgxpool.Pool) *AccountBalanceRepository {
	return &AccountBalanceRepository{db: db}
}

// GetBalance retrieves the current balance for an account
func (r *AccountBalanceRepository) GetBalance(ctx context.Context, accountID int) (*models.AccountBalance, error) {
	var bal models.AccountBalance
	var lastTxnID sql.NullInt64

	err := r.db.QueryRow(ctx,
		`SELECT id, account_id, balance, last_updated_txn, version, updated_at
		 FROM account_balances
		 WHERE account_id = $1`,
		accountID,
	).Scan(&bal.ID, &bal.AccountID, &bal.Balance, &lastTxnID, &bal.Version, &bal.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if lastTxnID.Valid {
		bal.LastUpdatedTxnID = &[]int{int(lastTxnID.Int64)}[0]
	}
	return &bal, nil
}

// CreateBalance initializes a balance record for a new account
func (r *AccountBalanceRepository) CreateBalance(ctx context.Context, accountID int, initialBalance float64) (*models.AccountBalance, error) {
	var bal models.AccountBalance
	err := r.db.QueryRow(ctx,
		`INSERT INTO account_balances (account_id, balance, version)
		 VALUES ($1, $2, $3)
		 RETURNING id, account_id, balance, last_updated_txn, version, updated_at`,
		accountID, initialBalance, 1,
	).Scan(&bal.ID, &bal.AccountID, &bal.Balance, &bal.LastUpdatedTxnID, &bal.Version, &bal.UpdatedAt)

	return &bal, err
}

// UpdateBalance atomically updates the balance and version
// Returns true if successful, false if version mismatch (concurrent update)
func (r *AccountBalanceRepository) UpdateBalance(ctx context.Context, accountID int, newBalance float64, lastTxnID int, currentVersion int) (bool, error) {
	result, err := r.db.Exec(ctx,
		`UPDATE account_balances
		 SET balance = $2, last_updated_txn = $3, version = version + 1, updated_at = now()
		 WHERE account_id = $1 AND version = $4`,
		accountID, newBalance, lastTxnID, currentVersion,
	)

	if err != nil {
		return false, err
	}

	// If no rows were affected, version mismatch occurred
	return result.RowsAffected() > 0, nil
}

// RecalculateBalance recalculates balance from ledger entries (for reconciliation)
// This should be used sparingly, only for sync/reconciliation operations
func (r *AccountBalanceRepository) RecalculateBalance(ctx context.Context, accountID int, ledgerRepo *LedgerRepository) (*models.AccountBalance, error) {
	// Get the sum from ledger entries
	balance, err := ledgerRepo.GetBalance(ctx, accountID)
	if err != nil {
		return nil, err
	}

	// Get the last transaction
	var lastTxnID sql.NullInt64
	err = r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(transaction_id), NULL)
		 FROM ledger_entries
		 WHERE account_id = $1`,
		accountID,
	).Scan(&lastTxnID)

	if err != nil {
		return nil, err
	}

	// Update the balance
	var bal models.AccountBalance
	var txnID *int
	if lastTxnID.Valid {
		txnID = &[]int{int(lastTxnID.Int64)}[0]
	}

	err = r.db.QueryRow(ctx,
		`UPDATE account_balances
		 SET balance = $2, last_updated_txn = $3, version = version + 1, updated_at = now()
		 WHERE account_id = $1
		 RETURNING id, account_id, balance, last_updated_txn, version, updated_at`,
		accountID, balance, txnID,
	).Scan(&bal.ID, &bal.AccountID, &bal.Balance, &bal.LastUpdatedTxnID, &bal.Version, &bal.UpdatedAt)

	return &bal, err
}

// DeleteBalance removes balance record (when account is deleted)
func (r *AccountBalanceRepository) DeleteBalance(ctx context.Context, accountID int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM account_balances WHERE account_id = $1`, accountID)
	return err
}

// GetAccountsWithBalance retrieves balance for multiple accounts
func (r *AccountBalanceRepository) GetAccountsWithBalance(ctx context.Context, accountIDs []int) (map[int]*models.AccountBalance, error) {
	if len(accountIDs) == 0 {
		return make(map[int]*models.AccountBalance), nil
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, balance, last_updated_txn, version, updated_at
		 FROM account_balances
		 WHERE account_id = ANY($1)`,
		accountIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]*models.AccountBalance)
	for rows.Next() {
		var bal models.AccountBalance
		var lastTxnID sql.NullInt64
		if err := rows.Scan(&bal.ID, &bal.AccountID, &bal.Balance, &lastTxnID, &bal.Version, &bal.UpdatedAt); err != nil {
			return nil, err
		}
		if lastTxnID.Valid {
			bal.LastUpdatedTxnID = &[]int{int(lastTxnID.Int64)}[0]
		}
		result[bal.AccountID] = &bal
	}
	return result, rows.Err()
}
