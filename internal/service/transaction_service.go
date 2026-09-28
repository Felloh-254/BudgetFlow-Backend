package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
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
}

func NewTransactionService(
	transactions *repository.TransactionRepository,
	categories *repository.CategoryRepository,
	accounts *repository.AccountsRepository,
	ledger *repository.LedgerRepository,
	balances *repository.AccountBalanceRepository,
	db *pgxpool.Pool,
) *TransactionService {
	log.Println("[service.transaction] NewTransactionService: created")
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
	log.Printf("[service.transaction] List: user_id=%d limit=%d offset=%d type=%q",
		userID, filter.Limit, filter.Offset, filter.Type)

	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	txns, err := s.transactions.ListByUser(ctx, userID, filter)
	if err != nil {
		log.Printf("[service.transaction] List: repo error user_id=%d error=%v", userID, err)
		return nil, err
	}

	log.Printf("[service.transaction] List: OK user_id=%d count=%d", userID, len(txns))
	return txns, nil
}

// GetByID retrieves a single transaction with all details
func (s *TransactionService) GetByID(ctx context.Context, transactionID, userID int) (*models.TransactionDetail, error) {
	log.Printf("[service.transaction] GetByID: transaction_id=%d user_id=%d", transactionID, userID)

	detail, err := s.enrichTransactionDetail(ctx, transactionID, userID)
	if err != nil {
		log.Printf("[service.transaction] GetByID: enrich failed transaction_id=%d error=%v", transactionID, err)
		return nil, err
	}
	if detail == nil {
		log.Printf("[service.transaction] GetByID: not found transaction_id=%d user_id=%d", transactionID, userID)
		return nil, apperr.ErrNotFound
	}

	log.Printf("[service.transaction] GetByID: OK transaction_id=%d type=%s entries=%d",
		transactionID, detail.Transaction.Type, len(detail.Entries))
	return detail, nil
}

// Update updates transaction metadata (title, date, note) and optionally category
func (s *TransactionService) Update(ctx context.Context, transactionID, userID int, in models.UpdateTransactionInput) (*models.TransactionDetail, error) {
	log.Printf("[service.transaction] Update: transaction_id=%d user_id=%d title=%q date=%q category=%q",
		transactionID, userID, in.Title, in.Date, in.Category)

	txn, err := s.transactions.GetByID(ctx, transactionID, userID)
	if err != nil {
		log.Printf("[service.transaction] Update: get failed transaction_id=%d error=%v", transactionID, err)
		return nil, err
	}
	if txn == nil {
		log.Printf("[service.transaction] Update: not found transaction_id=%d user_id=%d", transactionID, userID)
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
		log.Printf("[service.transaction] Update: no changes requested transaction_id=%d", transactionID)
		return s.enrichTransactionDetail(ctx, transactionID, userID)
	}

	_, err = s.transactions.Update(ctx, transactionID, userID, title, date, note)
	if err != nil {
		log.Printf("[service.transaction] Update: repo update failed transaction_id=%d error=%v", transactionID, err)
		return nil, err
	}

	if strings.TrimSpace(in.Category) != "" && txn.Type != "transfer" {
		catType := txn.Type
		cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), catType)
		if err != nil {
			log.Printf("[service.transaction] Update: category find/create failed transaction_id=%d error=%v", transactionID, err)
			return nil, err
		}
		if err := s.transactions.SetCategory(ctx, transactionID, cat.ID); err != nil {
			log.Printf("[service.transaction] Update: set category failed transaction_id=%d error=%v", transactionID, err)
			return nil, err
		}
	}

	log.Printf("[service.transaction] Update: OK transaction_id=%d", transactionID)
	return s.enrichTransactionDetail(ctx, transactionID, userID)
}

