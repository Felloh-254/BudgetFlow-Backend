package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"budgetapp/internal/apperr"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionService struct {
	transactions *repository.TransactionRepository
	categories   *repository.CategoryRepository
	accounts     *repository.AccountsRepository
	ledger       *repository.LedgerRepository
	balances     *repository.AccountBalanceRepository
	db           *pgxpool.Pool // For transaction management
}

func NewTransactionService(
	transactions *repository.TransactionRepository,
	categories *repository.CategoryRepository,
	accounts *repository.AccountsRepository,
	ledger *repository.LedgerRepository,
	balances *repository.AccountBalanceRepository,
	db *pgxpool.Pool,
) *TransactionService {
	return &TransactionService{
		transactions: transactions,
		categories:   categories,
		accounts:     accounts,
		ledger:       ledger,
		balances:     balances,
		db:           db,
	}
}

// List returns filtered and paginated transactions for a user
func (s *TransactionService) List(ctx context.Context, userID int, filter models.TransactionFilter) ([]models.Transaction, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.transactions.ListByUser(ctx, userID, filter)
}

// GetByID retrieves a single transaction with all details
func (s *TransactionService) GetByID(ctx context.Context, transactionID, userID int) (*models.TransactionDetail, error) {
	detail, err := s.enrichTransactionDetail(ctx, transactionID, userID)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, apperr.ErrNotFound
	}
	return detail, nil
}

// Update updates transaction metadata (title, date, note) and optionally category
func (s *TransactionService) Update(ctx context.Context, transactionID, userID int, in models.UpdateTransactionInput) (*models.TransactionDetail, error) {
	txn, err := s.transactions.GetByID(ctx, transactionID, userID)
	if err != nil {
		return nil, err
	}
	if txn == nil {
		return nil, apperr.ErrNotFound
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = txn.Title
	}
	date := strings.TrimSpace(in.Date)
	if date == "" {
		date = txn.Date
	}
	note := in.Note
	if note == "" && in.Title == "" && in.Date == "" && in.Category == "" {
		return s.enrichTransactionDetail(ctx, transactionID, userID)
	}

	_, err = s.transactions.Update(ctx, transactionID, userID, title, date, note)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(in.Category) != "" && txn.Type != "transfer" {
		catType := txn.Type
		cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), catType)
		if err != nil {
			return nil, err
		}
		if err := s.transactions.SetCategory(ctx, transactionID, cat.ID); err != nil {
			return nil, err
		}
	}

	return s.enrichTransactionDetail(ctx, transactionID, userID)
}

// CreateIncome creates an income transaction (money enters an account)
// - amount is positive
// - creates 1 ledger entry (debit to account)
// - associates category
func (s *TransactionService) CreateIncome(ctx context.Context, userID int, in models.TransactionInput) (*models.TransactionDetail, error) {
	if err := s.validateTransactionInput(in, "income"); err != nil {
		return nil, err
	}

	// Verify account belongs to user
	if exists, err := s.accounts.ExistsForUser(ctx, in.AccountID, userID); err != nil {
		return nil, err
	} else if !exists {
		return nil, apperr.ErrNotFound
	}

	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}

	// Find or create category
	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "income")
	if err != nil {
		return nil, err
	}

	// Create transaction and ledger entry atomically
	detail, err := s.createTransactionWithLedgerEntries(ctx, userID, models.Transaction{
		Type:  "income",
		Title: strings.TrimSpace(in.Title),
		Date:  in.Date,
		Note:  in.Note,
	}, []struct {
		AccountID int
		Amount    float64
		EntryType string
	}{
		{AccountID: in.AccountID, Amount: in.Amount, EntryType: "debit"},
	}, []int{cat.ID}, in.IdempotencyKey)

	return detail, err
}

// CreateExpense creates an expense transaction (money leaves an account)
// - amount is positive (stored as-is, but represents money leaving)
// - creates 1 ledger entry (credit from account)
// - associates category
func (s *TransactionService) CreateExpense(ctx context.Context, userID int, in models.TransactionInput) (*models.TransactionDetail, error) {
	if err := s.validateTransactionInput(in, "expense"); err != nil {
		return nil, err
	}

	// Verify account belongs to user
	if exists, err := s.accounts.ExistsForUser(ctx, in.AccountID, userID); err != nil {
		return nil, err
	} else if !exists {
		return nil, apperr.ErrNotFound
	}

	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}

	// Find or create category
	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "expense")
	if err != nil {
		return nil, err
	}

	// Create transaction and ledger entry atomically
	// Expense is stored as negative in ledger
	detail, err := s.createTransactionWithLedgerEntries(ctx, userID, models.Transaction{
		Type:  "expense",
		Title: strings.TrimSpace(in.Title),
		Date:  in.Date,
		Note:  in.Note,
	}, []struct {
		AccountID int
		Amount    float64
		EntryType string
	}{
		{AccountID: in.AccountID, Amount: -in.Amount, EntryType: "credit"},
	}, []int{cat.ID}, in.IdempotencyKey)

	return detail, err
}

