package handler

import (
	"budgetapp/internal/models"
	"budgetapp/internal/service"
	"net/http"
	"strconv"

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

func (h *AccountHandler) ListAccounts(c echo.Context) error {
	accounts, err := h.accounts.List(c.Request().Context(), currentUserID(c))

	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, accounts)
}

func (h *AccountHandler) CreateAccount(c echo.Context) error {
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

func (h *AccountHandler) UpdateAccount(c echo.Context) error {
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

func (h *AccountHandler) DeleteAccount(c echo.Context) error {
	accountID, err := parseIDParam(c, "id")
	if err != nil {
		return c.JSON(
			http.StatusBadRequest,
			echo.Map{"error": "invalid account id"},
		)
	}

	err = h.accounts.Delete(
		c.Request().Context(),
		currentUserID(c),
		accountID,
	)
	if err != nil {
		return respondError(c, err)
	}

	return c.NoContent(http.StatusNoContent)
}

func parseIDParam(c echo.Context, name string) (int, error) {
	value := c.Param(name)

	id, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}

	return id, nil
}
