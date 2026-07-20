package repository

import (
	"context"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BudgetRepository struct {
	db *pgxpool.Pool
}

func NewBudgetRepository(db *pgxpool.Pool) *BudgetRepository {
	return &BudgetRepository{db: db}
}

const budgetSelectWithSpent = `
	SELECT b.id, b.user_id, b.category_id, c.name, b.name, b.amount, b.color, b.created_at,
		COALESCE(SUM(t.amount) FILTER (WHERE t.type = 'expense'), 0) AS spent
	FROM budgets b
	JOIN categories c ON c.id = b.category_id
	LEFT JOIN transactions t ON t.budget_id = b.id
	WHERE b.user_id = $1
	GROUP BY b.id, c.name
	ORDER BY b.created_at DESC`

func (r *BudgetRepository) ListByUser(ctx context.Context, userID int) ([]models.Budget, error) {
	rows, err := r.db.Query(ctx, budgetSelectWithSpent, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	budgets := []models.Budget{}
	for rows.Next() {
		var b models.Budget
		if err := rows.Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Category, &b.Name, &b.Amount, &b.Color, &b.CreatedAt, &b.Spent); err != nil {
			return nil, err
		}
		budgets = append(budgets, b)
	}
	return budgets, rows.Err()
}

func (r *BudgetRepository) Create(ctx context.Context, userID, categoryID int, name string, amount float64, color string) (*models.Budget, error) {
	var b models.Budget
	err := r.db.QueryRow(ctx,
		`INSERT INTO budgets (user_id, category_id, name, amount, color)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, user_id, category_id, name, amount, color, created_at`,
		userID, categoryID, name, amount, color,
	).Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Name, &b.Amount, &b.Color, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// Update returns pgx.ErrNoRows (unwrapped, caller maps it) if no budget
// with that id+userID exists — ownership check and existence check in one
// query instead of a separate SELECT-then-UPDATE round trip.
func (r *BudgetRepository) Update(ctx context.Context, id, userID, categoryID int, name string, amount float64, color string) (*models.Budget, error) {
	var b models.Budget
	err := r.db.QueryRow(ctx,
		`UPDATE budgets SET category_id = $1, name = $2, amount = $3, color = $4
		 WHERE id = $5 AND user_id = $6
		 RETURNING id, user_id, category_id, name, amount, color, created_at`,
		categoryID, name, amount, color, id, userID,
	).Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Name, &b.Amount, &b.Color, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *BudgetRepository) Delete(ctx context.Context, id, userID int) (bool, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM budgets WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
