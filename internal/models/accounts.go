package models

import "time"

type Account struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	AccountNumber *string   `json:"account_number"`
	Balance       float64   `json:"balance"` // sourced from account_balances table
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Currency      string    `json:"currency"`
}

type AccountInput struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	AccountNumber *string `json:"account_number"`
	Balance       float64 `json:"balance"` // initial balance only; used when creating account
	Currency      string  `json:"currency"`
}

// AccountBalance represents the denormalized balance state
type AccountBalance struct {
	ID               int       `json:"id"`
	AccountID        int       `json:"account_id"`
	Balance          float64   `json:"balance"`
	LastUpdatedTxnID *int      `json:"last_updated_txn_id,omitempty"`
	Version          int       `json:"version"` // for optimistic locking
	UpdatedAt        time.Time `json:"updated_at"`
}
