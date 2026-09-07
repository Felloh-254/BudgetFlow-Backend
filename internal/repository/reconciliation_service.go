package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ReconciliationResult contains the results of a reconciliation operation
type ReconciliationResult struct {
	AccountID         int
	LedgerBalance     float64
	StoredBalance     float64
	IsConsistent      bool
	Discrepancy       float64
	TransactionCount  int
	LastTransactionID *int
	Fixed             bool
	ErrorMessage      string
}

// ReconciliationService provides utilities for verifying and fixing account balance consistency
type ReconciliationService struct {
	db *pgxpool.Pool
}

func NewReconciliationService(db *pgxpool.Pool) *ReconciliationService {
	return &ReconciliationService{db: db}
}

// ReconcileAccount verifies and optionally fixes a single account's balance
// Returns true if the balance was already consistent or successfully fixed
func (s *ReconciliationService) ReconcileAccount(ctx context.Context, accountID int, fix bool) (*ReconciliationResult, error) {
	result := &ReconciliationResult{
		AccountID: accountID,
	}

	// Get ledger balance (sum of all ledger entries)
	var ledgerBal sql.NullFloat64
	var txnCount int
	var lastTxnID sql.NullInt64

	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) as balance, COUNT(*) as txn_count, MAX(transaction_id) as last_txn
		 FROM ledger_entries
		 WHERE account_id = $1`,
		accountID,
	).Scan(&ledgerBal, &txnCount, &lastTxnID)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("failed to calculate ledger balance: %v", err)
		return result, nil
	}

	if ledgerBal.Valid {
		result.LedgerBalance = ledgerBal.Float64
	}
	result.TransactionCount = txnCount

	if lastTxnID.Valid {
		result.LastTransactionID = &[]int{int(lastTxnID.Int64)}[0]
	}

	// Get stored balance from account_balances
	var storedBal sql.NullFloat64
	var version int

	err = s.db.QueryRow(ctx,
		`SELECT balance, version FROM account_balances WHERE account_id = $1`,
		accountID,
	).Scan(&storedBal, &version)

	if err != nil {
		if err == sql.ErrNoRows {
			result.ErrorMessage = "account_balances record not found"
			return result, nil
		}
		result.ErrorMessage = fmt.Sprintf("failed to fetch stored balance: %v", err)
		return result, nil
	}

	if storedBal.Valid {
		result.StoredBalance = storedBal.Float64
	}

	// Check consistency
	discrepancy := result.LedgerBalance - result.StoredBalance
	result.Discrepancy = discrepancy
	result.IsConsistent = discrepancy == 0

	if result.IsConsistent {
		return result, nil
	}

	// If requested, fix the balance
	if fix {
		fixErr := s.fixAccountBalance(ctx, accountID, result.LedgerBalance, result.LastTransactionID, version)
		if fixErr != nil {
			result.ErrorMessage = fmt.Sprintf("failed to fix balance: %v", fixErr)
			result.Fixed = false
			return result, nil
		}
		result.Fixed = true
	}

	return result, nil
}

// ReconcileAllAccounts reconciles all accounts for a user
func (s *ReconciliationService) ReconcileAllAccounts(ctx context.Context, userID int, fix bool) ([]ReconciliationResult, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id FROM accounts WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ReconciliationResult
	for rows.Next() {
		var accountID int
		if err := rows.Scan(&accountID); err != nil {
			return nil, err
		}

		result, err := s.ReconcileAccount(ctx, accountID, fix)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
	}

	return results, rows.Err()
}

// fixAccountBalance updates the stored balance to match the ledger balance
func (s *ReconciliationService) fixAccountBalance(ctx context.Context, accountID int, correctBalance float64, lastTxnID *int, currentVersion int) error {
	result, err := s.db.Exec(ctx,
		`UPDATE account_balances
		 SET balance = $1, last_updated_txn = $2, version = version + 1, updated_at = now()
		 WHERE account_id = $3 AND version = $4`,
		correctBalance, lastTxnID, accountID, currentVersion,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("concurrent update detected - version mismatch, please retry")
	}

	return nil
}

// GetAccountHistory returns the ledger history for an account with transaction details
type AccountHistoryEntry struct {
	TransactionID   int
	AccountID       int
	Amount          float64
	EntryType       string
	TransactionType string
	Title           string
	Date            string
	CreatedAt       string
}

func (s *ReconciliationService) GetAccountHistory(ctx context.Context, accountID int, limit, offset int) ([]AccountHistoryEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := s.db.Query(ctx,
		`SELECT le.transaction_id, le.account_id, le.amount, le.entry_type, 
		        t.type as transaction_type, t.title, to_char(t.date, 'YYYY-MM-DD'), to_char(le.created_at, 'YYYY-MM-DD HH24:MI:SS')
		 FROM ledger_entries le
		 INNER JOIN transactions_v2 t ON t.id = le.transaction_id
		 WHERE le.account_id = $1
		 ORDER BY le.created_at DESC, le.id DESC
		 LIMIT $2 OFFSET $3`,
		accountID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AccountHistoryEntry
	for rows.Next() {
		var entry AccountHistoryEntry
		if err := rows.Scan(&entry.TransactionID, &entry.AccountID, &entry.Amount, &entry.EntryType,
			&entry.TransactionType, &entry.Title, &entry.Date, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}
