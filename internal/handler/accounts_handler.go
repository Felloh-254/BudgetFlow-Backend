package handler

import (
	"budgetapp/internal/models"
	"budgetapp/internal/service"
	"net/http"

	"github.com/labstack/echo/v4"
)

type AccountHandler struct {
	accounts *service.AccountsService
}

func NewAccountsHandler(accounts *service.AccountsService) *AccountHandler {
	return &AccountHandler{
		accounts: accounts,
	}
}

func (h *AccountHandler) List(c echo.Context) error {
	accounts, err := h.accounts.List(c.Request().Context(), currentUserID(c))

	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, accounts)
}

func (h *AccountHandler) Create(c echo.Context) error {
	var in models.AccountInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	account, err := h.accounts.Create(c.Request().Context(), currentUserID(c), in)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, account)

}

func (h *AccountHandler) Update(c echo.Context) error {
	var in models.AccountInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	account, err := h.accounts.Update(c.Request().Context(), currentUserID(c), in)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, account)
}
