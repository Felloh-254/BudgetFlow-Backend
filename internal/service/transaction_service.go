package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
	db           *pgxpool.Pool
	log          *slog.Logger
}

// ledgerLine is one account movement inside a transaction.
type ledgerLine struct {
	AccountID int
	Amount    float64
	EntryType string
}

func NewTransactionService(
	transactions *repository.TransactionRepository,
	categories *repository.CategoryRepository,
	accounts *repository.AccountsRepository,
	ledger *repository.LedgerRepository,
	balances *repository.AccountBalanceRepository,
	db *pgxpool.Pool,
	log *slog.Logger,
) *TransactionService {
	return &TransactionService{
		transactions: transactions,
		categories:   categories,
		accounts:     accounts,
		ledger:       ledger,
		balances:     balances,
		db:           db,
		log:          log.With("component", "service.transaction"),
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

	txns, err := s.transactions.ListByUser(ctx, userID, filter)
	if err != nil {
		return nil, fmt.Errorf("list transactions (user=%d): %w", userID, err)
	}
	return txns, nil
}

// GetByID retrieves a single transaction with all details
func (s *TransactionService) GetByID(ctx context.Context, transactionID, userID int) (*models.TransactionDetail, error) {
	detail, err := s.enrichTransactionDetail(ctx, transactionID, userID)
	if err != nil {
		return nil, fmt.Errorf("get transaction %d: %w", transactionID, err)
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
		return nil, fmt.Errorf("update transaction %d: load: %w", transactionID, err)
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
		s.log.DebugContext(ctx, "update requested no changes", "transaction_id", transactionID)
		return s.enrichTransactionDetail(ctx, transactionID, userID)
	}

	if _, err = s.transactions.Update(ctx, transactionID, userID, title, date, note); err != nil {
		return nil, fmt.Errorf("update transaction %d: %w", transactionID, err)
	}

	if strings.TrimSpace(in.Category) != "" && txn.Type != "transfer" {
		cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), txn.Type)
		if err != nil {
			return nil, fmt.Errorf("update transaction %d: find/create category: %w", transactionID, err)
		}
		if err := s.transactions.SetCategory(ctx, transactionID, cat.ID); err != nil {
			return nil, fmt.Errorf("update transaction %d: set category: %w", transactionID, err)
		}
	}

	s.log.InfoContext(ctx, "transaction updated", "user_id", userID, "transaction_id", transactionID)
	return s.enrichTransactionDetail(ctx, transactionID, userID)
}

// CreateIncome creates an income transaction
func (s *TransactionService) CreateIncome(ctx context.Context, userID int, in models.TransactionInput, idempotencyKey string) (*models.TransactionDetail, error) {
	if err := s.validateTransactionInput(in, "income"); err != nil {
		return nil, err
	}
	if err := s.requireAccount(ctx, in.AccountID, userID); err != nil {
		return nil, err
	}
	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "income")
	if err != nil {
		return nil, fmt.Errorf("create income: category: %w", err)
	}

	return s.createTransactionWithLedgerEntries(ctx, userID, models.Transaction{
		Type:  "income",
		Title: strings.TrimSpace(in.Title),
		Date:  in.Date,
		Note:  in.Note,
	}, []ledgerLine{
		{AccountID: in.AccountID, Amount: in.Amount, EntryType: "debit"},
	}, []int{cat.ID}, idempotencyKey)
}

// CreateExpense creates an expense transaction
func (s *TransactionService) CreateExpense(ctx context.Context, userID int, in models.TransactionInput, idempotencyKey string) (*models.TransactionDetail, error) {
	if err := s.validateTransactionInput(in, "expense"); err != nil {
		return nil, err
	}
	if err := s.requireAccount(ctx, in.AccountID, userID); err != nil {
		return nil, err
	}
	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "expense")
	if err != nil {
		return nil, fmt.Errorf("create expense: category: %w", err)
	}

	return s.createTransactionWithLedgerEntries(ctx, userID, models.Transaction{
		Type:  "expense",
		Title: strings.TrimSpace(in.Title),
		Date:  in.Date,
		Note:  in.Note,
	}, []ledgerLine{
		{AccountID: in.AccountID, Amount: -in.Amount, EntryType: "credit"},
	}, []int{cat.ID}, idempotencyKey)
}

