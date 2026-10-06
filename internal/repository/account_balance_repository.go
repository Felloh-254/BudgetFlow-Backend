package repository

import (
	"context"
	"database/sql"
	"log"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AccountBalanceRepository struct {
	db *pgxpool.Pool
}

func NewAccountBalanceRepository(db *pgxpool.Pool) *AccountBalanceRepository {
	log.Println("[repo.account_balance] NewAccountBalanceRepository: created")
	return &AccountBalanceRepository{db: db}
}

// GetBalance retrieves the current balance for an account
func (r *AccountBalanceRepository) GetBalance(ctx context.Context, accountID int) (*models.AccountBalance, error) {
	log.Printf("[repo.account_balance] GetBalance: account_id=%d", accountID)

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
			log.Printf("[repo.account_balance] GetBalance: not found account_id=%d", accountID)
			return nil, nil
		}
		log.Printf("[repo.account_balance] GetBalance: FAILED account_id=%d error=%v", accountID, err)
		return nil, err
	}

	if lastTxnID.Valid {
		bal.LastUpdatedTxnID = &[]int{int(lastTxnID.Int64)}[0]
	}
	log.Printf("[repo.account_balance] GetBalance: OK account_id=%d balance=%.2f version=%d", accountID, bal.Balance, bal.Version)
	return &bal, nil
}

// CreateBalance initializes a balance record for a new account
func (r *AccountBalanceRepository) CreateBalance(ctx context.Context, accountID int, initialBalance float64) (*models.AccountBalance, error) {
	log.Printf("[repo.account_balance] CreateBalance: account_id=%d initial_balance=%.2f", accountID, initialBalance)

	var bal models.AccountBalance
	err := r.db.QueryRow(ctx,
		`INSERT INTO account_balances (account_id, balance, version)
		 VALUES ($1, $2, $3)
		 RETURNING id, account_id, balance, last_updated_txn, version, updated_at`,
		accountID, initialBalance, 1,
	).Scan(&bal.ID, &bal.AccountID, &bal.Balance, &bal.LastUpdatedTxnID, &bal.Version, &bal.UpdatedAt)

	if err != nil {
		log.Printf("[repo.account_balance] CreateBalance: FAILED account_id=%d error=%v", accountID, err)
		return nil, err
	}

	log.Printf("[repo.account_balance] CreateBalance: OK account_id=%d balance=%.2f version=%d", accountID, bal.Balance, bal.Version)
	return &bal, nil
}

// UpdateBalance atomically updates the balance and version
// Returns true if successful, false if version mismatch (concurrent update)
func (r *AccountBalanceRepository) UpdateBalance(ctx context.Context, accountID int, newBalance float64, lastTxnID int, currentVersion int) (bool, error) {
	log.Printf("[repo.account_balance] UpdateBalance: account_id=%d new_balance=%.2f last_txn_id=%d current_version=%d",
		accountID, newBalance, lastTxnID, currentVersion)

	result, err := r.db.Exec(ctx,
		`UPDATE account_balances
		 SET balance = $2, last_updated_txn = $3, version = version + 1, updated_at = now()
		 WHERE account_id = $1 AND version = $4`,
		accountID, newBalance, lastTxnID, currentVersion,
	)

	if err != nil {
		log.Printf("[repo.account_balance] UpdateBalance: FAILED account_id=%d error=%v", accountID, err)
		return false, err
	}

	success := result.RowsAffected() > 0
	if !success {
		log.Printf("[repo.account_balance] UpdateBalance: CONFLICT account_id=%d expected_version=%d (version mismatch)", accountID, currentVersion)
	} else {
		log.Printf("[repo.account_balance] UpdateBalance: OK account_id=%d new_balance=%.2f", accountID, newBalance)
	}

	return success, nil
}

// GetBalanceForUpdateTx loads and locks an account balance using the caller's transaction.
func (r *AccountBalanceRepository) GetBalanceForUpdateTx(ctx context.Context, tx pgx.Tx, accountID int) (float64, int, error) {
	var balance float64
	var version int
	err := tx.QueryRow(ctx,
		`SELECT balance, version FROM account_balances WHERE account_id = $1 FOR UPDATE`,
		accountID,
	).Scan(&balance, &version)
	return balance, version, err
}

