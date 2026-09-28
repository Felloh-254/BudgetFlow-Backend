package service

import (
	"context"
	"errors"
	"log"

	"budgetapp/internal/apperr"
	"budgetapp/internal/constants"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"

	"github.com/jackc/pgx/v5"
)

type AccountsService struct {
	accounts *repository.AccountsRepository
}

func NewAccountsService(accounts *repository.AccountsRepository) *AccountsService {
	log.Println("[service.accounts] NewAccountsService: created")
	return &AccountsService{accounts: accounts}
}

func (s *AccountsService) List(ctx context.Context, userID int) ([]models.Account, error) {
	log.Printf("[service.accounts] List: user_id=%d", userID)

	accounts, err := s.accounts.ListAccountsByUser(ctx, userID)
	if err != nil {
		log.Printf("[service.accounts] List: repo error user_id=%d error=%v", userID, err)
		return nil, err
	}

	log.Printf("[service.accounts] List: OK user_id=%d count=%d", userID, len(accounts))
	return accounts, nil
}

func (s *AccountsService) Create(ctx context.Context, userID int, in models.AccountInput) (*models.Account, error) {
	log.Printf("[service.accounts] Create: user_id=%d name=%q type=%q currency=%q balance=%.2f",
		userID, in.Name, in.Type, in.Currency, in.Balance)

	if err := validateAccountInput(in); err != nil {
		log.Printf("[service.accounts] Create: validation failed user_id=%d error=%v", userID, err)
		return nil, err
	}

	account, err := s.accounts.CreateAccount(ctx, userID, in.Name, in.Type, in.AccountNumber, in.Balance, in.Currency)
	if err != nil {
		log.Printf("[service.accounts] Create: repo error user_id=%d error=%v", userID, err)
		return nil, err
	}

	log.Printf("[service.accounts] Create: OK user_id=%d account_id=%d name=%q", userID, account.ID, account.Name)
	return account, nil
}

func (s *AccountsService) Update(ctx context.Context, accountID, userID int, in models.AccountInput) (*models.Account, error) {
	log.Printf("[service.accounts] Update: account_id=%d user_id=%d name=%q type=%q",
		accountID, userID, in.Name, in.Type)

	if err := validateAccountInput(in); err != nil {
		log.Printf("[service.accounts] Update: validation failed account_id=%d error=%v", accountID, err)
		return nil, err
	}

	account, err := s.accounts.UpdateAccount(ctx, accountID, userID, in.Name, in.Type, in.AccountNumber, in.Currency)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("[service.accounts] Update: not found account_id=%d user_id=%d", accountID, userID)
		return nil, apperr.ErrNotFound
	}
	if err != nil {
		log.Printf("[service.accounts] Update: repo error account_id=%d error=%v", accountID, err)
		return nil, err
	}

	log.Printf("[service.accounts] Update: OK account_id=%d name=%q", accountID, account.Name)
	return account, nil
}

func (s *AccountsService) Delete(ctx context.Context, userID int, accountID int) error {
	log.Printf("[service.accounts] Delete: user_id=%d account_id=%d", userID, accountID)

	err := s.accounts.DeleteAccount(ctx, userID, accountID)
	if err != nil {
		log.Printf("[service.accounts] Delete: repo error user_id=%d account_id=%d error=%v", userID, accountID, err)
		return err
	}

	log.Printf("[service.accounts] Delete: OK user_id=%d account_id=%d", userID, accountID)
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
