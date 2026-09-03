package handler

import (
	"net/http"
	"strconv"

	"budgetapp/internal/models"
	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type TransactionHandler struct {
	transactions *service.TransactionService
}

func NewTransactionHandler(transactions *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{transactions: transactions}
}

func (h *TransactionHandler) List(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	txns, err := h.transactions.List(c.Request().Context(), currentUserID(c), limit, offset)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, txns)
}

func (h *TransactionHandler) Create(c echo.Context) error {
	var in models.TransactionInput
	if err := c.Bind(&in); err != nil {
		c.Logger().Errorf("transaction create bind failed: request_id=%s user_id=%d error=%v", c.Response().Header().Get(echo.HeaderXRequestID), currentUserID(c), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	c.Logger().Infof("transaction create: request_id=%s user_id=%d account_id=%d budget_id=%v amount=%.2f type=%s", c.Response().Header().Get(echo.HeaderXRequestID), currentUserID(c), in.AccountID, in.BudgetID, in.Amount, in.Type)
	t, err := h.transactions.Create(c.Request().Context(), currentUserID(c), in)
	if err != nil {
		return respondError(c, err)
	}
	c.Logger().Infof("transaction created: request_id=%s user_id=%d transaction_id=%d account_id=%d", c.Response().Header().Get(echo.HeaderXRequestID), currentUserID(c), t.ID, t.AccountID)
	return c.JSON(http.StatusCreated, t)
}

func (h *TransactionHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}
	var in models.TransactionInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	t, err := h.transactions.Update(c.Request().Context(), id, currentUserID(c), in)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, t)
}

func (h *TransactionHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}
	if err := h.transactions.Delete(c.Request().Context(), id, currentUserID(c)); err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"message": "deleted"})
}