// CreateTransfer creates a transfer transaction
func (s *TransactionService) CreateTransfer(ctx context.Context, userID int, in models.TransferInput, idempotencyKey string) (*models.TransactionDetail, error) {
	if err := s.validateTransferInput(in); err != nil {
		return nil, err
	}
	if err := s.requireAccount(ctx, in.FromAccountID, userID); err != nil {
		return nil, err
	}
	if err := s.requireAccount(ctx, in.ToAccountID, userID); err != nil {
		return nil, err
	}
	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}

	return s.createTransactionWithLedgerEntries(ctx, userID, models.Transaction{
		Type:  "transfer",
		Title: strings.TrimSpace(in.Title),
		Date:  in.Date,
		Note:  in.Note,
	}, []ledgerLine{
		{AccountID: in.FromAccountID, Amount: -in.Amount, EntryType: "credit"},
		{AccountID: in.ToAccountID, Amount: in.Amount, EntryType: "debit"},
	}, []int{}, idempotencyKey)
}

// requireAccount returns apperr.ErrNotFound unless the account belongs to the user.
func (s *TransactionService) requireAccount(ctx context.Context, accountID, userID int) error {
	exists, err := s.accounts.ExistsForUser(ctx, accountID, userID)
	if err != nil {
		return fmt.Errorf("check account %d: %w", accountID, err)
	}
	if !exists {
		return apperr.ErrNotFound
	}
	return nil
}