// CreateIncome creates an income transaction
func (s *TransactionService) CreateIncome(ctx context.Context, userID int, in models.TransactionInput, idempotencyKey string) (*models.TransactionDetail, error) {
	log.Printf("[service.transaction] CreateIncome: ENTER user_id=%d idempotency_key=%s amount=%.2f account_id=%d title=%q",
		userID, idempotencyKey, in.Amount, in.AccountID, in.Title)

	if err := s.validateTransactionInput(in, "income"); err != nil {
		log.Printf("[service.transaction] CreateIncome: validation failed user_id=%d error=%v", userID, err)
		return nil, err
	}

	if exists, err := s.accounts.ExistsForUser(ctx, in.AccountID, userID); err != nil {
		log.Printf("[service.transaction] CreateIncome: account check failed user_id=%d account_id=%d error=%v", userID, in.AccountID, err)
		return nil, err
	} else if !exists {
		log.Printf("[service.transaction] CreateIncome: account not found user_id=%d account_id=%d", userID, in.AccountID)
		return nil, apperr.ErrNotFound
	}

	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "income")
	if err != nil {
		log.Printf("[service.transaction] CreateIncome: category failed user_id=%d error=%v", userID, err)
		return nil, err
	}

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
	}, []int{cat.ID}, idempotencyKey)

	if err != nil {
		log.Printf("[service.transaction] CreateIncome: EXIT ERROR user_id=%d idempotency_key=%s error=%v", userID, idempotencyKey, err)
	} else {
		log.Printf("[service.transaction] CreateIncome: EXIT OK user_id=%d idempotency_key=%s transaction_id=%d",
			userID, idempotencyKey, detail.Transaction.ID)
	}
	return detail, err
}

// CreateExpense creates an expense transaction
func (s *TransactionService) CreateExpense(ctx context.Context, userID int, in models.TransactionInput, idempotencyKey string) (*models.TransactionDetail, error) {
	log.Printf("[service.transaction] CreateExpense: ENTER user_id=%d idempotency_key=%s amount=%.2f account_id=%d title=%q",
		userID, idempotencyKey, in.Amount, in.AccountID, in.Title)

	if err := s.validateTransactionInput(in, "expense"); err != nil {
		log.Printf("[service.transaction] CreateExpense: validation failed user_id=%d error=%v", userID, err)
		return nil, err
	}

	if exists, err := s.accounts.ExistsForUser(ctx, in.AccountID, userID); err != nil {
		log.Printf("[service.transaction] CreateExpense: account check failed user_id=%d account_id=%d error=%v", userID, in.AccountID, err)
		return nil, err
	} else if !exists {
		log.Printf("[service.transaction] CreateExpense: account not found user_id=%d account_id=%d", userID, in.AccountID)
		return nil, apperr.ErrNotFound
	}

	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "expense")
	if err != nil {
		log.Printf("[service.transaction] CreateExpense: category failed user_id=%d error=%v", userID, err)
		return nil, err
	}

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
	}, []int{cat.ID}, idempotencyKey)

	if err != nil {
		log.Printf("[service.transaction] CreateExpense: EXIT ERROR user_id=%d idempotency_key=%s error=%v", userID, idempotencyKey, err)
	} else {
		log.Printf("[service.transaction] CreateExpense: EXIT OK user_id=%d idempotency_key=%s transaction_id=%d",
			userID, idempotencyKey, detail.Transaction.ID)
	}
	return detail, err
}

// CreateTransfer creates a transfer transaction
func (s *TransactionService) CreateTransfer(ctx context.Context, userID int, in models.TransferInput, idempotencyKey string) (*models.TransactionDetail, error) {
	log.Printf("[service.transaction] CreateTransfer: ENTER user_id=%d idempotency_key=%s amount=%.2f from=%d to=%d",
		userID, idempotencyKey, in.Amount, in.FromAccountID, in.ToAccountID)

	if err := s.validateTransferInput(in); err != nil {
		log.Printf("[service.transaction] CreateTransfer: validation failed user_id=%d error=%v", userID, err)
		return nil, err
	}

	if exists, err := s.accounts.ExistsForUser(ctx, in.FromAccountID, userID); err != nil {
		return nil, err
	} else if !exists {
		log.Printf("[service.transaction] CreateTransfer: from account not found user_id=%d account_id=%d", userID, in.FromAccountID)
		return nil, apperr.ErrNotFound
	}

	if exists, err := s.accounts.ExistsForUser(ctx, in.ToAccountID, userID); err != nil {
		return nil, err
	} else if !exists {
		log.Printf("[service.transaction] CreateTransfer: to account not found user_id=%d account_id=%d", userID, in.ToAccountID)
		return nil, apperr.ErrNotFound
	}

	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}

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
	}, []int{}, idempotencyKey)

	if err != nil {
		log.Printf("[service.transaction] CreateTransfer: EXIT ERROR user_id=%d idempotency_key=%s error=%v", userID, idempotencyKey, err)
	} else {
		log.Printf("[service.transaction] CreateTransfer: EXIT OK user_id=%d idempotency_key=%s transaction_id=%d",
			userID, idempotencyKey, detail.Transaction.ID)
	}
	return detail, err
}