// CreateTransfer creates a transfer transaction (money moves between accounts)
// - creates 2 ledger entries (credit from source, debit to destination)
// - no category
func (s *TransactionService) CreateTransfer(ctx context.Context, userID int, in models.TransferInput) (*models.TransactionDetail, error) {
	if err := s.validateTransferInput(in); err != nil {
		return nil, err
	}

	// Verify both accounts belong to user
	if exists, err := s.accounts.ExistsForUser(ctx, in.FromAccountID, userID); err != nil {
		return nil, err
	} else if !exists {
		return nil, apperr.ErrNotFound
	}

	if exists, err := s.accounts.ExistsForUser(ctx, in.ToAccountID, userID); err != nil {
		return nil, err
	} else if !exists {
		return nil, apperr.ErrNotFound
	}

	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}

	// Create transaction with 2 ledger entries atomically
	detail, err := s.createTransactionWithLedgerEntries(ctx, userID, models.Transaction{
		Type:  "transfer",
		Title: strings.TrimSpace(in.Title),
		Date:  in.Date,
		Note:  in.Note,
	}, []struct {
		AccountID int
		Amount    float64
		EntryType string
	}{
		{AccountID: in.FromAccountID, Amount: -in.Amount, EntryType: "credit"},
		{AccountID: in.ToAccountID, Amount: in.Amount, EntryType: "debit"},
	}, []int{}, in.IdempotencyKey)

	return detail, err
}

