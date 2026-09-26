package handler

import (
	"net/http"
	"strconv"
	"strings"

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

// List returns all transactions for the current user matching filters
func (h *TransactionHandler) List(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	catID, _ := strconv.Atoi(c.QueryParam("category_id"))
	accID, _ := strconv.Atoi(c.QueryParam("account_id"))

	filter := models.TransactionFilter{
		Limit:      limit,
		Offset:     offset,
		Month:      c.QueryParam("month"),
		StartDate:  c.QueryParam("start_date"),
		EndDate:    c.QueryParam("end_date"),
		Type:       c.QueryParam("type"),
		CategoryID: catID,
		AccountID:  accID,
	}

	txns, err := h.transactions.List(c.Request().Context(), currentUserID(c), filter)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, txns)
}

// GetByID retrieves a single transaction by ID with full details
func (h *TransactionHandler) GetByID(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}
	detail, err := h.transactions.GetByID(c.Request().Context(), id, currentUserID(c))
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, detail)
}

// Create handles the generic POST /api/transactions endpoint
func (h *TransactionHandler) Create(c echo.Context) error {
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)
	c.Logger().Infof("[TXN-TRACE] Create (generic) HIT: req_id=%s user_id=%d remote_addr=%s idempotency_key=%s",
		reqID, currentUserID(c), c.Request().RemoteAddr, c.Request().Header.Get("Idempotency-Key"))

	var in models.TransactionInput

	if err := c.Bind(&in); err != nil {
		c.Logger().Errorf(
			"transaction create bind failed: user_id=%d error=%v",
			currentUserID(c),
			err,
		)
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid request body",
		})
	}

	idempotencyKey := c.Request().Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Idempotency-Key header is required",
		})
	}

	switch strings.ToLower(in.Type) {
	case "income":
		in.Type = "income"

		detail, err := h.transactions.CreateIncome(
			c.Request().Context(),
			currentUserID(c),
			in,
			idempotencyKey,
		)
		if err != nil {
			return respondError(c, err)
		}

		return c.JSON(http.StatusCreated, detail)

	case "expense":
		in.Type = "expense"

		detail, err := h.transactions.CreateExpense(
			c.Request().Context(),
			currentUserID(c),
			in,
			idempotencyKey,
		)
		if err != nil {
			return respondError(c, err)
		}

		return c.JSON(http.StatusCreated, detail)

	default:
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "type must be 'income' or 'expense'",
		})
	}
}

// Update handles updating transaction metadata
func (h *TransactionHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}

	var in models.UpdateTransactionInput
	if err := c.Bind(&in); err != nil {
		c.Logger().Errorf("transaction update bind failed: user_id=%d error=%v", currentUserID(c), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	detail, err := h.transactions.Update(c.Request().Context(), id, currentUserID(c), in)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, detail)
}

// CreateIncome creates an income transaction
func (h *TransactionHandler) CreateIncome(c echo.Context) error {
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)
	c.Logger().Infof("[TXN-TRACE] CreateIncome HIT: req_id=%s user_id=%d remote_addr=%s idempotency_key=%s",
		reqID, currentUserID(c), c.Request().RemoteAddr, c.Request().Header.Get("Idempotency-Key"))

	var in models.TransactionInput
	if err := c.Bind(&in); err != nil {
		c.Logger().Errorf("income create bind failed: user_id=%d error=%v", currentUserID(c), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	idempotencyKey := c.Request().Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Idempotency-Key header is required",
		})
	}

	in.Type = "income"

	c.Logger().Infof("[TXN-TRACE] CreateIncome calling service: req_id=%s user_id=%d idempotency_key=%s amount=%.2f account_id=%d title=%q",
		reqID, currentUserID(c), idempotencyKey, in.Amount, in.AccountID, in.Title)

	detail, err := h.transactions.CreateIncome(c.Request().Context(), currentUserID(c), in, idempotencyKey)
	if err != nil {
		c.Logger().Errorf("[TXN-TRACE] CreateIncome service error: req_id=%s user_id=%d idempotency_key=%s error=%v",
			reqID, currentUserID(c), idempotencyKey, err)
		return respondError(c, err)
	}
	c.Logger().Infof("[TXN-TRACE] CreateIncome DONE: req_id=%s user_id=%d transaction_id=%d idempotency_key=%s amount=%.2f entries=%d",
		reqID, currentUserID(c), detail.Transaction.ID, idempotencyKey, detail.Entries[0].Amount, len(detail.Entries))
	return c.JSON(http.StatusCreated, detail)
}

// CreateExpense creates an expense transaction
func (h *TransactionHandler) CreateExpense(c echo.Context) error {
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)
	c.Logger().Infof("[TXN-TRACE] CreateExpense HIT: req_id=%s user_id=%d remote_addr=%s idempotency_key=%s",
		reqID, currentUserID(c), c.Request().RemoteAddr, c.Request().Header.Get("Idempotency-Key"))

	var in models.TransactionInput
	if err := c.Bind(&in); err != nil {
		c.Logger().Errorf("expense create bind failed: user_id=%d error=%v", currentUserID(c), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	idempotencyKey := c.Request().Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Idempotency-Key header is required",
		})
	}

	in.Type = "expense"

	c.Logger().Infof("[TXN-TRACE] CreateExpense calling service: req_id=%s user_id=%d idempotency_key=%s amount=%.2f account_id=%d title=%q",
		reqID, currentUserID(c), idempotencyKey, in.Amount, in.AccountID, in.Title)

	detail, err := h.transactions.CreateExpense(c.Request().Context(), currentUserID(c), in, idempotencyKey)
	if err != nil {
		c.Logger().Errorf("[TXN-TRACE] CreateExpense service error: req_id=%s user_id=%d idempotency_key=%s error=%v",
			reqID, currentUserID(c), idempotencyKey, err)
		return respondError(c, err)
	}
	c.Logger().Infof("[TXN-TRACE] CreateExpense DONE: req_id=%s user_id=%d transaction_id=%d idempotency_key=%s amount=%.2f entries=%d",
		reqID, currentUserID(c), detail.Transaction.ID, idempotencyKey, -detail.Entries[0].Amount, len(detail.Entries))
	return c.JSON(http.StatusCreated, detail)
}

// CreateTransfer creates a transfer between two accounts
func (h *TransactionHandler) CreateTransfer(c echo.Context) error {
	var in models.TransferInput
	if err := c.Bind(&in); err != nil {
		c.Logger().Errorf("transfer create bind failed: user_id=%d error=%v", currentUserID(c), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	idempotencyKey := c.Request().Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Idempotency-Key header is required",
		})
	}

	detail, err := h.transactions.CreateTransfer(c.Request().Context(), currentUserID(c), in, idempotencyKey)
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