// UpdateBalanceTx writes a balance and transaction reference atomically.
func (r *AccountBalanceRepository) UpdateBalanceTx(ctx context.Context, tx pgx.Tx, accountID int, newBalance float64, transactionID, version int) (bool, error) {
	result, err := tx.Exec(ctx,
		`UPDATE account_balances
		 SET balance = $1, last_updated_txn = $2, version = version + 1, updated_at = now()
		 WHERE account_id = $3 AND version = $4`,
		newBalance, transactionID, accountID, version,
	)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

// ReverseBalanceTx restores a balance and clears its transaction reference when appropriate.
func (r *AccountBalanceRepository) ReverseBalanceTx(ctx context.Context, tx pgx.Tx, accountID int, newBalance float64, transactionID, version int) (bool, error) {
	result, err := tx.Exec(ctx,
		`UPDATE account_balances
		 SET balance = $1,
		     last_updated_txn = CASE WHEN last_updated_txn = $4 THEN NULL ELSE last_updated_txn END,
		     version = version + 1,
		     updated_at = now()
		 WHERE account_id = $2 AND version = $3`,
		newBalance, accountID, version, transactionID,
	)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

// RecalculateBalance recalculates balance from ledger entries (for reconciliation)
func (r *AccountBalanceRepository) RecalculateBalance(ctx context.Context, accountID int, ledgerRepo *LedgerRepository) (*models.AccountBalance, error) {
	log.Printf("[repo.account_balance] RecalculateBalance: account_id=%d", accountID)

	balance, err := ledgerRepo.GetBalance(ctx, accountID)
	if err != nil {
		log.Printf("[repo.account_balance] RecalculateBalance: ledger GetBalance failed account_id=%d error=%v", accountID, err)
		return nil, err
	}

	var lastTxnID sql.NullInt64
	err = r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(transaction_id), NULL)
		 FROM ledger_entries
		 WHERE account_id = $1`,
		accountID,
	).Scan(&lastTxnID)

	if err != nil {
		log.Printf("[repo.account_balance] RecalculateBalance: max txn query failed account_id=%d error=%v", accountID, err)
		return nil, err
	}

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

	if err != nil {
		log.Printf("[repo.account_balance] RecalculateBalance: FAILED account_id=%d error=%v", accountID, err)
		return nil, err
	}

	log.Printf("[repo.account_balance] RecalculateBalance: OK account_id=%d recalculated_balance=%.2f version=%d",
		accountID, bal.Balance, bal.Version)
	return &bal, nil
}

// DeleteBalance removes balance record (when account is deleted)
func (r *AccountBalanceRepository) DeleteBalance(ctx context.Context, accountID int) error {
	log.Printf("[repo.account_balance] DeleteBalance: account_id=%d", accountID)

	_, err := r.db.Exec(ctx, `DELETE FROM account_balances WHERE account_id = $1`, accountID)
	if err != nil {
		log.Printf("[repo.account_balance] DeleteBalance: FAILED account_id=%d error=%v", accountID, err)
		return err
	}

	log.Printf("[repo.account_balance] DeleteBalance: OK account_id=%d", accountID)
	return nil
}

// GetAccountsWithBalance retrieves balance for multiple accounts
func (r *AccountBalanceRepository) GetAccountsWithBalance(ctx context.Context, accountIDs []int) (map[int]*models.AccountBalance, error) {
	log.Printf("[repo.account_balance] GetAccountsWithBalance: count=%d account_ids=%v", len(accountIDs), accountIDs)

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
		log.Printf("[repo.account_balance] GetAccountsWithBalance: FAILED error=%v", err)
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]*models.AccountBalance)
	for rows.Next() {
		var bal models.AccountBalance
		var lastTxnID sql.NullInt64
		if err := rows.Scan(&bal.ID, &bal.AccountID, &bal.Balance, &lastTxnID, &bal.Version, &bal.UpdatedAt); err != nil {
			log.Printf("[repo.account_balance] GetAccountsWithBalance: scan failed error=%v", err)
			return nil, err
		}
		if lastTxnID.Valid {
			bal.LastUpdatedTxnID = &[]int{int(lastTxnID.Int64)}[0]
		}
		result[bal.AccountID] = &bal
	}

	log.Printf("[repo.account_balance] GetAccountsWithBalance: OK found=%d", len(result))
	return result, rows.Err()
}
