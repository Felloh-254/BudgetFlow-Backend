package handler

import (
	"log"
	"net/http"
	"strconv"

	"budgetapp/internal/models"
	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type AccountHandler struct {
	accounts *service.AccountsService
}

func NewAccountsHandler(accounts *service.AccountsService) *AccountHandler {
	log.Println("[handler.accounts] NewAccountsHandler: created")
	return &AccountHandler{
		accounts: accounts,
	}
}

func (h *AccountHandler) ListAccounts(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.accounts] ListAccounts: user_id=%d", userID)

	accounts, err := h.accounts.List(c.Request().Context(), userID)
	if err != nil {
		log.Printf("[handler.accounts] ListAccounts: service error user_id=%d error=%v", userID, err)
		return respondError(c, err)
	}

	log.Printf("[handler.accounts] ListAccounts: OK user_id=%d count=%d", userID, len(accounts))
	return c.JSON(http.StatusOK, accounts)
}

func (h *AccountHandler) CreateAccount(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.accounts] CreateAccount: user_id=%d", userID)

	var in models.AccountInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.accounts] CreateAccount: bind failed user_id=%d error=%v", userID, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.accounts] CreateAccount: bound input user_id=%d name=%q type=%q currency=%q balance=%.2f",
		userID, in.Name, in.Type, in.Currency, in.Balance)

	account, err := h.accounts.Create(c.Request().Context(), userID, in)
	if err != nil {
		log.Printf("[handler.accounts] CreateAccount: service error user_id=%d error=%v", userID, err)
		return respondError(c, err)
	}

	log.Printf("[handler.accounts] CreateAccount: OK user_id=%d account_id=%d name=%q", userID, account.ID, account.Name)
	return c.JSON(http.StatusCreated, account)
}

func (h *AccountHandler) UpdateAccount(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.accounts] UpdateAccount: user_id=%d", userID)

	accountID, err := parseIDParam(c, "id")
	if err != nil || accountID <= 0 {
		log.Printf("[handler.accounts] UpdateAccount: invalid account id user_id=%d raw=%q error=%v",
			userID, c.Param("id"), err)
		return respondClientError(c, http.StatusBadRequest, "invalid account id", err)
	}

	var in models.AccountInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.accounts] UpdateAccount: bind failed user_id=%d account_id=%d error=%v",
			userID, accountID, err)
		return respondClientError(c, http.StatusBadRequest, "invalid request body", err)
	}

	log.Printf("[handler.accounts] UpdateAccount: bound input user_id=%d account_id=%d name=%q type=%q",
		userID, accountID, in.Name, in.Type)

	account, err := h.accounts.Update(c.Request().Context(), accountID, userID, in)
	if err != nil {
		log.Printf("[handler.accounts] UpdateAccount: service error user_id=%d account_id=%d error=%v",
			userID, accountID, err)
		return respondError(c, err)
	}

	log.Printf("[handler.accounts] UpdateAccount: OK user_id=%d account_id=%d", userID, accountID)
	return c.JSON(http.StatusOK, account)
}

func (h *AccountHandler) DeleteAccount(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.accounts] DeleteAccount: user_id=%d", userID)

	accountID, err := parseIDParam(c, "id")
	if err != nil {
		log.Printf("[handler.accounts] DeleteAccount: invalid account id user_id=%d raw=%q error=%v",
			userID, c.Param("id"), err)
		return c.JSON(
			http.StatusBadRequest,
			echo.Map{"error": "invalid account id"},
		)
	}

	err = h.accounts.Delete(c.Request().Context(), userID, accountID)
	if err != nil {
		log.Printf("[handler.accounts] DeleteAccount: service error user_id=%d account_id=%d error=%v",
			userID, accountID, err)
		return respondError(c, err)
	}

	log.Printf("[handler.accounts] DeleteAccount: OK user_id=%d account_id=%d", userID, accountID)
	return c.NoContent(http.StatusNoContent)
}

func parseIDParam(c echo.Context, name string) (int, error) {
	value := c.Param(name)
	log.Printf("[handler.accounts] parseIDParam: name=%s raw_value=%q", name, value)

	id, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("[handler.accounts] parseIDParam: FAILED to parse name=%s value=%q error=%v", name, value, err)
		return 0, err
	}

	log.Printf("[handler.accounts] parseIDParam: OK name=%s id=%d", name, id)
	return id, nil
}