// createTransactionWithLedgerEntries atomically creates a transaction, ledger entries, and updates balances
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
	idempotencyKey string,
) (*models.TransactionDetail, error) {
	log.Printf("[service.transaction] createTransactionWithLedgerEntries: ENTER user_id=%d type=%s idempotency_key=%s num_entries=%d",
		userID, txn.Type, idempotencyKey, len(entries))

	conn, err := s.db.Acquire(ctx)
	if err != nil {
		log.Printf("[service.transaction] createTransactionWithLedgerEntries: acquire conn failed error=%v", err)
		return nil, err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Printf("[service.transaction] createTransactionWithLedgerEntries: begin tx failed error=%v", err)
		return nil, err
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
			log.Printf("[service.transaction] idempotency fast-path HIT: user_id=%d idempotency_key=%s existing_transaction_id=%d — returning existing",
				userID, idempotencyKey, existingID)
			return s.enrichTransactionDetail(ctx, existingID, userID)
		}
		if err != pgx.ErrNoRows {
			return nil, err
		}
		log.Printf("[service.transaction] idempotency fast-path MISS: user_id=%d idempotency_key=%s — proceeding to insert",
			userID, idempotencyKey)
	}

	var createdTxn models.Transaction
	var idempKey sql.NullString

	var idempotencyKeyParam interface{} = nil
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

	if err == pgx.ErrNoRows {
		log.Printf("[service.transaction] INSERT lost ON CONFLICT race: user_id=%d idempotency_key=%s — fetching winner's row",
			userID, idempotencyKey)
		if idempotencyKey == "" {
			return nil, errors.New("transaction insert returned no rows unexpectedly")
		}
		var winnerID int
		lookupErr := s.db.QueryRow(ctx,
			`SELECT id FROM transactions_v2 WHERE idempotency_key = $1 AND user_id = $2`,
			idempotencyKey, userID,
		).Scan(&winnerID)
		if lookupErr != nil {
			return nil, lookupErr
		}
		return s.enrichTransactionDetail(ctx, winnerID, userID)
	}
	if err != nil {
		log.Printf("[service.transaction] INSERT failed: user_id=%d idempotency_key=%s error=%v", userID, idempotencyKey, err)
		return nil, err
	}

	if idempKey.Valid {
		createdTxn.IdempotencyKey = &idempKey.String
	}

	log.Printf("[service.transaction] INSERT won: user_id=%d idempotency_key=%s new_transaction_id=%d — writing %d ledger entries",
		userID, idempotencyKey, createdTxn.ID, len(entries))

	for i, e := range entries {
		_, err := tx.Exec(ctx,
			`INSERT INTO ledger_entries (transaction_id, account_id, amount, entry_type)
			 VALUES ($1, $2, $3, $4)`,
			createdTxn.ID, e.AccountID, e.Amount, e.EntryType,
		)
		if err != nil {
			log.Printf("[service.transaction] ledger entry %d/%d insert failed: transaction_id=%d error=%v",
				i+1, len(entries), createdTxn.ID, err)
			return nil, err
		}
		log.Printf("[service.transaction] ledger entry %d/%d inserted: transaction_id=%d account_id=%d amount=%.2f entry_type=%s",
			i+1, len(entries), createdTxn.ID, e.AccountID, e.Amount, e.EntryType)

		var currentBalance float64
		var version int
		err = tx.QueryRow(ctx,
			`SELECT balance, version FROM account_balances WHERE account_id = $1 FOR UPDATE`,
			e.AccountID,
		).Scan(&currentBalance, &version)
		if err != nil {
			log.Printf("[service.transaction] balance select failed: account_id=%d error=%v", e.AccountID, err)
			return nil, err
		}

		newBalance := currentBalance + e.Amount
		log.Printf("[service.transaction] balance update: transaction_id=%d account_id=%d current_balance=%.2f delta=%.2f new_balance=%.2f version=%d",
			createdTxn.ID, e.AccountID, currentBalance, e.Amount, newBalance, version)

		result, err := tx.Exec(ctx,
			`UPDATE account_balances
			 SET balance = $1, last_updated_txn = $2, version = version + 1, updated_at = now()
			 WHERE account_id = $3 AND version = $4`,
			newBalance, createdTxn.ID, e.AccountID, version,
		)
		if err != nil {
			log.Printf("[service.transaction] balance update failed: account_id=%d error=%v", e.AccountID, err)
			return nil, err
		}
		if result.RowsAffected() == 0 {
			log.Printf("[service.transaction] balance update CONFLICT: account_id=%d expected_version=%d", e.AccountID, version)
			return nil, errors.New("concurrent balance update detected, please retry")
		}
	}

	for _, catID := range categoryIDs {
		_, err := tx.Exec(ctx,
			`INSERT INTO transaction_categories (transaction_id, category_id)
			 VALUES ($1, $2)
			 ON CONFLICT (transaction_id, category_id) DO NOTHING`,
			createdTxn.ID, catID,
		)
		if err != nil {
			log.Printf("[service.transaction] category insert failed: transaction_id=%d category_id=%d error=%v",
				createdTxn.ID, catID, err)
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		log.Printf("[service.transaction] COMMIT FAILED: user_id=%d idempotency_key=%s transaction_id=%d error=%v",
			userID, idempotencyKey, createdTxn.ID, err)
		return nil, err
	}
	log.Printf("[service.transaction] COMMIT OK: user_id=%d idempotency_key=%s transaction_id=%d", userID, idempotencyKey, createdTxn.ID)

	return s.enrichTransactionDetail(ctx, createdTxn.ID, userID)
}

