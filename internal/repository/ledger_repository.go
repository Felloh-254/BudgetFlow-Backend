package repository

import (
	"context"
	"database/sql"
	"log"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LedgerRepository struct {
	db *pgxpool.Pool
}

func NewLedgerRepository(db *pgxpool.Pool) *LedgerRepository {
	log.Println("[repo.ledger] NewLedgerRepository: created")
	return &LedgerRepository{db: db}
}

// CreateLedgerEntry inserts a single ledger entry
func (r *LedgerRepository) CreateLedgerEntry(ctx context.Context, transactionID, accountID int, amount float64, entryType string) (*models.LedgerEntry, error) {
	log.Printf("[repo.ledger] CreateLedgerEntry: transaction_id=%d account_id=%d amount=%.2f entry_type=%s",
		transactionID, accountID, amount, entryType)

	var entry models.LedgerEntry
	err := r.db.QueryRow(ctx,
		`INSERT INTO ledger_entries (transaction_id, account_id, amount, entry_type)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, transaction_id, account_id, amount, entry_type, created_at`,
		transactionID, accountID, amount, entryType,
	).Scan(&entry.ID, &entry.TransactionID, &entry.AccountID, &entry.Amount, &entry.EntryType, &entry.CreatedAt)

	if err != nil {
		log.Printf("[repo.ledger] CreateLedgerEntry: FAILED transaction_id=%d error=%v", transactionID, err)
		return nil, err
	}

	log.Printf("[repo.ledger] CreateLedgerEntry: OK entry_id=%d transaction_id=%d account_id=%d amount=%.2f",
		entry.ID, entry.TransactionID, entry.AccountID, entry.Amount)
	return &entry, nil
}

// ListByTransaction returns all ledger entries for a transaction
func (r *LedgerRepository) ListByTransaction(ctx context.Context, transactionID int) ([]models.LedgerEntry, error) {
	log.Printf("[repo.ledger] ListByTransaction: transaction_id=%d", transactionID)

	rows, err := r.db.Query(ctx,
		`SELECT id, transaction_id, account_id, amount, entry_type, created_at
		 FROM ledger_entries
		 WHERE transaction_id = $1
		 ORDER BY created_at ASC`,
		transactionID,
	)
	if err != nil {
		log.Printf("[repo.ledger] ListByTransaction: query failed transaction_id=%d error=%v", transactionID, err)
		return nil, err
	}
	defer rows.Close()

	entries := []models.LedgerEntry{}
	for rows.Next() {
		var e models.LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Amount, &e.EntryType, &e.CreatedAt); err != nil {
			log.Printf("[repo.ledger] ListByTransaction: scan failed transaction_id=%d error=%v", transactionID, err)
			return nil, err
		}
		entries = append(entries, e)
	}

	log.Printf("[repo.ledger] ListByTransaction: OK transaction_id=%d count=%d", transactionID, len(entries))
	return entries, rows.Err()
}

// ListByAccount returns all ledger entries for an account (for balance calculation)
func (r *LedgerRepository) ListByAccount(ctx context.Context, accountID int) ([]models.LedgerEntry, error) {
	log.Printf("[repo.ledger] ListByAccount: account_id=%d", accountID)

	rows, err := r.db.Query(ctx,
		`SELECT id, transaction_id, account_id, amount, entry_type, created_at
		 FROM ledger_entries
		 WHERE account_id = $1
		 ORDER BY created_at ASC`,
		accountID,
	)
	if err != nil {
		log.Printf("[repo.ledger] ListByAccount: query failed account_id=%d error=%v", accountID, err)
		return nil, err
	}
	defer rows.Close()

	entries := []models.LedgerEntry{}
	for rows.Next() {
		var e models.LedgerEntry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.Amount, &e.EntryType, &e.CreatedAt); err != nil {
			log.Printf("[repo.ledger] ListByAccount: scan failed account_id=%d error=%v", accountID, err)
			return nil, err
		}
		entries = append(entries, e)
	}

	log.Printf("[repo.ledger] ListByAccount: OK account_id=%d count=%d", accountID, len(entries))
	return entries, rows.Err()
}

// GetBalance computes the current balance for an account from ledger entries
func (r *LedgerRepository) GetBalance(ctx context.Context, accountID int) (float64, error) {
	log.Printf("[repo.ledger] GetBalance: account_id=%d", accountID)

	var balance sql.NullFloat64
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM ledger_entries WHERE account_id = $1`,
		accountID,
	).Scan(&balance)

	if err != nil {
		log.Printf("[repo.ledger] GetBalance: FAILED account_id=%d error=%v", accountID, err)
		return 0, err
	}
	if balance.Valid {
		log.Printf("[repo.ledger] GetBalance: OK account_id=%d balance=%.2f", accountID, balance.Float64)
		return balance.Float64, nil
	}

	log.Printf("[repo.ledger] GetBalance: OK account_id=%d balance=0 (no entries)", accountID)
	return 0, nil
}

// DeleteByTransaction removes all ledger entries for a transaction
func (r *LedgerRepository) DeleteByTransaction(ctx context.Context, transactionID int) error {
	log.Printf("[repo.ledger] DeleteByTransaction: transaction_id=%d", transactionID)

	_, err := r.db.Exec(ctx, `DELETE FROM ledger_entries WHERE transaction_id = $1`, transactionID)
	if err != nil {
		log.Printf("[repo.ledger] DeleteByTransaction: FAILED transaction_id=%d error=%v", transactionID, err)
		return err
	}

	log.Printf("[repo.ledger] DeleteByTransaction: OK transaction_id=%d", transactionID)
	return nil
}

// GetByTransactionAndAccount retrieves a specific ledger entry
func (r *LedgerRepository) GetByTransactionAndAccount(ctx context.Context, transactionID, accountID int) (*models.LedgerEntry, error) {
	log.Printf("[repo.ledger] GetByTransactionAndAccount: transaction_id=%d account_id=%d", transactionID, accountID)

	var entry models.LedgerEntry
	err := r.db.QueryRow(ctx,
		`SELECT id, transaction_id, account_id, amount, entry_type, created_at
		 FROM ledger_entries
		 WHERE transaction_id = $1 AND account_id = $2`,
		transactionID, accountID,
	).Scan(&entry.ID, &entry.TransactionID, &entry.AccountID, &entry.Amount, &entry.EntryType, &entry.CreatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("[repo.ledger] GetByTransactionAndAccount: not found transaction_id=%d account_id=%d", transactionID, accountID)
			return nil, nil
		}
		log.Printf("[repo.ledger] GetByTransactionAndAccount: FAILED transaction_id=%d account_id=%d error=%v", transactionID, accountID, err)
		return nil, err
	}

	log.Printf("[repo.ledger] GetByTransactionAndAccount: OK entry_id=%d amount=%.2f", entry.ID, entry.Amount)
	return &entry, nil
}
