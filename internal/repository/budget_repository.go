package repository

import (
	"context"
	"log"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BudgetRepository struct {
	db *pgxpool.Pool
}

func NewBudgetRepository(db *pgxpool.Pool) *BudgetRepository {
	log.Println("[repo.budget] NewBudgetRepository: created")
	return &BudgetRepository{db: db}
}

const budgetSelectWithSpent = `
	SELECT b.id, b.user_id, b.category_id, c.name, b.name, b.amount, b.color, b.created_at,
		COALESCE(SUM(exp.amount), 0) AS spent
	FROM budgets b
	JOIN categories c ON c.id = b.category_id
	LEFT JOIN (
		SELECT tc.category_id, t.user_id, ABS(le.amount) AS amount
		FROM transactions_v2 t
		JOIN transaction_categories tc ON tc.transaction_id = t.id
		JOIN ledger_entries le ON le.transaction_id = t.id AND le.amount < 0
		WHERE t.type = 'expense'
		UNION ALL
		SELECT category_id, user_id, amount
		FROM transactions
		WHERE type = 'expense'
	) exp ON exp.category_id = b.category_id AND exp.user_id = b.user_id
	WHERE b.user_id = $1
	GROUP BY b.id, b.user_id, b.category_id, c.name, b.name, b.amount, b.color, b.created_at
	ORDER BY b.created_at DESC`

func (r *BudgetRepository) ListByUser(ctx context.Context, userID int) ([]models.Budget, error) {
	log.Printf("[repo.budget] ListByUser: user_id=%d", userID)

	rows, err := r.db.Query(ctx, budgetSelectWithSpent, userID)
	if err != nil {
		log.Printf("[repo.budget] ListByUser: query failed user_id=%d error=%v", userID, err)
		return nil, err
	}
	defer rows.Close()

	budgets := []models.Budget{}
	for rows.Next() {
		var b models.Budget
		if err := rows.Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Category, &b.Name, &b.Amount, &b.Color, &b.CreatedAt, &b.Spent); err != nil {
			log.Printf("[repo.budget] ListByUser: scan failed user_id=%d error=%v", userID, err)
			return nil, err
		}
		budgets = append(budgets, b)
	}

	log.Printf("[repo.budget] ListByUser: OK user_id=%d count=%d", userID, len(budgets))
	return budgets, rows.Err()
}

func (r *BudgetRepository) Create(ctx context.Context, userID, categoryID int, name string, amount float64, color string) (*models.Budget, error) {
	log.Printf("[repo.budget] Create: user_id=%d category_id=%d name=%q amount=%.2f color=%q",
		userID, categoryID, name, amount, color)

	var b models.Budget
	err := r.db.QueryRow(ctx,
		`INSERT INTO budgets (user_id, category_id, name, amount, color)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, user_id, category_id, name, amount, color, created_at`,
		userID, categoryID, name, amount, color,
	).Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Name, &b.Amount, &b.Color, &b.CreatedAt)
	if err != nil {
		log.Printf("[repo.budget] Create: FAILED user_id=%d error=%v", userID, err)
		return nil, err
	}

	log.Printf("[repo.budget] Create: OK budget_id=%d name=%q amount=%.2f", b.ID, b.Name, b.Amount)
	return &b, nil
}

// Update returns pgx.ErrNoRows (unwrapped, caller maps it) if no budget
// with that id+userID exists
func (r *BudgetRepository) Update(ctx context.Context, id, userID, categoryID int, name string, amount float64, color string) (*models.Budget, error) {
	log.Printf("[repo.budget] Update: budget_id=%d user_id=%d category_id=%d name=%q amount=%.2f",
		id, userID, categoryID, name, amount)

	var b models.Budget
	err := r.db.QueryRow(ctx,
		`UPDATE budgets SET category_id = $1, name = $2, amount = $3, color = $4
		 WHERE id = $5 AND user_id = $6
		 RETURNING id, user_id, category_id, name, amount, color, created_at`,
		categoryID, name, amount, color, id, userID,
	).Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Name, &b.Amount, &b.Color, &b.CreatedAt)
	if err != nil {
		log.Printf("[repo.budget] Update: FAILED budget_id=%d user_id=%d error=%v", id, userID, err)
		return nil, err
	}

	log.Printf("[repo.budget] Update: OK budget_id=%d name=%q", b.ID, b.Name)
	return &b, nil
}

func (r *BudgetRepository) Delete(ctx context.Context, id, userID int) (bool, error) {
	log.Printf("[repo.budget] Delete: budget_id=%d user_id=%d", id, userID)

	tag, err := r.db.Exec(ctx, `DELETE FROM budgets WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		log.Printf("[repo.budget] Delete: FAILED budget_id=%d user_id=%d error=%v", id, userID, err)
		return false, err
	}

	deleted := tag.RowsAffected() > 0
	log.Printf("[repo.budget] Delete: budget_id=%d user_id=%d deleted=%v", id, userID, deleted)
	return deleted, nil
}
