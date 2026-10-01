package models

import (
	"time"
)

type Budget struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	CategoryID int       `json:"category_id"`
	Category   string    `json:"category"`
	Name       string    `json:"name"`
	Amount     float64   `json:"amount"`
	Color      string    `json:"color"`
	Spent      float64   `json:"spent"`
	Month      string    `json:"month"` // YYYY-MM
	CreatedAt  time.Time `json:"created_at"`
}

type BudgetInput struct {
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	Category string  `json:"category"`
	Color    string  `json:"color"`
	Month    string  `json:"month,omitempty"` // optional; defaults to current month
}
