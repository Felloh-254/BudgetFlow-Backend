package models

type User struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	Password  string `json:"-"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type Budget struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	Name      string  `json:"name"`
	Amount    float64 `json:"amount"`
	Category  string  `json:"category"`
	Color     string  `json:"color"`
	Spent     float64 `json:"spent"`
	CreatedAt string  `json:"created_at"`
}

type Transaction struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	BudgetID  *int    `json:"budget_id"`
	Title     string  `json:"title"`
	Amount    float64 `json:"amount"`
	Type      string  `json:"type"`
	Category  string  `json:"category"`
	Date      string  `json:"date"`
	Note      string  `json:"note"`
	CreatedAt string  `json:"created_at"`
}

type Summary struct {
	TotalIncome   float64          `json:"total_income"`
	TotalExpenses float64          `json:"total_expenses"`
	Balance       float64          `json:"balance"`
	BudgetStats   []BudgetStat     `json:"budget_stats"`
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
