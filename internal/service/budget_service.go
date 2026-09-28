package service

import (
	"context"
	"errors"
	"log"
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
	log.Println("[service.budget] NewBudgetService: created")
	return &BudgetService{budgets: budgets, categories: categories}
}

func (s *BudgetService) List(ctx context.Context, userID int) ([]models.Budget, error) {
	log.Printf("[service.budget] List: user_id=%d", userID)

	budgets, err := s.budgets.ListByUser(ctx, userID)
	if err != nil {
		log.Printf("[service.budget] List: repo error user_id=%d error=%v", userID, err)
		return nil, err
	}

	log.Printf("[service.budget] List: OK user_id=%d count=%d", userID, len(budgets))
	return budgets, nil
}

func (s *BudgetService) Create(ctx context.Context, userID int, in models.BudgetInput) (*models.Budget, error) {
	log.Printf("[service.budget] Create: user_id=%d name=%q amount=%.2f category=%q",
		userID, in.Name, in.Amount, in.Category)

	if err := validateBudgetInput(in); err != nil {
		log.Printf("[service.budget] Create: validation failed user_id=%d error=%v", userID, err)
		return nil, err
	}
	if in.Color == "" {
		in.Color = "#6366f1"
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "expense")
	if err != nil {
		log.Printf("[service.budget] Create: category find/create failed user_id=%d category=%q error=%v",
			userID, in.Category, err)
		return nil, err
	}

	b, err := s.budgets.Create(ctx, userID, cat.ID, strings.TrimSpace(in.Name), in.Amount, in.Color)
	if err != nil {
		log.Printf("[service.budget] Create: repo error user_id=%d error=%v", userID, err)
		return nil, err
	}
	b.Category = cat.Name

	log.Printf("[service.budget] Create: OK user_id=%d budget_id=%d name=%q", userID, b.ID, b.Name)
	return b, nil
}

func (s *BudgetService) Update(ctx context.Context, id, userID int, in models.BudgetInput) (*models.Budget, error) {
	log.Printf("[service.budget] Update: budget_id=%d user_id=%d name=%q amount=%.2f",
		id, userID, in.Name, in.Amount)

	if err := validateBudgetInput(in); err != nil {
		log.Printf("[service.budget] Update: validation failed budget_id=%d error=%v", id, err)
		return nil, err
	}
	if in.Color == "" {
		in.Color = "#6366f1"
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), "expense")
	if err != nil {
		log.Printf("[service.budget] Update: category find/create failed budget_id=%d error=%v", id, err)
		return nil, err
	}

	b, err := s.budgets.Update(ctx, id, userID, cat.ID, strings.TrimSpace(in.Name), in.Amount, in.Color)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[service.budget] Update: not found budget_id=%d user_id=%d", id, userID)
			return nil, apperr.ErrNotFound
		}
		log.Printf("[service.budget] Update: repo error budget_id=%d error=%v", id, err)
		return nil, err
	}
	b.Category = cat.Name

	log.Printf("[service.budget] Update: OK budget_id=%d name=%q", id, b.Name)
	return b, nil
}

func (s *BudgetService) Delete(ctx context.Context, id, userID int) error {
	log.Printf("[service.budget] Delete: budget_id=%d user_id=%d", id, userID)

	ok, err := s.budgets.Delete(ctx, id, userID)
	if err != nil {
		log.Printf("[service.budget] Delete: repo error budget_id=%d error=%v", id, err)
		return err
	}
	if !ok {
		log.Printf("[service.budget] Delete: not found budget_id=%d user_id=%d", id, userID)
		return apperr.ErrNotFound
	}

	log.Printf("[service.budget] Delete: OK budget_id=%d", id)
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
