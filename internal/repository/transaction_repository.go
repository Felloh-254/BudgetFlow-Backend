package repository

import (
	"context"
	"database/sql"
	"fmt"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository struct {
	db *pgxpool.Pool
}

func NewTransactionRepository(db *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{db: db}
}

const enrichedTransactionSelect = `
	SELECT 
		t.id, 
		t.user_id, 
		t.type, 
		t.title, 
		to_char(t.date, 'YYYY-MM-DD') as date, 
		t.note, 
		t.idempotency_key, 
		t.created_at, 
		t.updated_at,
		COALESCE(
			MAX(CASE WHEN t.type = 'transfer' AND le.amount > 0 THEN le.amount
			         WHEN t.type != 'transfer' THEN ABS(le.amount)
			    END), 0
		) as amount,
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
		&t.ID, &t.UserID, &t.Type, &t.Title, &t.Date, &t.Note, &idempKey, &t.CreatedAt, &t.UpdatedAt,
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
		return nil, err
	}
	defer rows.Close()

	txns := []models.Transaction{}
	for rows.Next() {
		t, err := scanEnrichedTransaction(rows)
		if err != nil {
			return nil, err
		}
		txns = append(txns, *t)
	}
	return txns, rows.Err()
}

// Create creates a transaction event (without ledger entries)
// Ledger entries must be created separately via LedgerRepository
func (r *TransactionRepository) Create(ctx context.Context, userID int, txnType, title, date, note string, idempotencyKey *string) (*models.Transaction, error) {
	var t models.Transaction
	var idempKey sql.NullString
	err := r.db.QueryRow(ctx,
		`INSERT INTO transactions_v2 (user_id, type, title, date, note, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, type, title, to_char(date, 'YYYY-MM-DD'), note, idempotency_key, created_at, updated_at`,
		userID, txnType, title, date, note, idempotencyKey,
	).Scan(&t.ID, &t.UserID, &t.Type, &t.Title, &t.Date, &t.Note, &idempKey, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if idempKey.Valid {
		t.IdempotencyKey = &idempKey.String
	}
	return &t, nil
}

// GetByID retrieves a single transaction by ID with enriched details
func (r *TransactionRepository) GetByID(ctx context.Context, transactionID, userID int) (*models.Transaction, error) {
	query := enrichedTransactionSelect + `
		WHERE t.id = $1 AND t.user_id = $2
		GROUP BY t.id`
	row := r.db.QueryRow(ctx, query, transactionID, userID)
	t, err := scanEnrichedTransaction(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

// GetByIdempotencyKey retrieves a transaction by its idempotency key (for deduplication)
func (r *TransactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Transaction, error) {
	var t models.Transaction
	var idempKey sql.NullString
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, type, title, to_char(date, 'YYYY-MM-DD'), note, idempotency_key, created_at, updated_at
		 FROM transactions_v2
		 WHERE idempotency_key = $1`,
		key,
	).Scan(&t.ID, &t.UserID, &t.Type, &t.Title, &t.Date, &t.Note, &idempKey, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if idempKey.Valid {
		t.IdempotencyKey = &idempKey.String
	}
	return &t, nil
}

// Update updates transaction metadata (title, date, note)
func (r *TransactionRepository) Update(ctx context.Context, transactionID, userID int, title, date, note string) (*models.Transaction, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE transactions_v2
		 SET title = $1, date = $2, note = $3, updated_at = now()
		 WHERE id = $4 AND user_id = $5`,
		title, date, note, transactionID, userID,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, nil
	}
	return r.GetByID(ctx, transactionID, userID)
}

// Delete removes a transaction and all associated ledger entries
func (r *TransactionRepository) Delete(ctx context.Context, transactionID, userID int) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM transactions_v2 WHERE id = $1 AND user_id = $2`,
		transactionID, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// AddCategory associates a category with a transaction
func (r *TransactionRepository) AddCategory(ctx context.Context, transactionID, categoryID int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO transaction_categories (transaction_id, category_id)
		 VALUES ($1, $2)
		 ON CONFLICT (transaction_id, category_id) DO NOTHING`,
		transactionID, categoryID,
	)
	return err
}

// GetCategories retrieves all categories for a transaction
func (r *TransactionRepository) GetCategories(ctx context.Context, transactionID int) ([]models.Category, error) {
	rows, err := r.db.Query(ctx,
		`SELECT c.id, c.user_id, c.name, c.type, c.color, c.created_at
		 FROM categories c
		 INNER JOIN transaction_categories tc ON tc.category_id = c.id
		 WHERE tc.transaction_id = $1`,
		transactionID,
	)
	if err != nil {
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
	return categories, rows.Err()
}

// RemoveCategory unlinks a category from a transaction
func (r *TransactionRepository) RemoveCategory(ctx context.Context, transactionID, categoryID int) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM transaction_categories WHERE transaction_id = $1 AND category_id = $2`,
		transactionID, categoryID,
	)
	return err
}

// SetCategory replaces all categories for a transaction with a single category
func (r *TransactionRepository) SetCategory(ctx context.Context, transactionID, categoryID int) error {
	_, err := r.db.Exec(ctx, `DELETE FROM transaction_categories WHERE transaction_id = $1`, transactionID)
	if err != nil {
		return err
	}
	return r.AddCategory(ctx, transactionID, categoryID)
}
