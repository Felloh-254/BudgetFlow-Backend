package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"budgetapp/internal/apperr"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"

	"github.com/jackc/pgx/v5"
)

type BudgetService struct {
	budgets    *repository.BudgetRepository
	categories *repository.CategoryRepository
	log        *slog.Logger
}

func NewBudgetService(budgets *repository.BudgetRepository, categories *repository.CategoryRepository, log *slog.Logger) *BudgetService {
	return &BudgetService{
		budgets:    budgets,
		categories: categories,
		log:        log.With("component", "service.budget"),
	}
}

func (s *BudgetService) List(ctx context.Context, userID int, month string) ([]models.Budget, error) {
	budgets, err := s.budgets.ListByUser(ctx, userID, month)
	if err != nil {
		return nil, fmt.Errorf("list budgets (user=%d, month=%s): %w", userID, month, err)
	}
	return budgets, nil
}

func (s *BudgetService) Create(ctx context.Context, userID int, in models.BudgetInput) (*models.Budget, error) {
	if in.Month == "" {
		in.Month = time.Now().Format("2006-01")
	}

	if err := validateBudgetInput(in); err != nil {
		return nil, err
	}
	if in.Color == "" {
		in.Color = "#6366f1"
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "expense")
	if err != nil {
		return nil, fmt.Errorf("find/create category: %w", err)
	}

	b, err := s.budgets.Create(ctx, userID, cat.ID, strings.TrimSpace(in.Name), in.Amount, in.Color, in.Month)
	if err != nil {
		return nil, fmt.Errorf("create budget: %w", err)
	}
	b.Category = cat.Name

	s.log.InfoContext(ctx, "budget created",
		"user_id", userID,
		"budget_id", b.ID,
		"month", b.Month,
		"amount", b.Amount,
	)
	return b, nil
}

func (s *BudgetService) Update(ctx context.Context, id, userID int, in models.BudgetInput) (*models.Budget, error) {
	if in.Month == "" {
		in.Month = time.Now().Format("2006-01")
	}

	if err := validateBudgetInput(in); err != nil {
		return nil, err
	}
	if in.Color == "" {
		in.Color = "#6366f1"
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "expense")
	if err != nil {
		return nil, fmt.Errorf("find/create category: %w", err)
	}

	b, err := s.budgets.Update(ctx, id, userID, cat.ID, strings.TrimSpace(in.Name),
		in.Amount, in.Color, in.Month)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrNotFound
		}
		return nil, fmt.Errorf("update budget (id=%d): %w", id, err)
	}
	b.Category = cat.Name

	s.log.InfoContext(ctx, "budget updated",
		"user_id", userID,
		"budget_id", b.ID,
		"month", b.Month,
	)
	return b, nil
}

func (s *BudgetService) Delete(ctx context.Context, id, userID int) error {
	ok, err := s.budgets.Delete(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("delete budget (id=%d): %w", id, err)
	}
	if !ok {
		return apperr.ErrNotFound
	}

	s.log.InfoContext(ctx, "budget deleted",
		"user_id", userID,
		"budget_id", id,
	)
	return nil
}

func (s *BudgetService) CopyFromPreviousMonth(ctx context.Context, userID int, targetMonth string) (int, error) {
	t, err := time.Parse("2006-01", targetMonth)
	if err != nil {
		return 0, apperr.Validation("month must be in YYYY-MM format")
	}
	prevMonth := t.AddDate(0, -1, 0).Format("2006-01")

	n, err := s.budgets.CopyBudgetsFromMonth(ctx, userID, prevMonth, targetMonth)
	if err != nil {
		return 0, fmt.Errorf("copy budgets from %s to %s: %w", prevMonth, targetMonth, err)
	}

	s.log.InfoContext(ctx, "budgets copied",
		"user_id", userID,
		"copied", n,
		"from_month", prevMonth,
		"to_month", targetMonth,
	)
	return n, nil
}

func validateBudgetInput(in models.BudgetInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return apperr.Validation("budget name is required")
	}
	if strings.TrimSpace(in.Category) == "" {
		return apperr.Validation("category is required")
	}
	if in.Amount <= 0 {
		return apperr.Validation("amount must be greater than 0")
	}
	if in.Month != "" && len(in.Month) != 7 {
		return apperr.Validation("month must be in YYYY-MM format")
	}
	return nil
}
