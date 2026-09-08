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
			COALESCE(SUM(income), 0),
			COALESCE(SUM(expense), 0)
		 FROM (
			SELECT
				CASE WHEN t.type = 'income' AND le.amount > 0 THEN le.amount ELSE 0 END AS income,
				CASE WHEN t.type = 'expense' AND le.amount < 0 THEN ABS(le.amount) ELSE 0 END AS expense
			FROM transactions_v2 t
			JOIN ledger_entries le ON le.transaction_id = t.id
			WHERE t.user_id = $1
			UNION ALL
			SELECT
				CASE WHEN type = 'income' THEN amount ELSE 0 END AS income,
				CASE WHEN type = 'expense' THEN amount ELSE 0 END AS expense
			FROM transactions
			WHERE user_id = $1
		 ) combined`,
		userID,
	).Scan(&income, &expense)
	return income, expense, err
}

func (r *SummaryRepository) BudgetStats(ctx context.Context, userID int) ([]models.BudgetStat, error) {
	rows, err := r.db.Query(ctx,
		`SELECT b.name, b.amount,
			COALESCE(SUM(exp.amount), 0) AS spent,
			b.color, c.name
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
		 GROUP BY b.id, b.name, b.amount, b.color, c.name
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
		`SELECT month,
			COALESCE(SUM(income), 0) AS income,
			COALESCE(SUM(expense), 0) AS expense
		 FROM (
			SELECT to_char(date_trunc('month', t.date), 'YYYY-MM') AS month,
				CASE WHEN t.type = 'income' AND le.amount > 0 THEN le.amount ELSE 0 END AS income,
				CASE WHEN t.type = 'expense' AND le.amount < 0 THEN ABS(le.amount) ELSE 0 END AS expense
			FROM transactions_v2 t
			JOIN ledger_entries le ON le.transaction_id = t.id
			WHERE t.user_id = $1 AND ((t.type = 'income' AND le.amount > 0) OR (t.type = 'expense' AND le.amount < 0))
			UNION ALL
			SELECT to_char(date_trunc('month', date), 'YYYY-MM') AS month,
				CASE WHEN type = 'income' THEN amount ELSE 0 END AS income,
				CASE WHEN type = 'expense' THEN amount ELSE 0 END AS expense
			FROM transactions
			WHERE user_id = $1
		 ) combined
		 GROUP BY month
		 ORDER BY month DESC
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
