package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository struct {
	db *pgxpool.Pool
}

func NewTransactionRepository(db *pgxpool.Pool) *TransactionRepository {
	log.Println("[repo.transaction] NewTransactionRepository: created")
	return &TransactionRepository{db: db}
}

const enrichedTransactionSelect = `
	SELECT 
		t.id, 
		t.user_id, 
		t.type, 
		t.title, 
		t.transaction_cost,
		to_char(t.date, 'YYYY-MM-DD') as date, 
		t.note, 
		t.idempotency_key, 
		t.created_at, 
		t.updated_at,
		COALESCE(
			MAX(CASE WHEN t.type = 'transfer' AND le.amount > 0 THEN le.amount
			         WHEN t.type != 'transfer' THEN ABS(le.amount)
			    END), 0
		) - CASE WHEN t.type = 'expense' THEN t.transaction_cost ELSE 0 END as amount,
		COALESCE(MAX(c.name), '') as category,
		MAX(c.id) as category_id,
		MAX(CASE WHEN t.type != 'transfer' THEN a.id END) as account_id,
		COALESCE(MAX(CASE WHEN t.type != 'transfer' THEN a.name END), '') as account_name,
		MAX(CASE WHEN t.type = 'transfer' AND le.amount < 0 THEN a.id END) as from_account_id,
		COALESCE(MAX(CASE WHEN t.type = 'transfer' AND le.amount < 0 THEN a.name END), '') as from_account_name,
		MAX(CASE WHEN t.type = 'transfer' AND le.amount > 0 THEN a.id END) as to_account_id,
		COALESCE(MAX(CASE WHEN t.type = 'transfer' AND le.amount > 0 THEN a.name END), '') as to_account_name
	FROM transactions_v2 t
	LEFT JOIN ledger_entries le ON le.transaction_id = t.id
	LEFT JOIN accounts a ON a.id = le.account_id
	LEFT JOIN transaction_categories tc ON tc.transaction_id = t.id
	LEFT JOIN categories c ON c.id = tc.category_id
`

func scanEnrichedTransaction(row interface{ Scan(dest ...any) error }) (*models.Transaction, error) {
	var t models.Transaction
	var idempKey sql.NullString
	var catID, accID, fromAccID, toAccID sql.NullInt64
	err := row.Scan(
		&t.ID, &t.UserID, &t.Type, &t.Title, &t.TrxCost, &t.Date, &t.Note, &idempKey, &t.CreatedAt, &t.UpdatedAt,
		&t.Amount, &t.Category, &catID, &accID, &t.AccountName,
		&fromAccID, &t.FromAccountName, &toAccID, &t.ToAccountName,
	)
	if err != nil {
		return nil, err
	}
	if idempKey.Valid {
		t.IdempotencyKey = &idempKey.String
	}
	if catID.Valid {
		v := int(catID.Int64)
		t.CategoryID = &v
	}
	if accID.Valid {
		v := int(accID.Int64)
		t.AccountID = &v
	}
	if fromAccID.Valid {
		v := int(fromAccID.Int64)
		t.FromAccountID = &v
	}
	if toAccID.Valid {
		v := int(toAccID.Int64)
		t.ToAccountID = &v
	}
	return &t, nil
}

// ListByUser returns all transactions for a user with filters and pagination
func (r *TransactionRepository) ListByUser(ctx context.Context, userID int, filter models.TransactionFilter) ([]models.Transaction, error) {
	log.Printf("[repo.transaction] ListByUser: user_id=%d limit=%d offset=%d type=%q month=%q category_id=%d account_id=%d",
		userID, filter.Limit, filter.Offset, filter.Type, filter.Month, filter.CategoryID, filter.AccountID)

	query := enrichedTransactionSelect + ` WHERE t.user_id = $1`
	args := []any{userID}

	if filter.Type != "" {
		args = append(args, filter.Type)
		query += fmt.Sprintf(" AND t.type = $%d", len(args))
	}
	if filter.Month != "" {
		args = append(args, filter.Month)
		query += fmt.Sprintf(" AND to_char(t.date, 'YYYY-MM') = $%d", len(args))
	}
	if filter.StartDate != "" {
		args = append(args, filter.StartDate)
		query += fmt.Sprintf(" AND t.date >= $%d", len(args))
	}
	if filter.EndDate != "" {
		args = append(args, filter.EndDate)
		query += fmt.Sprintf(" AND t.date <= $%d", len(args))
	}
	if filter.CategoryID > 0 {
		args = append(args, filter.CategoryID)
		query += fmt.Sprintf(" AND tc.category_id = $%d", len(args))
	}
	if filter.AccountID > 0 {
		args = append(args, filter.AccountID)
		query += fmt.Sprintf(" AND le.account_id = $%d", len(args))
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	args = append(args, limit)
	limitArgPos := len(args)
	args = append(args, offset)
	offsetArgPos := len(args)

	query += fmt.Sprintf(`
		GROUP BY t.id
		ORDER BY t.date DESC, t.created_at DESC
		LIMIT $%d OFFSET $%d`, limitArgPos, offsetArgPos)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		log.Printf("[repo.transaction] ListByUser: query failed user_id=%d error=%v", userID, err)
		return nil, err
	}
	defer rows.Close()

	txns := []models.Transaction{}
	for rows.Next() {
		t, err := scanEnrichedTransaction(rows)
		if err != nil {
			log.Printf("[repo.transaction] ListByUser: scan failed user_id=%d error=%v", userID, err)
			return nil, err
		}
		txns = append(txns, *t)
	}

	log.Printf("[repo.transaction] ListByUser: OK user_id=%d count=%d", userID, len(txns))
	return txns, rows.Err()
}

