package repository

import (
	"context"
	"log"
	"time"

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
	SELECT b.id, b.user_id, b.category_id, c.name, b.name, b.amount, b.color,
	       b.month, b.created_at,
	       COALESCE(SUM(exp.amount), 0) AS spent
	FROM budgets b
	JOIN categories c ON c.id = b.category_id
	LEFT JOIN (
		SELECT tc.category_id, t.user_id,
		       to_char(t.date, 'YYYY-MM') AS txn_month,
		       ABS(le.amount) AS amount
		FROM transactions_v2 t
		JOIN transaction_categories tc ON tc.transaction_id = t.id
		JOIN ledger_entries le ON le.transaction_id = t.id AND le.amount < 0
		WHERE t.type = 'expense'
		UNION ALL
		SELECT category_id, user_id,
		       to_char(date, 'YYYY-MM') AS txn_month,
		       amount
		FROM transactions
		WHERE type = 'expense'
	) exp ON exp.category_id = b.category_id
	     AND exp.user_id = b.user_id
	     AND exp.txn_month = b.month
	WHERE b.user_id = $1
	GROUP BY b.id, b.user_id, b.category_id, c.name, b.name, b.amount, b.color, b.month, b.created_at
	ORDER BY b.month DESC, b.created_at DESC`

const budgetSelectWithSpentMonth = `
	SELECT b.id, b.user_id, b.category_id, c.name, b.name, b.amount, b.color,
	       b.month, b.created_at,
	       COALESCE(SUM(exp.amount), 0) AS spent
	FROM budgets b
	JOIN categories c ON c.id = b.category_id
	LEFT JOIN (
		SELECT tc.category_id, t.user_id,
		       to_char(t.date, 'YYYY-MM') AS txn_month,
		       ABS(le.amount) AS amount
		FROM transactions_v2 t
		JOIN transaction_categories tc ON tc.transaction_id = t.id
		JOIN ledger_entries le ON le.transaction_id = t.id AND le.amount < 0
		WHERE t.type = 'expense'
		UNION ALL
		SELECT category_id, user_id,
		       to_char(date, 'YYYY-MM') AS txn_month,
		       amount
		FROM transactions
		WHERE type = 'expense'
	) exp ON exp.category_id = b.category_id
	     AND exp.user_id = b.user_id
	     AND exp.txn_month = b.month
	WHERE b.user_id = $1 AND b.month = $2
	GROUP BY b.id, b.user_id, b.category_id, c.name, b.name, b.amount, b.color, b.month, b.created_at
	ORDER BY b.created_at DESC`

func (r *BudgetRepository) ListByUser(ctx context.Context, userID int, month string) ([]models.Budget, error) {
	log.Printf("[repo.budget] ListByUser: user_id=%d month=%q", userID, month)

	var rows interface {
		Next() bool
		Scan(dest ...any) error
		Err() error
		Close()
	}
	var err error

	if month == "" {
		rows, err = r.db.Query(ctx, budgetSelectWithSpent, userID)
	} else {
		rows, err = r.db.Query(ctx, budgetSelectWithSpentMonth, userID, month)
	}
	if err != nil {
		log.Printf("[repo.budget] ListByUser: query failed user_id=%d month=%q error=%v", userID, month, err)
		return nil, err
	}
	defer rows.Close()

	budgets := []models.Budget{}
	for rows.Next() {
		var b models.Budget
		if err := rows.Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Category, &b.Name,
			&b.Amount, &b.Color, &b.Month, &b.CreatedAt, &b.Spent); err != nil {
			log.Printf("[repo.budget] ListByUser: scan failed user_id=%d month=%q error=%v", userID, month, err)
			return nil, err
		}
		budgets = append(budgets, b)
	}

	log.Printf("[repo.budget] ListByUser: OK user_id=%d month=%q count=%d", userID, month, len(budgets))
	return budgets, rows.Err()
}

func (r *BudgetRepository) Create(ctx context.Context, userID, categoryID int, name string,
	amount float64, color, month string) (*models.Budget, error) {

	if month == "" {
		month = time.Now().Format("2006-01")
	}
	log.Printf("[repo.budget] Create: user_id=%d category_id=%d name=%q amount=%.2f month=%q",
		userID, categoryID, name, amount, month)

	var b models.Budget
	err := r.db.QueryRow(ctx,
		`INSERT INTO budgets (user_id, category_id, name, amount, color, month)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, category_id, name, amount, color, month, created_at`,
		userID, categoryID, name, amount, color, month,
	).Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Name, &b.Amount, &b.Color, &b.Month, &b.CreatedAt)
	if err != nil {
		log.Printf("[repo.budget] Create: FAILED user_id=%d error=%v", userID, err)
		return nil, err
	}
	log.Printf("[repo.budget] Create: OK budget_id=%d month=%q", b.ID, b.Month)
	return &b, nil
}

func (r *BudgetRepository) Update(ctx context.Context, id, userID, categoryID int,
	name string, amount float64, color, month string) (*models.Budget, error) {
	log.Printf("[repo.budget] Update: budget_id=%d user_id=%d month=%q", id, userID, month)

	var b models.Budget
	err := r.db.QueryRow(ctx,
		`UPDATE budgets
		 SET category_id = $1, name = $2, amount = $3, color = $4, month = $5
		 WHERE id = $6 AND user_id = $7
		 RETURNING id, user_id, category_id, name, amount, color, month, created_at`,
		categoryID, name, amount, color, month, id, userID,
	).Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Name, &b.Amount, &b.Color, &b.Month, &b.CreatedAt)
	if err != nil {
		log.Printf("[repo.budget] Update: FAILED budget_id=%d error=%v", id, err)
		return nil, err
	}
	log.Printf("[repo.budget] Update: OK budget_id=%d month=%q", b.ID, b.Month)
	return &b, nil
}

func (r *BudgetRepository) Delete(ctx context.Context, id, userID int) (bool, error) {
	log.Printf("[repo.budget] Delete: budget_id=%d user_id=%d", id, userID)
	tag, err := r.db.Exec(ctx, `DELETE FROM budgets WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		log.Printf("[repo.budget] Delete: FAILED budget_id=%d error=%v", id, err)
		return false, err
	}
	deleted := tag.RowsAffected() > 0
	log.Printf("[repo.budget] Delete: budget_id=%d deleted=%v", id, deleted)
	return deleted, nil
}

func (r *BudgetRepository) CopyBudgetsFromMonth(ctx context.Context, userID int, fromMonth, toMonth string) (int, error) {
	log.Printf("[repo.budget] CopyBudgetsFromMonth: user_id=%d from=%q to=%q", userID, fromMonth, toMonth)

	tag, err := r.db.Exec(ctx,
		`INSERT INTO budgets (user_id, category_id, name, amount, color, month)
		 SELECT user_id, category_id, name, amount, color, $1
		 FROM budgets
		 WHERE user_id = $2 AND month = $3
		 ON CONFLICT (user_id, category_id, month) DO NOTHING`,
		toMonth, userID, fromMonth,
	)
	if err != nil {
		log.Printf("[repo.budget] CopyBudgetsFromMonth: FAILED user_id=%d error=%v", userID, err)
		return 0, err
	}
	copied := int(tag.RowsAffected())
	log.Printf("[repo.budget] CopyBudgetsFromMonth: OK user_id=%d from=%q to=%q copied=%d",
		userID, fromMonth, toMonth, copied)
	return copied, nil
}
