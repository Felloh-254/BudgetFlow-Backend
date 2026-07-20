package models

type Summary struct {
	TotalIncome   float64            `json:"total_income"`
	TotalExpenses float64            `json:"total_expenses"`
	Balance       float64            `json:"balance"`
	BudgetStats   []BudgetStat       `json:"budget_stats"`
	MonthlyData   []MonthlyDataPoint `json:"monthly_data"`
}

type BudgetStat struct {
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	Spent    float64 `json:"spent"`
	Color    string  `json:"color"`
	Category string  `json:"category"`
}

type MonthlyDataPoint struct {
	Month   string  `json:"month"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
}
