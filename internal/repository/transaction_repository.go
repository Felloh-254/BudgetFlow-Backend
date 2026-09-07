package repository

import (
	"context"
	"database/sql"

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

// ListByUser returns all transactions for a user with pagination
func (r *TransactionRepository) ListByUser(ctx context.Context, userID, limit, offset int) ([]models.Transaction, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, type, title, to_char(date, 'YYYY-MM-DD') as date, note, idempotency_key, created_at, updated_at
		 FROM transactions_v2
		 WHERE user_id = $1
		 ORDER BY date DESC, created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	txns := []models.Transaction{}
	for rows.Next() {
		var t models.Transaction
		var idempKey sql.NullString
		if err := rows.Scan(&t.ID, &t.UserID, &t.Type, &t.Title, &t.Date, &t.Note, &idempKey, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		if idempKey.Valid {
			t.IdempotencyKey = &idempKey.String
		}
		txns = append(txns, t)
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

// GetByID retrieves a single transaction by ID
func (r *TransactionRepository) GetByID(ctx context.Context, transactionID, userID int) (*models.Transaction, error) {
	var t models.Transaction
	var idempKey sql.NullString
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, type, title, to_char(date, 'YYYY-MM-DD'), note, idempotency_key, created_at, updated_at
		 FROM transactions_v2
		 WHERE id = $1 AND user_id = $2`,
		transactionID, userID,
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

// Update updates transaction metadata (but not ledger entries)
func (r *TransactionRepository) Update(ctx context.Context, transactionID, userID int, title, date, note string) (*models.Transaction, error) {
	var t models.Transaction
	var idempKey sql.NullString
	err := r.db.QueryRow(ctx,
		`UPDATE transactions_v2
		 SET title = $1, date = $2, note = $3, updated_at = now()
		 WHERE id = $4 AND user_id = $5
		 RETURNING id, user_id, type, title, to_char(date, 'YYYY-MM-DD'), note, idempotency_key, created_at, updated_at`,
		title, date, note, transactionID, userID,
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
