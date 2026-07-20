package service

import (
	"context"
	"errors"
	"strings"

	"budgetapp/internal/apperr"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"

	"github.com/jackc/pgx/v5"
)

type BudgetService struct {
	budgets    *repository.BudgetRepository
	categories *repository.CategoryRepository
}

func NewBudgetService(budgets *repository.BudgetRepository, categories *repository.CategoryRepository) *BudgetService {
	return &BudgetService{budgets: budgets, categories: categories}
}

func (s *BudgetService) List(ctx context.Context, userID int) ([]models.Budget, error) {
	return s.budgets.ListByUser(ctx, userID)
}

func (s *BudgetService) Create(ctx context.Context, userID int, in models.BudgetInput) (*models.Budget, error) {
	if err := validateBudgetInput(in); err != nil {
		return nil, err
	}
	if in.Color == "" {
		in.Color = "#6366f1"
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "expense")
	if err != nil {
		return nil, err
	}

	b, err := s.budgets.Create(ctx, userID, cat.ID, strings.TrimSpace(in.Name), in.Amount, in.Color)
	if err != nil {
		return nil, err
	}
	b.Category = cat.Name
	return b, nil
}

func (s *BudgetService) Update(ctx context.Context, id, userID int, in models.BudgetInput) (*models.Budget, error) {
	if err := validateBudgetInput(in); err != nil {
		return nil, err
	}
	if in.Color == "" {
		in.Color = "#6366f1"
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "expense")
	if err != nil {
		return nil, err
	}

	b, err := s.budgets.Update(ctx, id, userID, cat.ID, strings.TrimSpace(in.Name), in.Amount, in.Color)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	b.Category = cat.Name
	return b, nil
}

func (s *BudgetService) Delete(ctx context.Context, id, userID int) error {
	ok, err := s.budgets.Delete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrNotFound
	}
	return nil
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
	return nil
}