// Create creates a transaction event (without ledger entries)
func (r *TransactionRepository) Create(ctx context.Context, userID int, txnType, title, date, note string, idempotencyKey *string) (*models.Transaction, error) {
	log.Printf("[repo.transaction] Create: user_id=%d type=%s title=%q date=%q", userID, txnType, title, date)

	var t models.Transaction
	var idempKey sql.NullString
	err := r.db.QueryRow(ctx,
		`INSERT INTO transactions_v2 (user_id, type, title, date, note, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, type, title, transaction_cost, to_char(date, 'YYYY-MM-DD'), note, idempotency_key, created_at, updated_at`,
		userID, txnType, title, date, note, idempotencyKey,
	).Scan(&t.ID, &t.UserID, &t.Type, &t.Title, &t.TrxCost, &t.Date, &t.Note, &idempKey, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		log.Printf("[repo.transaction] Create: FAILED user_id=%d error=%v", userID, err)
		return nil, err
	}

	if idempKey.Valid {
		t.IdempotencyKey = &idempKey.String
	}

	log.Printf("[repo.transaction] Create: OK transaction_id=%d user_id=%d type=%s", t.ID, t.UserID, t.Type)
	return &t, nil
}

// GetByID retrieves a single transaction by ID with enriched details
func (r *TransactionRepository) GetByID(ctx context.Context, transactionID, userID int) (*models.Transaction, error) {
	log.Printf("[repo.transaction] GetByID: transaction_id=%d user_id=%d", transactionID, userID)

	query := enrichedTransactionSelect + `
		WHERE t.id = $1 AND t.user_id = $2
		GROUP BY t.id`
	row := r.db.QueryRow(ctx, query, transactionID, userID)
	t, err := scanEnrichedTransaction(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("[repo.transaction] GetByID: not found transaction_id=%d user_id=%d", transactionID, userID)
			return nil, nil
		}
		log.Printf("[repo.transaction] GetByID: FAILED transaction_id=%d user_id=%d error=%v", transactionID, userID, err)
		return nil, err
	}

	log.Printf("[repo.transaction] GetByID: OK transaction_id=%d type=%s amount=%.2f", t.ID, t.Type, t.Amount)
	return t, nil
}

