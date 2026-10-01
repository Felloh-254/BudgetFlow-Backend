package models

import (
	"time"
)

type Goal struct {
	ID            int       `json:"id"`
	UserID        int       `json:"user_id"`
	Name          string    `json:"name"`
	TargetAmount  float64   `json:"target_amount"`
	CurrentAmount float64   `json:"current_amount"`
	TargetDate    *string   `json:"target_date,omitempty"`
	Color         string    `json:"color"`
	Status        string    `json:"status"`
	Progress      float64   `json:"progress"` // 0..100, computed
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type GoalInput struct {
	Name         string  `json:"name"`
	TargetAmount float64 `json:"target_amount"`
	TargetDate   string  `json:"target_date,omitempty"`
	Color        string  `json:"color,omitempty"`
}

type GoalContributeInput struct {
	Amount float64 `json:"amount"`
}
