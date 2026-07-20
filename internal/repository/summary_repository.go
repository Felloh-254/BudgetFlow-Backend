package repository

import (
	"context"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SummaryRepository struct {
	db *pgxpool.Pool
}

func NewSummaryRepository(db *pgxpool.Pool) *SummaryRepository {
	return &SummaryRepository{db: db}
}

func (r *SummaryRepository) Totals(ctx context.Context, userID int) (income, expense float64, err error) {
	err = r.db.QueryRow(ctx,
		`SELECT
			COALESCE(SUM(amount) FILTER (WHERE type = 'income'), 0),
			COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0)
		 FROM transactions WHERE user_id = $1`,
		userID,
	).Scan(&income, &expense)
	return income, expense, err
}

func (r *SummaryRepository) BudgetStats(ctx context.Context, userID int) ([]models.BudgetStat, error) {
	rows, err := r.db.Query(ctx,
		`SELECT b.name, b.amount, COALESCE(SUM(t.amount) FILTER (WHERE t.type = 'expense'), 0), b.color, c.name
		 FROM budgets b
		 JOIN categories c ON c.id = b.category_id
		 LEFT JOIN transactions t ON t.budget_id = b.id
		 WHERE b.user_id = $1
		 GROUP BY b.id, c.name
		 ORDER BY b.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := []models.BudgetStat{}
	for rows.Next() {
		var s models.BudgetStat
		if err := rows.Scan(&s.Name, &s.Amount, &s.Spent, &s.Color, &s.Category); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

func (r *SummaryRepository) MonthlyData(ctx context.Context, userID, months int) ([]models.MonthlyDataPoint, error) {
	rows, err := r.db.Query(ctx,
		`SELECT to_char(date_trunc('month', date), 'YYYY-MM') AS month,
			COALESCE(SUM(amount) FILTER (WHERE type = 'income'), 0),
			COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0)
		 FROM transactions
		 WHERE user_id = $1
		 GROUP BY 1
		 ORDER BY 1 DESC
		 LIMIT $2`,
		userID, months,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []models.MonthlyDataPoint{}
	for rows.Next() {
		var m models.MonthlyDataPoint
		if err := rows.Scan(&m.Month, &m.Income, &m.Expense); err != nil {
			return nil, err
		}
		points = append(points, m)
	}
	return points, rows.Err()
}
