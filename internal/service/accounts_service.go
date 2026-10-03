package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"budgetapp/internal/apperr"
	"budgetapp/internal/constants"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"

	"github.com/jackc/pgx/v5"
)

type AccountsService struct {
	accounts *repository.AccountsRepository
	log      *slog.Logger
}

func NewAccountsService(accounts *repository.AccountsRepository, log *slog.Logger) *AccountsService {
	return &AccountsService{
		accounts: accounts,
		log:      log.With("component", "service.accounts"),
	}
}

func (s *AccountsService) List(ctx context.Context, userID int) ([]models.Account, error) {
	accounts, err := s.accounts.ListAccountsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list accounts (user=%d): %w", userID, err)
	}
	return accounts, nil
}

func (s *AccountsService) Create(ctx context.Context, userID int, in models.AccountInput) (*models.Account, error) {
	var err error
	in, err = normalizeAccountInput(in)
	if err != nil {
		return nil, err
	}
	if err := validateAccountInput(in); err != nil {
		return nil, err
	}

	account, err := s.accounts.CreateAccount(ctx, userID, in.Name, in.Type, in.Provider, in.AccountNumber, in.Balance, in.Currency)
	if err != nil {
		return nil, fmt.Errorf("create account (user=%d): %w", userID, err)
	}

	s.log.InfoContext(ctx, "account created",
		"user_id", userID,
		"account_id", account.ID,
		"name", account.Name,
		"type", account.Type,
		"currency", account.Currency,
	)
	return account, nil
}

func (s *AccountsService) Update(ctx context.Context, accountID, userID int, in models.AccountInput) (*models.Account, error) {
	var err error
	in, err = normalizeAccountInput(in)
	if err != nil {
		return nil, err
	}
	if err := validateAccountInput(in); err != nil {
		return nil, err
	}

	account, err := s.accounts.UpdateAccount(ctx, accountID, userID, in.Name, in.Type, in.Provider, in.AccountNumber, in.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update account (id=%d, user=%d): %w", accountID, userID, err)
	}

	s.log.InfoContext(ctx, "account updated",
		"user_id", userID,
		"account_id", account.ID,
		"name", account.Name,
		"type", account.Type,
	)
	return account, nil
}

func (s *AccountsService) Delete(ctx context.Context, userID int, accountID int) error {
	err := s.accounts.DeleteAccount(ctx, userID, accountID)
	if err != nil {
		return fmt.Errorf("delete account (id=%d, user=%d): %w", accountID, userID, err)
	}

	s.log.InfoContext(ctx, "account deleted",
		"user_id", userID,
		"account_id", accountID,
	)
	return nil
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
	if in.Type == "mobile_money" {
		if in.Provider != "" && !constants.AllowedMobileMoneyProviders[in.Provider] {
			return apperr.ErrUnsupportedAccountType
		}
	} else if in.Provider != "" {
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

// normalizeAccountInput keeps provider-specific mobile money labels compatible
// with clients that send them as the account type.
func normalizeAccountInput(in models.AccountInput) (models.AccountInput, error) {
	switch in.Type {
	case "mpesa", "airtel_money":
		provider := in.Type
		if in.Provider != "" && in.Provider != provider {
			return in, apperr.ErrUnsupportedAccountType
		}
		in.Type = "mobile_money"
		in.Provider = provider
	}
	return in, nil
}
