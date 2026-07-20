package models

import "time"

type Transaction struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	BudgetID   *int      `json:"budget_id"`
	CategoryID int       `json:"category_id"`
	Category   string    `json:"category"`
	Title      string    `json:"title"`
	Amount     float64   `json:"amount"`
	Type       string    `json:"type"` // "income" | "expense"
	Date       string    `json:"date"` // YYYY-MM-DD
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

type TransactionInput struct {
	BudgetID *int    `json:"budget_id"`
	Title    string  `json:"title"`
	Amount   float64 `json:"amount"`
	Type     string  `json:"type"`
	Category string  `json:"category"`
	Date     string  `json:"date"`
	Note     string  `json:"note"`
}