// GetByIdempotencyKey retrieves a transaction by its idempotency key
func (r *TransactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Transaction, error) {
	log.Printf("[repo.transaction] GetByIdempotencyKey: key=%q", key)

	var t models.Transaction
	var idempKey sql.NullString
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, type, title, transaction_cost, to_char(date, 'YYYY-MM-DD'), note, idempotency_key, created_at, updated_at
		 FROM transactions_v2
		 WHERE idempotency_key = $1`,
		key,
	).Scan(&t.ID, &t.UserID, &t.Type, &t.Title, &t.TrxCost, &t.Date, &t.Note, &idempKey, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("[repo.transaction] GetByIdempotencyKey: not found key=%q", key)
			return nil, nil
		}
		log.Printf("[repo.transaction] GetByIdempotencyKey: FAILED key=%q error=%v", key, err)
		return nil, err
	}

	if idempKey.Valid {
		t.IdempotencyKey = &idempKey.String
	}

	log.Printf("[repo.transaction] GetByIdempotencyKey: OK transaction_id=%d key=%q", t.ID, key)
	return &t, nil
}

// Update updates transaction metadata (title, date, note)
func (r *TransactionRepository) Update(ctx context.Context, transactionID, userID int, title, date, note string) (*models.Transaction, error) {
	log.Printf("[repo.transaction] Update: transaction_id=%d user_id=%d title=%q date=%q", transactionID, userID, title, date)

	tag, err := r.db.Exec(ctx,
		`UPDATE transactions_v2
		 SET title = $1, date = $2, note = $3, updated_at = now()
		 WHERE id = $4 AND user_id = $5`,
		title, date, note, transactionID, userID,
	)
	if err != nil {
		log.Printf("[repo.transaction] Update: FAILED transaction_id=%d error=%v", transactionID, err)
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		log.Printf("[repo.transaction] Update: not found transaction_id=%d user_id=%d", transactionID, userID)
		return nil, nil
	}

	log.Printf("[repo.transaction] Update: OK transaction_id=%d", transactionID)
	return r.GetByID(ctx, transactionID, userID)
}

// Delete removes a transaction and all associated ledger entries
func (r *TransactionRepository) Delete(ctx context.Context, transactionID, userID int) (bool, error) {
	log.Printf("[repo.transaction] Delete: transaction_id=%d user_id=%d", transactionID, userID)

	tag, err := r.db.Exec(ctx,
		`DELETE FROM transactions_v2 WHERE id = $1 AND user_id = $2`,
		transactionID, userID)
	if err != nil {
		log.Printf("[repo.transaction] Delete: FAILED transaction_id=%d error=%v", transactionID, err)
		return false, err
	}

	deleted := tag.RowsAffected() > 0
	log.Printf("[repo.transaction] Delete: transaction_id=%d user_id=%d deleted=%v", transactionID, userID, deleted)
	return deleted, nil
}

// AddCategory associates a category with a transaction
func (r *TransactionRepository) AddCategory(ctx context.Context, transactionID, categoryID int) error {
	log.Printf("[repo.transaction] AddCategory: transaction_id=%d category_id=%d", transactionID, categoryID)

	_, err := r.db.Exec(ctx,
		`INSERT INTO transaction_categories (transaction_id, category_id)
		 VALUES ($1, $2)
		 ON CONFLICT (transaction_id, category_id) DO NOTHING`,
		transactionID, categoryID,
	)
	if err != nil {
		log.Printf("[repo.transaction] AddCategory: FAILED transaction_id=%d category_id=%d error=%v", transactionID, categoryID, err)
		return err
	}

	log.Printf("[repo.transaction] AddCategory: OK transaction_id=%d category_id=%d", transactionID, categoryID)
	return nil
}

// GetCategories retrieves all categories for a transaction
func (r *TransactionRepository) GetCategories(ctx context.Context, transactionID int) ([]models.Category, error) {
	log.Printf("[repo.transaction] GetCategories: transaction_id=%d", transactionID)

	rows, err := r.db.Query(ctx,
		`SELECT c.id, c.user_id, c.name, c.type, c.color, c.created_at
		 FROM categories c
		 INNER JOIN transaction_categories tc ON tc.category_id = c.id
		 WHERE tc.transaction_id = $1`,
		transactionID,
	)
	if err != nil {
		log.Printf("[repo.transaction] GetCategories: query failed transaction_id=%d error=%v", transactionID, err)
		return nil, err
	}
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var cat models.Category
		var userID sql.NullInt64
		if err := rows.Scan(&cat.ID, &userID, &cat.Name, &cat.Type, &cat.Color, &cat.CreatedAt); err != nil {
			return nil, err
		}
		if userID.Valid {
			cat.UserID = &[]int{int(userID.Int64)}[0]
		}
		categories = append(categories, cat)
	}

	log.Printf("[repo.transaction] GetCategories: OK transaction_id=%d count=%d", transactionID, len(categories))
	return categories, rows.Err()
}

// RemoveCategory unlinks a category from a transaction
func (r *TransactionRepository) RemoveCategory(ctx context.Context, transactionID, categoryID int) error {
	log.Printf("[repo.transaction] RemoveCategory: transaction_id=%d category_id=%d", transactionID, categoryID)

	_, err := r.db.Exec(ctx,
		`DELETE FROM transaction_categories WHERE transaction_id = $1 AND category_id = $2`,
		transactionID, categoryID,
	)
	if err != nil {
		log.Printf("[repo.transaction] RemoveCategory: FAILED transaction_id=%d error=%v", transactionID, err)
		return err
	}

	log.Printf("[repo.transaction] RemoveCategory: OK transaction_id=%d category_id=%d", transactionID, categoryID)
	return nil
}

// SetCategory replaces all categories for a transaction with a single category
func (r *TransactionRepository) SetCategory(ctx context.Context, transactionID, categoryID int) error {
	log.Printf("[repo.transaction] SetCategory: transaction_id=%d category_id=%d", transactionID, categoryID)

	_, err := r.db.Exec(ctx, `DELETE FROM transaction_categories WHERE transaction_id = $1`, transactionID)
	if err != nil {
		log.Printf("[repo.transaction] SetCategory: delete failed transaction_id=%d error=%v", transactionID, err)
		return err
	}

	err = r.AddCategory(ctx, transactionID, categoryID)
	if err != nil {
		log.Printf("[repo.transaction] SetCategory: add failed transaction_id=%d error=%v", transactionID, err)
		return err
	}

	log.Printf("[repo.transaction] SetCategory: OK transaction_id=%d category_id=%d", transactionID, categoryID)
	return nil
}
