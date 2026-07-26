package models

import "time"

// Category normalizes what used to be a free-text field on Budget and
// Transaction. UserID is nil for global/default categories shared by all
// users, and set for a user's own custom category.
type Category struct {
	ID        int       `json:"id"`
	UserID    *int      `json:"user_id,omitempty"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // "expense" or "income"
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}