// enrichTransactionDetail loads a transaction with all related data
func (s *TransactionService) enrichTransactionDetail(ctx context.Context, txnID, userID int) (*models.TransactionDetail, error) {
	log.Printf("[service.transaction] enrichTransactionDetail: transaction_id=%d user_id=%d", txnID, userID)

	txn, err := s.transactions.GetByID(ctx, txnID, userID)
	if err != nil || txn == nil {
		return nil, err
	}

	entries, err := s.ledger.ListByTransaction(ctx, txnID)
	if err != nil {
		log.Printf("[service.transaction] enrichTransactionDetail: ledger list failed transaction_id=%d error=%v", txnID, err)
		return nil, err
	}

	categories, err := s.transactions.GetCategories(ctx, txnID)
	if err != nil {
		log.Printf("[service.transaction] enrichTransactionDetail: categories failed transaction_id=%d error=%v", txnID, err)
		return nil, err
	}

	accountNames := make(map[int]string)
	for _, entry := range entries {
		acc, err := s.accounts.GetAccountByID(ctx, entry.AccountID, userID)
		if err == nil && acc != nil {
			accountNames[entry.AccountID] = acc.Name
		}
	}

	log.Printf("[service.transaction] enrichTransactionDetail: OK transaction_id=%d entries=%d categories=%d accounts=%d",
		txnID, len(entries), len(categories), len(accountNames))

	return &models.TransactionDetail{
		Transaction:  *txn,
		Entries:      entries,
		Categories:   categories,
		AccountNames: accountNames,
	}, nil
}

// Delete removes a transaction and reverses all ledger entries
func (s *TransactionService) Delete(ctx context.Context, transactionID, userID int) error {
	log.Printf("[service.transaction] Delete: ENTER transaction_id=%d user_id=%d", transactionID, userID)

	txn, err := s.transactions.GetByID(ctx, transactionID, userID)
	if err != nil {
		return err
	}
	if txn == nil {
		log.Printf("[service.transaction] Delete: not found transaction_id=%d user_id=%d", transactionID, userID)
		return apperr.ErrNotFound
	}

	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	entries, err := s.ledger.ListByTransaction(ctx, transactionID)
	if err != nil {
		return err
	}

	log.Printf("[service.transaction] Delete: reversing %d ledger entries transaction_id=%d", len(entries), transactionID)

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

		newBalance := currentBalance - entry.Amount
		log.Printf("[service.transaction] Delete: reversing entry transaction_id=%d account_id=%d current=%.2f reverse=%.2f new=%.2f",
			transactionID, entry.AccountID, currentBalance, entry.Amount, newBalance)

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
			log.Printf("[service.transaction] Delete: balance CONFLICT account_id=%d version=%d", entry.AccountID, version)
			return errors.New("concurrent balance update detected")
		}
	}

	_, err = tx.Exec(ctx, `DELETE FROM ledger_entries WHERE transaction_id = $1`, transactionID)
	if err != nil {
		return err
	}

	result, err := tx.Exec(ctx, `DELETE FROM transactions_v2 WHERE id = $1 AND user_id = $2`, transactionID, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperr.ErrNotFound
	}

	if err = tx.Commit(ctx); err != nil {
		log.Printf("[service.transaction] Delete: COMMIT FAILED transaction_id=%d error=%v", transactionID, err)
		return err
	}

	log.Printf("[service.transaction] Delete: OK transaction_id=%d user_id=%d", transactionID, userID)
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
