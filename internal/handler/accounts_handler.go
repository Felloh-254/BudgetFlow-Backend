package handler

import "budgetapp/internal/service"

type AccountHandler struct {
	accounts *service.AccountsService
}

func NewAccountsHandler(accounts *service.AccountsService) *AccountHandler {
	return &AccountHandler{
		accounts: accounts,
	}
}
