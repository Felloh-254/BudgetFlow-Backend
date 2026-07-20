package repository

import (
	"context"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository struct {
	db *pgxpool.Pool
}

func NewTransactionRepository(db *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// date is cast with to_char(...) explicitly rather than left as a native
// DATE, so it always comes back as a plain "YYYY-MM-DD" string — matching
// what the frontend already expects and avoiding any ambiguity around how
// the driver maps DATE into a Go string.
const transactionSelect = `
	SELECT t.id, t.user_id, t.budget_id, t.category_id, c.name, t.title, t.amount, t.type,
		to_char(t.date, 'YYYY-MM-DD') AS date, t.note, t.created_at
	FROM transactions t
	JOIN categories c ON c.id = t.category_id`

func (r *TransactionRepository) ListByUser(ctx context.Context, userID, limit, offset int) ([]models.Transaction, error) {
	rows, err := r.db.Query(ctx,
		transactionSelect+` WHERE t.user_id = $1 ORDER BY t.date DESC, t.created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	txns := []models.Transaction{}
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(&t.ID, &t.UserID, &t.BudgetID, &t.CategoryID, &t.Category, &t.Title, &t.Amount, &t.Type, &t.Date, &t.Note, &t.CreatedAt); err != nil {
			return nil, err
		}
		txns = append(txns, t)
	}
	return txns, rows.Err()
}

func (r *TransactionRepository) Create(ctx context.Context, userID int, budgetID *int, categoryID int, title string, amount float64, txnType, date, note string) (*models.Transaction, error) {
	var t models.Transaction
	err := r.db.QueryRow(ctx,
		`INSERT INTO transactions (user_id, budget_id, category_id, title, amount, type, date, note)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, user_id, budget_id, category_id, title, amount, type, to_char(date, 'YYYY-MM-DD'), note, created_at`,
		userID, budgetID, categoryID, title, amount, txnType, date, note,
	).Scan(&t.ID, &t.UserID, &t.BudgetID, &t.CategoryID, &t.Title, &t.Amount, &t.Type, &t.Date, &t.Note, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TransactionRepository) Update(ctx context.Context, id, userID int, budgetID *int, categoryID int, title string, amount float64, txnType, date, note string) (*models.Transaction, error) {
	var t models.Transaction
	err := r.db.QueryRow(ctx,
		`UPDATE transactions
		 SET budget_id = $1, category_id = $2, title = $3, amount = $4, type = $5, date = $6, note = $7
		 WHERE id = $8 AND user_id = $9
		 RETURNING id, user_id, budget_id, category_id, title, amount, type, to_char(date, 'YYYY-MM-DD'), note, created_at`,
		budgetID, categoryID, title, amount, txnType, date, note, id, userID,
	).Scan(&t.ID, &t.UserID, &t.BudgetID, &t.CategoryID, &t.Title, &t.Amount, &t.Type, &t.Date, &t.Note, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TransactionRepository) Delete(ctx context.Context, id, userID int) (bool, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM transactions WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
