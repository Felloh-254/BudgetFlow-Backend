package models

import "time"

// Transaction represents the financial event itself (income, expense, transfer)
type Transaction struct {
	ID             int       `json:"id"`
	UserID         int       `json:"user_id"`
	Type           string    `json:"type"` // "income" | "expense" | "transfer"
	Title          string    `json:"title"`
	Date           string    `json:"date"` // YYYY-MM-DD
	Note           string    `json:"note"`
	IdempotencyKey *string   `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// LedgerEntry represents a single account movement
type LedgerEntry struct {
	ID            int       `json:"id"`
	TransactionID int       `json:"transaction_id"`
	AccountID     int       `json:"account_id"`
	Amount        float64   `json:"amount"`     // signed: positive for debit, negative for credit
	EntryType     string    `json:"entry_type"` // "debit" | "credit"
	CreatedAt     time.Time `json:"created_at"`
}

// TransactionDetail is the rich API response combining transaction, entries, and categories
type TransactionDetail struct {
	Transaction  Transaction    `json:"transaction"`
	Entries      []LedgerEntry  `json:"entries"`
	Categories   []Category     `json:"categories,omitempty"`
	AccountNames map[int]string `json:"account_names,omitempty"` // account_id -> account name for display
}

// TransactionInput for creating income/expense
type TransactionInput struct {
	Title          string  `json:"title"`
	Amount         float64 `json:"amount"`
	Type           string  `json:"type"` // "income" | "expense"
	Category       string  `json:"category"`
	AccountID      int     `json:"account_id"`
	Date           string  `json:"date"`
	Note           string  `json:"note"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}

// TransferInput for creating transfers between accounts
type TransferInput struct {
	Title          string  `json:"title"`
	Amount         float64 `json:"amount"`
	FromAccountID  int     `json:"from_account_id"`
	ToAccountID    int     `json:"to_account_id"`
	Date           string  `json:"date"`
	Note           string  `json:"note"`
	IdempotencyKey *string `json:"idempotency_key,omitempty"`
}
