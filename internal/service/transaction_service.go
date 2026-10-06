package service

import (
	"context"
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
		Type:    "expense",
		Title:   strings.TrimSpace(in.Title),
		Amount:  in.Amount,
		TrxCost: in.TrxCost,
		Date:    in.Date,
		Note:    in.Note,
	}, []ledgerLine{
		{AccountID: in.AccountID, Amount: -(in.Amount + in.TrxCost), EntryType: "credit"},
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
		Type:    "transfer",
		Title:   strings.TrimSpace(in.Title),
		Amount:  in.Amount,
		TrxCost: in.TrxCost,
		Date:    in.Date,
		Note:    in.Note,
	}, []ledgerLine{
		{AccountID: in.FromAccountID, Amount: -(in.Amount + in.TrxCost), EntryType: "credit"},
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

	var transactionID int
	if idempotencyKey != "" {
		existingID, err := s.transactions.GetIDByIdempotencyKeyTx(ctx, tx, userID, idempotencyKey)
		if err == nil {
			if err := tx.Commit(ctx); err != nil {
				return nil, fmt.Errorf("commit idempotent replay (transaction=%d): %w", existingID, err)
			}
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

	transactionID, err = s.transactions.CreateTx(ctx, tx, userID, txn, idempotencyKey)

	if errors.Is(err, pgx.ErrNoRows) {
		// Another request with the same key won the race.
		if idempotencyKey == "" {
			return nil, errors.New("transaction insert returned no rows unexpectedly")
		}
		s.log.InfoContext(ctx, "lost idempotency race, fetching winner",
			"user_id", userID,
			"idempotency_key", idempotencyKey,
		)
		winnerID, lookupErr := s.transactions.GetIDByIdempotencyKeyTx(ctx, tx, userID, idempotencyKey)
		if lookupErr != nil {
			return nil, fmt.Errorf("idempotency winner lookup: %w", lookupErr)
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit idempotent winner lookup (transaction=%d): %w", winnerID, err)
		}
		return s.enrichTransactionDetail(ctx, winnerID, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("insert transaction: %w", err)
	}

	for i, e := range entries {
		if err := s.ledger.CreateLedgerEntryTx(ctx, tx, transactionID, e.AccountID, e.Amount, e.EntryType); err != nil {
			return nil, fmt.Errorf("insert ledger entry %d/%d (transaction=%d): %w", i+1, len(entries), transactionID, err)
		}

		currentBalance, version, err := s.balances.GetBalanceForUpdateTx(ctx, tx, e.AccountID)
		if err != nil {
			return nil, fmt.Errorf("select balance (account=%d): %w", e.AccountID, err)
		}

		newBalance := currentBalance + e.Amount
		if newBalance < 0 {
			return nil, apperr.ErrInsufficientFunds
		}
		s.log.DebugContext(ctx, "updating balance",
			"transaction_id", transactionID,
			"account_id", e.AccountID,
			"current_balance", currentBalance,
			"delta", e.Amount,
			"new_balance", newBalance,
			"version", version,
		)

		updated, err := s.balances.UpdateBalanceTx(ctx, tx, e.AccountID, newBalance, transactionID, version)
		if err != nil {
			return nil, fmt.Errorf("update balance (account=%d): %w", e.AccountID, err)
		}
		if !updated {
			s.log.WarnContext(ctx, "balance version conflict",
				"account_id", e.AccountID,
				"expected_version", version,
			)
			return nil, errors.New("concurrent balance update detected, please retry")
		}
	}

	for _, catID := range categoryIDs {
		if err := s.transactions.AddCategoryTx(ctx, tx, transactionID, catID); err != nil {
			return nil, fmt.Errorf("insert category link (transaction=%d, category=%d): %w", transactionID, catID, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit (transaction=%d): %w", transactionID, err)
	}

	s.log.InfoContext(ctx, "transaction created",
		"user_id", userID,
		"transaction_id", transactionID,
		"type", txn.Type,
		"entries", len(entries),
		"idempotency_key", idempotencyKey,
	)

	return s.enrichTransactionDetail(ctx, transactionID, userID)
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

	entries, err := s.ledger.ListByTransactionTx(ctx, tx, transactionID)
	if err != nil {
		return fmt.Errorf("list ledger entries (transaction=%d): %w", transactionID, err)
	}

	for _, entry := range entries {
		currentBalance, version, err := s.balances.GetBalanceForUpdateTx(ctx, tx, entry.AccountID)
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

		updated, err := s.balances.ReverseBalanceTx(ctx, tx, entry.AccountID, newBalance, transactionID, version)
		if err != nil {
			return fmt.Errorf("reverse balance (account=%d): %w", entry.AccountID, err)
		}
		if !updated {
			s.log.WarnContext(ctx, "balance version conflict",
				"account_id", entry.AccountID,
				"expected_version", version,
			)
			return errors.New("concurrent balance update detected")
		}
	}

	if err = s.ledger.DeleteByTransactionTx(ctx, tx, transactionID); err != nil {
		return fmt.Errorf("delete ledger entries (transaction=%d): %w", transactionID, err)
	}

	deleted, err := s.transactions.DeleteTx(ctx, tx, transactionID, userID)
	if err != nil {
		return fmt.Errorf("delete transaction %d: %w", transactionID, err)
	}
	if !deleted {
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
	if in.TrxCost < 0 {
		return apperr.Validation("transaction_cost must not be negative")
	}
	if in.Type != expectedType {
		return apperr.Validation("type must be '" + expectedType + "'")
	}
	if expectedType == "income" && in.TrxCost != 0 {
		return apperr.Validation("transaction_cost is only supported for expenses and transfers")
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
	if in.TrxCost < 0 {
		return apperr.Validation("transaction_cost must not be negative")
	}
	return nil
}
