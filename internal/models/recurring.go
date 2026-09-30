package models

import (
	"log"
	"time"
)

type RecurringRule struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	Type          string    `json:"type"` // income | expense | transfer
	Title         string    `json:"title"`
	Amount        float64   `json:"amount"`
	Note          string    `json:"note"`
	CategoryID    *int      `json:"category_id,omitempty"`
	AccountID     *int      `json:"account_id,omitempty"`
	FromAccountID *int      `json:"from_account_id,omitempty"`
	ToAccountID   *int      `json:"to_account_id,omitempty"`
	Frequency     string    `json:"frequency"` // daily | weekly | monthly
	IntervalCount int       `json:"interval_count"`
	StartDate     string    `json:"start_date"`
	EndDate       *string   `json:"end_date,omitempty"`
	NextRunAt     string    `json:"next_run_at"`
	LastRunAt     *string   `json:"last_run_at,omitempty"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RecurringRuleInput struct {
	Type          string  `json:"type"`
	Title         string  `json:"title"`
	Amount        float64 `json:"amount"`
	Note          string  `json:"note,omitempty"`
	Category      string  `json:"category,omitempty"`
	AccountID     int     `json:"account_id,omitempty"`
	FromAccountID int     `json:"from_account_id,omitempty"`
	ToAccountID   int     `json:"to_account_id,omitempty"`
	Frequency     string  `json:"frequency"`
	IntervalCount int     `json:"interval_count,omitempty"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date,omitempty"`
}

func init() {
	log.Println("[models.recurring] Recurring models loaded")
}
