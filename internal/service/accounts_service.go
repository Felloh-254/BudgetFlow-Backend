package service

import (
	"budgetapp/internal/apperr"
	"budgetapp/internal/constants"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"
	"context"
)

type AccountsService struct {
	accounts *repository.AccountsRepository
}

func NewAccountsService(accounts *repository.AccountsRepository) *AccountsService {
	return &AccountsService{accounts: accounts}
}

func (s *AccountsService) List(ctx context.Context, userID int) ([]models.Account, error) {
	return s.accounts.ListAccountsByUser(ctx, userID)
}

func (s *AccountsService) Create(ctx context.Context, userID int, in models.AccountInput) (*models.Account, error) {
	if err := validateAccountInput(in); err != nil {
		return nil, err
	}

	return s.accounts.CreateAccount(ctx, userID, in.Name, in.Type, in.AccountNumber, in.Balance, in.Currency)
}

func (s *AccountsService) Update(ctx context.Context, userID int, in models.AccountInput) (*models.Account, error) {
	return s.accounts.UpdateAccount(ctx, userID, in.Name, in.Type, in.AccountNumber, in.Balance, in.Currency)
}

func (s *AccountsService) Delete(ctx context.Context, userID int, accountID int) error {
	return s.accounts.DeleteAccount(ctx, userID, accountID)
}

func validateAccountInput(in models.AccountInput) error {
	if in.Name == "" {
		return apperr.ErrAccountNameRequired
	}

	if in.Type == "" {
		return apperr.ErrInvalidAccountType
	}

	if !constants.AllowedAccountTypes[in.Type] {
		return apperr.ErrUnsupportedAccountType
	}

	if in.Balance < 0 {
		return apperr.ErrInvalidBalance
	}

	if in.Currency == "" {
		return apperr.ErrUnsupportedCurrency
	}

	if !constants.AllowedCurrencies[in.Currency] {
		return apperr.ErrUnsupportedCurrency
	}

	return nil
}
