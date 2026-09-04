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

// List returns all transactions for the current user
func (h *TransactionHandler) List(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	txns, err := h.transactions.List(c.Request().Context(), currentUserID(c), limit, offset)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, txns)
}

// CreateIncome creates an income transaction
func (h *TransactionHandler) CreateIncome(c echo.Context) error {
	var in models.TransactionInput
	if err := c.Bind(&in); err != nil {
		c.Logger().Errorf("income create bind failed: user_id=%d error=%v", currentUserID(c), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	in.Type = "income"

	detail, err := h.transactions.CreateIncome(c.Request().Context(), currentUserID(c), in)
	if err != nil {
		return respondError(c, err)
	}
	c.Logger().Infof("income created: user_id=%d transaction_id=%d amount=%.2f", currentUserID(c), detail.Transaction.ID, detail.Entries[0].Amount)
	return c.JSON(http.StatusCreated, detail)
}

// CreateExpense creates an expense transaction
func (h *TransactionHandler) CreateExpense(c echo.Context) error {
	var in models.TransactionInput
	if err := c.Bind(&in); err != nil {
		c.Logger().Errorf("expense create bind failed: user_id=%d error=%v", currentUserID(c), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	in.Type = "expense"

	detail, err := h.transactions.CreateExpense(c.Request().Context(), currentUserID(c), in)
	if err != nil {
		return respondError(c, err)
	}
	c.Logger().Infof("expense created: user_id=%d transaction_id=%d amount=%.2f", currentUserID(c), detail.Transaction.ID, -detail.Entries[0].Amount)
	return c.JSON(http.StatusCreated, detail)
}

// CreateTransfer creates a transfer between two accounts
func (h *TransactionHandler) CreateTransfer(c echo.Context) error {
	var in models.TransferInput
	if err := c.Bind(&in); err != nil {
		c.Logger().Errorf("transfer create bind failed: user_id=%d error=%v", currentUserID(c), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	detail, err := h.transactions.CreateTransfer(c.Request().Context(), currentUserID(c), in)
	if err != nil {
		return respondError(c, err)
	}
	c.Logger().Infof("transfer created: user_id=%d transaction_id=%d amount=%.2f from=%d to=%d",
		currentUserID(c), detail.Transaction.ID, detail.Entries[0].Amount, in.FromAccountID, in.ToAccountID)
	return c.JSON(http.StatusCreated, detail)
}

// Delete removes a transaction
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
