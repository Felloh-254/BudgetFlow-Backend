package models

import "time"

type Account struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	AccountNumber *string   `json:"account_number"`
	Balance       float64   `json:"balance"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Currency      string    `json:"currency"`
}

type AccountInput struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	AccountNumber *string `json:"account_number"`
	Balance       float64 `json:"balance"`
	Currency      string  `json:"currency"`
}
