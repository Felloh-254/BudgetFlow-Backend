package repository

import (
	"context"
	"database/sql"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LedgerRepository struct {
	db *pgxpool.Pool
}

func NewLedgerRepository(db *pgxpool.Pool) *LedgerRepository {
	return &LedgerRepository{db: db}
}

// CreateLedgerEntry inserts a single ledger entry
func (r *LedgerRepository) CreateLedgerEntry(ctx context.Context, transactionID, accountID int, amount float64, entryType string) (*models.LedgerEntry, error) {
	var entry models.LedgerEntry
	err := r.db.QueryRow(ctx,
		`INSERT INTO ledger_entries (transaction_id, account_id, amount, entry_type)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, transaction_id, account_id, amount, entry_type, created_at`,
		transactionID, accountID, amount, entryType,
	).Scan(&entry.ID, &entry.TransactionID, &entry.AccountID, &entry.Amount, &entry.EntryType, &entry.CreatedAt)

	return &entry, err
}

// ListByTransaction returns all ledger entries for a transaction
func (r *LedgerRepository) ListByTransaction(ctx context.Context, transactionID int) ([]models.LedgerEntry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, transaction_id, account_id, amount, entry_type, created_at
		 FROM ledger_entries
		 WHERE transaction_id = $1
		 ORDER BY created_at ASC`,
		transactionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []models.LedgerEntry{}
	for rows.Next() {
		var e models.LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Amount, &e.EntryType, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ListByAccount returns all ledger entries for an account (for balance calculation)
func (r *LedgerRepository) ListByAccount(ctx context.Context, accountID int) ([]models.LedgerEntry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, transaction_id, account_id, amount, entry_type, created_at
		 FROM ledger_entries
		 WHERE account_id = $1
		 ORDER BY created_at ASC`,
		accountID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []models.LedgerEntry{}
	for rows.Next() {
		var e models.LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Amount, &e.EntryType, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetBalance computes the current balance for an account from ledger entries
func (r *LedgerRepository) GetBalance(ctx context.Context, accountID int) (float64, error) {
	var balance sql.NullFloat64
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM ledger_entries WHERE account_id = $1`,
		accountID,
	).Scan(&balance)

	if err != nil {
		return 0, err
	}
	if balance.Valid {
		return balance.Float64, nil
	}
	return 0, nil
}

// DeleteByTransaction removes all ledger entries for a transaction (used during transaction deletion)
func (r *LedgerRepository) DeleteByTransaction(ctx context.Context, transactionID int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM ledger_entries WHERE transaction_id = $1`, transactionID)
	return err
}

// GetByTransactionAndAccount retrieves a specific ledger entry
func (r *LedgerRepository) GetByTransactionAndAccount(ctx context.Context, transactionID, accountID int) (*models.LedgerEntry, error) {
	var entry models.LedgerEntry
	err := r.db.QueryRow(ctx,
		`SELECT id, transaction_id, account_id, amount, entry_type, created_at
		 FROM ledger_entries
		 WHERE transaction_id = $1 AND account_id = $2`,
		transactionID, accountID,
	).Scan(&entry.ID, &entry.TransactionID, &entry.AccountID, &entry.Amount, &entry.EntryType, &entry.CreatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &entry, nil
}
