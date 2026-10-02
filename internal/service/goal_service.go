package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"budgetapp/internal/apperr"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type GoalService struct {
	goals *repository.GoalRepository
	log   *slog.Logger
}

func NewGoalService(goals *repository.GoalRepository, log *slog.Logger) *GoalService {
	return &GoalService{
		goals: goals,
		log:   log.With("component", "service.goal"),
	}
}

func (s *GoalService) List(ctx context.Context, userID int) ([]models.Goal, error) {
	goals, err := s.goals.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list goals (user=%d): %w", userID, err)
	}
	return goals, nil
}

func (s *GoalService) Create(ctx context.Context, userID int, in models.GoalInput) (*models.Goal, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, apperr.Validation("goal name is required")
	}
	if in.TargetAmount <= 0 {
		return nil, apperr.Validation("target_amount must be greater than 0")
	}
	if in.Color == "" {
		in.Color = "#10b981"
	}
	g, err := s.goals.Create(ctx, userID, in)
	if err != nil {
		return nil, fmt.Errorf("create goal (user=%d): %w", userID, err)
	}

	s.log.InfoContext(ctx, "goal created",
		"user_id", userID,
		"goal_id", g.ID,
		"name", g.Name,
		"target_amount", g.TargetAmount,
	)
	return g, nil
}

func (s *GoalService) Update(ctx context.Context, id, userID int, in models.GoalInput) (*models.Goal, error) {
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperr.Validation("goal name is required")
	}
	if in.TargetAmount <= 0 {
		return nil, apperr.Validation("target_amount must be greater than 0")
	}
	if in.Color == "" {
		in.Color = "#10b981"
	}
	g, err := s.goals.Update(ctx, id, userID, in)
	if err != nil {
		return nil, fmt.Errorf("update goal (id=%d): %w", id, err)
	}

	s.log.InfoContext(ctx, "goal updated",
		"user_id", userID,
		"goal_id", id,
	)
	return g, nil
}

func (s *GoalService) Contribute(ctx context.Context, id, userID int, amount float64) (*models.Goal, error) {
	if amount <= 0 {
		return nil, apperr.Validation("amount must be greater than 0")
	}
	g, err := s.goals.Contribute(ctx, id, userID, amount)
	if err != nil {
		return nil, fmt.Errorf("contribute to goal (id=%d, amount=%.2f): %w", id, amount, err)
	}

	s.log.InfoContext(ctx, "goal contributed",
		"user_id", userID,
		"goal_id", id,
		"amount", amount,
		"status", g.Status,
		"progress", g.Progress,
	)
	return g, nil
}

func (s *GoalService) Delete(ctx context.Context, id, userID int) error {
	ok, err := s.goals.Delete(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("delete goal (id=%d): %w", id, err)
	}
	if !ok {
		return apperr.ErrNotFound
	}

	s.log.InfoContext(ctx, "goal deleted",
		"user_id", userID,
		"goal_id", id,
	)
	return nil
}