// createTransactionWithLedgerEntries atomically creates a transaction, ledger entries, and updates balances
// This is the core atomic operation that ensures data consistency
func (s *TransactionService) createTransactionWithLedgerEntries(
	ctx context.Context,
	userID int,
	txn models.Transaction,
	entries []struct {
		AccountID int
		Amount    float64
		EntryType string
	},
	categoryIDs []int,
	idempotencyKey *string,
) (*models.TransactionDetail, error) {
	// Acquire a connection for the transaction
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	// Start a database transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Check idempotency if key provided
	if idempotencyKey != nil {
		var existingID int
		err := tx.QueryRow(ctx,
			`SELECT id FROM transactions_v2 WHERE idempotency_key = $1 LIMIT 1`,
			*idempotencyKey,
		).Scan(&existingID)
		if err == nil {
			// Transaction already exists, return it
			return s.enrichTransactionDetail(ctx, existingID, userID)
		}
		if err != pgx.ErrNoRows {
			return nil, err
		}
	}

	// Create the transaction event
	var createdTxn models.Transaction
	var idempKey sql.NullString
	err = tx.QueryRow(ctx,
		`INSERT INTO transactions_v2 (user_id, type, title, date, note, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, type, title, to_char(date, 'YYYY-MM-DD'), note, idempotency_key, created_at, updated_at`,
		userID, txn.Type, txn.Title, txn.Date, txn.Note, idempotencyKey,
	).Scan(&createdTxn.ID, &createdTxn.UserID, &createdTxn.Type, &createdTxn.Title, &createdTxn.Date, &createdTxn.Note, &idempKey, &createdTxn.CreatedAt, &createdTxn.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if idempKey.Valid {
		createdTxn.IdempotencyKey = &idempKey.String
	}

	// Create ledger entries and update balances
	for _, e := range entries {
		// Insert ledger entry
		_, err := tx.Exec(ctx,
			`INSERT INTO ledger_entries (transaction_id, account_id, amount, entry_type)
			 VALUES ($1, $2, $3, $4)`,
			createdTxn.ID, e.AccountID, e.Amount, e.EntryType,
		)
		if err != nil {
			return nil, err
		}

		// Get current balance
		var currentBalance float64
		var version int
		err = tx.QueryRow(ctx,
			`SELECT balance, version FROM account_balances WHERE account_id = $1 FOR UPDATE`,
			e.AccountID,
		).Scan(&currentBalance, &version)
		if err != nil {
			return nil, err
		}

		// Update balance
		newBalance := currentBalance + e.Amount
		result, err := tx.Exec(ctx,
			`UPDATE account_balances
			 SET balance = $1, last_updated_txn = $2, version = version + 1, updated_at = now()
			 WHERE account_id = $3 AND version = $4`,
			newBalance, createdTxn.ID, e.AccountID, version,
		)
		if err != nil {
			return nil, err
		}
		if result.RowsAffected() == 0 {
			return nil, errors.New("concurrent balance update detected, please retry")
		}
	}

	// Associate categories
	for _, catID := range categoryIDs {
		_, err := tx.Exec(ctx,
			`INSERT INTO transaction_categories (transaction_id, category_id)
			 VALUES ($1, $2)
			 ON CONFLICT (transaction_id, category_id) DO NOTHING`,
			createdTxn.ID, catID,
		)
		if err != nil {
			return nil, err
		}
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Return enriched transaction detail
	return s.enrichTransactionDetail(ctx, createdTxn.ID, userID)
}

// enrichTransactionDetail loads a transaction with all related data
func (s *TransactionService) enrichTransactionDetail(ctx context.Context, txnID, userID int) (*models.TransactionDetail, error) {
	txn, err := s.transactions.GetByID(ctx, txnID, userID)
	if err != nil || txn == nil {
		return nil, err
	}

	// Get ledger entries
	entries, err := s.ledger.ListByTransaction(ctx, txnID)
	if err != nil {
		return nil, err
	}

	// Get categories
	categories, err := s.transactions.GetCategories(ctx, txnID)
	if err != nil {
		return nil, err
	}

	// Build account names map
	accountNames := make(map[int]string)
	for _, entry := range entries {
		acc, err := s.accounts.GetAccountByID(ctx, entry.AccountID, userID)
		if err == nil && acc != nil {
			accountNames[entry.AccountID] = acc.Name
		}
	}

	return &models.TransactionDetail{
		Transaction:  *txn,
		Entries:      entries,
		Categories:   categories,
		AccountNames: accountNames,
	}, nil
}

// Delete removes a transaction and reverses all ledger entries
func (s *TransactionService) Delete(ctx context.Context, transactionID, userID int) error {
	txn, err := s.transactions.GetByID(ctx, transactionID, userID)
	if err != nil {
		return err
	}
	if txn == nil {
		return apperr.ErrNotFound
	}

	// Acquire a connection for the transaction
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	// Start transaction
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get ledger entries
	entries, err := s.ledger.ListByTransaction(ctx, transactionID)
	if err != nil {
		return err
	}

	// Reverse each ledger entry's effect on balance
	for _, entry := range entries {
		var currentBalance float64
		var version int
		err = tx.QueryRow(ctx,
			`SELECT balance, version FROM account_balances WHERE account_id = $1 FOR UPDATE`,
			entry.AccountID,
		).Scan(&currentBalance, &version)
		if err != nil {
			return err
		}

		// Reverse the amount
		newBalance := currentBalance - entry.Amount
		result, err := tx.Exec(ctx,
			`UPDATE account_balances
			 SET balance = $1,
			     last_updated_txn = CASE WHEN last_updated_txn = $4 THEN NULL ELSE last_updated_txn END,
			     version = version + 1,
			     updated_at = now()
			 WHERE account_id = $2 AND version = $3`,
			newBalance, entry.AccountID, version, transactionID,
		)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return errors.New("concurrent balance update detected")
		}
	}

	// Delete ledger entries (will cascade)
	_, err = tx.Exec(ctx, `DELETE FROM ledger_entries WHERE transaction_id = $1`, transactionID)
	if err != nil {
		return err
	}

	// Delete transaction
	result, err := tx.Exec(ctx, `DELETE FROM transactions_v2 WHERE id = $1 AND user_id = $2`, transactionID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}

	return tx.Commit(ctx)
}

// Validation helpers

func (s *TransactionService) validateTransactionInput(in models.TransactionInput, expectedType string) error {
	if in.AccountID <= 0 {
		return apperr.Validation("account_id must be greater than 0")
	}
	if strings.TrimSpace(in.Title) == "" {
		return apperr.Validation("title is required")
	}
	if in.Amount <= 0 {
		return apperr.Validation("amount must be greater than 0")
	}
	if in.Type != expectedType {
		return apperr.Validation("type must be '" + expectedType + "'")
	}
	if strings.TrimSpace(in.Category) == "" {
		return apperr.Validation("category is required")
	}
	return nil
}

func (s *TransactionService) validateTransferInput(in models.TransferInput) error {
	if in.FromAccountID <= 0 {
		return apperr.Validation("from_account_id must be greater than 0")
	}
	if in.ToAccountID <= 0 {
		return apperr.Validation("to_account_id must be greater than 0")
	}
	if in.FromAccountID == in.ToAccountID {
		return apperr.Validation("cannot transfer to the same account")
	}
	if strings.TrimSpace(in.Title) == "" {
		return apperr.Validation("title is required")
	}
	if in.Amount <= 0 {
		return apperr.Validation("amount must be greater than 0")
	}
	return nil
}