// createTransactionWithLedgerEntries atomically creates a transaction, ledger entries, and updates balances
func (s *TransactionService) createTransactionWithLedgerEntries(
	ctx context.Context,
	userID int,
	txn models.Transaction,
	entries []ledgerLine,
	categoryIDs []int,
	idempotencyKey string,
) (*models.TransactionDetail, error) {
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Fast-path idempotency check
	if idempotencyKey != "" {
		var existingID int
		err := tx.QueryRow(ctx,
			`SELECT id FROM transactions_v2 WHERE idempotency_key = $1 AND user_id = $2 LIMIT 1`,
			idempotencyKey, userID,
		).Scan(&existingID)
		if err == nil {
			s.log.InfoContext(ctx, "idempotent replay, returning existing transaction",
				"user_id", userID,
				"idempotency_key", idempotencyKey,
				"transaction_id", existingID,
			)
			return s.enrichTransactionDetail(ctx, existingID, userID)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("idempotency lookup: %w", err)
		}
	}

	var createdTxn models.Transaction
	var idempKey sql.NullString

	var idempotencyKeyParam interface{}
	if idempotencyKey != "" {
		idempotencyKeyParam = idempotencyKey
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO transactions_v2 (user_id, type, title, date, note, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (user_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
		 RETURNING id, user_id, type, title, to_char(date, 'YYYY-MM-DD'), note, idempotency_key, created_at, updated_at`,
		userID, txn.Type, txn.Title, txn.Date, txn.Note, idempotencyKeyParam,
	).Scan(&createdTxn.ID, &createdTxn.UserID, &createdTxn.Type, &createdTxn.Title, &createdTxn.Date, &createdTxn.Note, &idempKey, &createdTxn.CreatedAt, &createdTxn.UpdatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		// Another request with the same key won the race.
		if idempotencyKey == "" {
			return nil, errors.New("transaction insert returned no rows unexpectedly")
		}
		s.log.InfoContext(ctx, "lost idempotency race, fetching winner",
			"user_id", userID,
			"idempotency_key", idempotencyKey,
		)
		var winnerID int
		lookupErr := s.db.QueryRow(ctx,
			`SELECT id FROM transactions_v2 WHERE idempotency_key = $1 AND user_id = $2`,
			idempotencyKey, userID,
		).Scan(&winnerID)
		if lookupErr != nil {
			return nil, fmt.Errorf("idempotency winner lookup: %w", lookupErr)
		}
		return s.enrichTransactionDetail(ctx, winnerID, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("insert transaction: %w", err)
	}

	if idempKey.Valid {
		createdTxn.IdempotencyKey = &idempKey.String
	}

	for i, e := range entries {
		if _, err := tx.Exec(ctx,
			`INSERT INTO ledger_entries (transaction_id, account_id, amount, entry_type)
			 VALUES ($1, $2, $3, $4)`,
			createdTxn.ID, e.AccountID, e.Amount, e.EntryType,
		); err != nil {
			return nil, fmt.Errorf("insert ledger entry %d/%d (transaction=%d): %w", i+1, len(entries), createdTxn.ID, err)
		}

		var currentBalance float64
		var version int
		err = tx.QueryRow(ctx,
			`SELECT balance, version FROM account_balances WHERE account_id = $1 FOR UPDATE`,
			e.AccountID,
		).Scan(&currentBalance, &version)
		if err != nil {
			return nil, fmt.Errorf("select balance (account=%d): %w", e.AccountID, err)
		}

		newBalance := currentBalance + e.Amount
		if newBalance < 0 {
			return nil, apperr.ErrInsufficientFunds
		}
		s.log.DebugContext(ctx, "updating balance",
			"transaction_id", createdTxn.ID,
			"account_id", e.AccountID,
			"current_balance", currentBalance,
			"delta", e.Amount,
			"new_balance", newBalance,
			"version", version,
		)

		result, err := tx.Exec(ctx,
			`UPDATE account_balances
			 SET balance = $1, last_updated_txn = $2, version = version + 1, updated_at = now()
			 WHERE account_id = $3 AND version = $4`,
			newBalance, createdTxn.ID, e.AccountID, version,
		)
		if err != nil {
			return nil, fmt.Errorf("update balance (account=%d): %w", e.AccountID, err)
		}
		if result.RowsAffected() == 0 {
			s.log.WarnContext(ctx, "balance version conflict",
				"account_id", e.AccountID,
				"expected_version", version,
			)
			return nil, errors.New("concurrent balance update detected, please retry")
		}
	}

	for _, catID := range categoryIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO transaction_categories (transaction_id, category_id)
			 VALUES ($1, $2)
			 ON CONFLICT (transaction_id, category_id) DO NOTHING`,
			createdTxn.ID, catID,
		); err != nil {
			return nil, fmt.Errorf("insert category link (transaction=%d, category=%d): %w", createdTxn.ID, catID, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit (transaction=%d): %w", createdTxn.ID, err)
	}

	s.log.InfoContext(ctx, "transaction created",
		"user_id", userID,
		"transaction_id", createdTxn.ID,
		"type", txn.Type,
		"entries", len(entries),
		"idempotency_key", idempotencyKey,
	)

	return s.enrichTransactionDetail(ctx, createdTxn.ID, userID)
}

// enrichTransactionDetail loads a transaction with all related data
func (s *TransactionService) enrichTransactionDetail(ctx context.Context, txnID, userID int) (*models.TransactionDetail, error) {
	txn, err := s.transactions.GetByID(ctx, txnID, userID)
	if err != nil || txn == nil {
		return nil, err
	}

	entries, err := s.ledger.ListByTransaction(ctx, txnID)
	if err != nil {
		return nil, fmt.Errorf("list ledger entries (transaction=%d): %w", txnID, err)
	}

	categories, err := s.transactions.GetCategories(ctx, txnID)
	if err != nil {
		return nil, fmt.Errorf("get categories (transaction=%d): %w", txnID, err)
	}

	accountNames := make(map[int]string)
	for _, entry := range entries {
		acc, err := s.accounts.GetAccountByID(ctx, entry.AccountID, userID)
		if err != nil {
			// Non-fatal: the detail is still useful without the name.
			s.log.WarnContext(ctx, "account name lookup failed",
				"account_id", entry.AccountID,
				"error", err,
			)
			continue
		}
		if acc != nil {
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
		return fmt.Errorf("delete transaction %d: load: %w", transactionID, err)
	}
	if txn == nil {
		return apperr.ErrNotFound
	}

	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	entries, err := s.ledger.ListByTransaction(ctx, transactionID)
	if err != nil {
		return fmt.Errorf("list ledger entries (transaction=%d): %w", transactionID, err)
	}

	for _, entry := range entries {
		var currentBalance float64
		var version int
		err = tx.QueryRow(ctx,
			`SELECT balance, version FROM account_balances WHERE account_id = $1 FOR UPDATE`,
			entry.AccountID,
		).Scan(&currentBalance, &version)
		if err != nil {
			return fmt.Errorf("select balance (account=%d): %w", entry.AccountID, err)
		}

		newBalance := currentBalance - entry.Amount
		if newBalance < 0 {
			return apperr.ErrInsufficientFunds
		}
		s.log.DebugContext(ctx, "reversing ledger entry",
			"transaction_id", transactionID,
			"account_id", entry.AccountID,
			"current_balance", currentBalance,
			"reverse_amount", entry.Amount,
			"new_balance", newBalance,
		)

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
			return fmt.Errorf("reverse balance (account=%d): %w", entry.AccountID, err)
		}
		if result.RowsAffected() == 0 {
			s.log.WarnContext(ctx, "balance version conflict",
				"account_id", entry.AccountID,
				"expected_version", version,
			)
			return errors.New("concurrent balance update detected")
		}
	}

	if _, err = tx.Exec(ctx, `DELETE FROM ledger_entries WHERE transaction_id = $1`, transactionID); err != nil {
		return fmt.Errorf("delete ledger entries (transaction=%d): %w", transactionID, err)
	}

	result, err := tx.Exec(ctx, `DELETE FROM transactions_v2 WHERE id = $1 AND user_id = $2`, transactionID, userID)
	if err != nil {
		return fmt.Errorf("delete transaction %d: %w", transactionID, err)
	}
	if result.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete (transaction=%d): %w", transactionID, err)
	}

	s.log.InfoContext(ctx, "transaction deleted",
		"user_id", userID,
		"transaction_id", transactionID,
		"entries_reversed", len(entries),
	)
	return nil
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
