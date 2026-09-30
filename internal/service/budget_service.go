package service

import (
	"context"
	"errors"
	"log"
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
}

func NewBudgetService(budgets *repository.BudgetRepository, categories *repository.CategoryRepository) *BudgetService {
	log.Println("[service.budget] NewBudgetService: created")
	return &BudgetService{budgets: budgets, categories: categories}
}

func (s *BudgetService) List(ctx context.Context, userID int, month string) ([]models.Budget, error) {
	log.Printf("[service.budget] List: user_id=%d month=%q", userID, month)

	budgets, err := s.budgets.ListByUser(ctx, userID, month)
	if err != nil {
		log.Printf("[service.budget] List: repo error user_id=%d month=%q error=%v", userID, month, err)
		return nil, err
	}

	log.Printf("[service.budget] List: OK user_id=%d month=%q count=%d", userID, month, len(budgets))
	return budgets, nil
}

func (s *BudgetService) Create(ctx context.Context, userID int, in models.BudgetInput) (*models.Budget, error) {
	if in.Month == "" {
		in.Month = time.Now().Format("2006-01")
	}
	log.Printf("[service.budget] Create: user_id=%d name=%q amount=%.2f month=%q",
		userID, in.Name, in.Amount, in.Month)

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

	b, err := s.budgets.Create(ctx, userID, cat.ID, strings.TrimSpace(in.Name), in.Amount, in.Color, in.Month)
	if err != nil {
		log.Printf("[service.budget] Create: repo error user_id=%d error=%v", userID, err)
		return nil, err
	}
	b.Category = cat.Name

	log.Printf("[service.budget] Create: OK user_id=%d budget_id=%d month=%q", userID, b.ID, b.Month)
	return b, nil
}

func (s *BudgetService) Update(ctx context.Context, id, userID int, in models.BudgetInput) (*models.Budget, error) {
	if in.Month == "" {
		in.Month = time.Now().Format("2006-01")
	}
	log.Printf("[service.budget] Update: budget_id=%d user_id=%d name=%q amount=%.2f month=%q",
		id, userID, in.Name, in.Amount, in.Month)

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

	b, err := s.budgets.Update(ctx, id, userID, cat.ID, strings.TrimSpace(in.Name),
		in.Amount, in.Color, in.Month)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[service.budget] Update: not found budget_id=%d user_id=%d", id, userID)
			return nil, apperr.ErrNotFound
		}
		log.Printf("[service.budget] Update: repo error budget_id=%d error=%v", id, err)
		return nil, err
	}
	b.Category = cat.Name

	log.Printf("[service.budget] Update: OK budget_id=%d month=%q", id, b.Month)
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

func (s *BudgetService) CopyFromPreviousMonth(ctx context.Context, userID int, targetMonth string) (int, error) {
	log.Printf("[service.budget] CopyFromPreviousMonth: user_id=%d target=%q", userID, targetMonth)

	t, err := time.Parse("2006-01", targetMonth)
	if err != nil {
		log.Printf("[service.budget] CopyFromPreviousMonth: invalid month user_id=%d target=%q error=%v",
			userID, targetMonth, err)
		return 0, apperr.Validation("month must be in YYYY-MM format")
	}
	prevMonth := t.AddDate(0, -1, 0).Format("2006-01")

	n, err := s.budgets.CopyBudgetsFromMonth(ctx, userID, prevMonth, targetMonth)
	if err != nil {
		log.Printf("[service.budget] CopyFromPreviousMonth: repo error user_id=%d error=%v", userID, err)
		return 0, err
	}
	log.Printf("[service.budget] CopyFromPreviousMonth: OK user_id=%d copied=%d from=%q to=%q",
		userID, n, prevMonth, targetMonth)
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
