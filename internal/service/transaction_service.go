package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"budgetapp/internal/apperr"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"

	"github.com/jackc/pgx/v5"
)

type TransactionService struct {
	transactions *repository.TransactionRepository
	categories   *repository.CategoryRepository
	accounts     *repository.AccountsRepository
}

func NewTransactionService(transactions *repository.TransactionRepository, categories *repository.CategoryRepository, accounts *repository.AccountsRepository) *TransactionService {
	return &TransactionService{transactions: transactions, categories: categories, accounts: accounts}
}

func (s *TransactionService) List(ctx context.Context, userID, limit, offset int) ([]models.Transaction, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.transactions.ListByUser(ctx, userID, limit, offset)
}

func (s *TransactionService) Create(ctx context.Context, userID int, in models.TransactionInput) (*models.Transaction, error) {
	if err := validateTransactionInput(in); err != nil {
		return nil, err
	}
	if exists, err := s.accounts.ExistsForUser(ctx, in.AccountID, userID); err != nil {
		return nil, err
	} else if !exists {
		return nil, apperr.ErrNotFound
	}
	if in.Date == "" {
		in.Date = time.Now().Format("2006-01-02")
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), in.Type)
	if err != nil {
		return nil, err
	}

	t, err := s.transactions.Create(ctx, userID, in.AccountID, in.BudgetID, cat.ID, strings.TrimSpace(in.Title), in.Amount, in.Type, in.Date, in.Note)
	if err != nil {
		return nil, err
	}
	t.Category = cat.Name
	return t, nil
}

func (s *TransactionService) Update(ctx context.Context, id, userID int, in models.TransactionInput) (*models.Transaction, error) {
	if err := validateTransactionInput(in); err != nil {
		return nil, err
	}
	if exists, err := s.accounts.ExistsForUser(ctx, in.AccountID, userID); err != nil {
		return nil, err
	} else if !exists {
		return nil, apperr.ErrNotFound
	}

	cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), in.Type)
	if err != nil {
		return nil, err
	}

	t, err := s.transactions.Update(ctx, id, userID, in.AccountID, in.BudgetID, cat.ID, strings.TrimSpace(in.Title), in.Amount, in.Type, in.Date, in.Note)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	t.Category = cat.Name
	return t, nil
}

func (s *TransactionService) Delete(ctx context.Context, id, userID int) error {
	ok, err := s.transactions.Delete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrNotFound
	}
	return nil
}

func validateTransactionInput(in models.TransactionInput) error {
	if in.AccountID <= 0 {
		return apperr.Validation("account_id must be greater than 0")
	}
	if strings.TrimSpace(in.Title) == "" {
		return apperr.Validation("title is required")
	}
	if in.Amount <= 0 {
		return apperr.Validation("amount must be greater than 0")
	}
	if in.Type != "income" && in.Type != "expense" {
		return apperr.Validation("type must be 'income' or 'expense'")
	}
	if strings.TrimSpace(in.Category) == "" {
		return apperr.Validation("category is required")
	}
	return nil
}
