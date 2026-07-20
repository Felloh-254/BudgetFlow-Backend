package models

import "time"

type Budget struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	CategoryID int       `json:"category_id"`
	Category   string    `json:"category"` // denormalized name, for API convenience
	Name       string    `json:"name"`
	Amount     float64   `json:"amount"`
	Color      string    `json:"color"`
	Spent      float64   `json:"spent"`
	CreatedAt  time.Time `json:"created_at"`
}

// BudgetInput is what create/update requests bind into. Keeping request
// shapes separate from the persisted model means clients can never set
// fields like ID, UserID, or Spent directly.
type BudgetInput struct {
	Name     string  `json:"name"`
	Amount   float64 `json:"amount"`
	Category string  `json:"category"`
	Color    string  `json:"color"`
}
