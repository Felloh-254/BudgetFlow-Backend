package handler

import (
	"log"
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
	log.Println("[handler.transaction] NewTransactionHandler: created")
	return &TransactionHandler{transactions: transactions}
}

// List returns all transactions for the current user matching filters
func (h *TransactionHandler) List(c echo.Context) error {
	userID := currentUserID(c)

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	catID, _ := strconv.Atoi(c.QueryParam("category_id"))
	accID, _ := strconv.Atoi(c.QueryParam("account_id"))

	log.Printf("[handler.transaction] List: user_id=%d limit=%d offset=%d month=%q start=%q end=%q type=%q category_id=%d account_id=%d",
		userID, limit, offset, c.QueryParam("month"), c.QueryParam("start_date"),
		c.QueryParam("end_date"), c.QueryParam("type"), catID, accID)

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

	txns, err := h.transactions.List(c.Request().Context(), userID, filter)
	if err != nil {
		log.Printf("[handler.transaction] List: service error user_id=%d error=%v", userID, err)
		return respondError(c, err)
	}

	log.Printf("[handler.transaction] List: OK user_id=%d count=%d", userID, len(txns))
	return c.JSON(http.StatusOK, txns)
}

// GetByID retrieves a single transaction by ID with full details
func (h *TransactionHandler) GetByID(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.transaction] GetByID: user_id=%d raw_id=%q", userID, c.Param("id"))

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("[handler.transaction] GetByID: invalid id user_id=%d raw=%q error=%v", userID, c.Param("id"), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}

	detail, err := h.transactions.GetByID(c.Request().Context(), id, userID)
	if err != nil {
		log.Printf("[handler.transaction] GetByID: service error user_id=%d transaction_id=%d error=%v", userID, id, err)
		return respondError(c, err)
	}

	log.Printf("[handler.transaction] GetByID: OK user_id=%d transaction_id=%d type=%s entries=%d",
		userID, id, detail.Transaction.Type, len(detail.Entries))
	return c.JSON(http.StatusOK, detail)
}

// Create handles the generic POST /api/transactions endpoint
func (h *TransactionHandler) Create(c echo.Context) error {
	userID := currentUserID(c)
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)

	log.Printf("[handler.transaction] Create (generic) HIT: req_id=%s user_id=%d remote_addr=%s idempotency_key=%q",
		reqID, userID, c.Request().RemoteAddr, c.Request().Header.Get("Idempotency-Key"))

	var in models.TransactionInput

	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.transaction] Create: bind failed user_id=%d req_id=%s error=%v", userID, reqID, err)
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "invalid request body",
		})
	}

	log.Printf("[handler.transaction] Create: bound input user_id=%d type=%q title=%q amount=%.2f account_id=%d category=%q date=%q",
		userID, in.Type, in.Title, in.Amount, in.AccountID, in.Category, in.Date)

	idempotencyKey := c.Request().Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		log.Printf("[handler.transaction] Create: missing Idempotency-Key header user_id=%d", userID)
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Idempotency-Key header is required",
		})
	}

	switch strings.ToLower(in.Type) {
	case "income":
		in.Type = "income"
		log.Printf("[handler.transaction] Create: routing to CreateIncome user_id=%d req_id=%s", userID, reqID)

		detail, err := h.transactions.CreateIncome(
			c.Request().Context(),
			userID,
			in,
			idempotencyKey,
		)
		if err != nil {
			log.Printf("[handler.transaction] Create: CreateIncome failed user_id=%d req_id=%s error=%v", userID, reqID, err)
			return respondError(c, err)
		}

		log.Printf("[handler.transaction] Create: CreateIncome OK user_id=%d transaction_id=%d", userID, detail.Transaction.ID)
		return c.JSON(http.StatusCreated, detail)

	case "expense":
		in.Type = "expense"
		log.Printf("[handler.transaction] Create: routing to CreateExpense user_id=%d req_id=%s", userID, reqID)

		detail, err := h.transactions.CreateExpense(
			c.Request().Context(),
			userID,
			in,
			idempotencyKey,
		)
		if err != nil {
			log.Printf("[handler.transaction] Create: CreateExpense failed user_id=%d req_id=%s error=%v", userID, reqID, err)
			return respondError(c, err)
		}

		log.Printf("[handler.transaction] Create: CreateExpense OK user_id=%d transaction_id=%d", userID, detail.Transaction.ID)
		return c.JSON(http.StatusCreated, detail)

	default:
		log.Printf("[handler.transaction] Create: invalid type user_id=%d type=%q", userID, in.Type)
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "type must be 'income' or 'expense'",
		})
	}
}

// Update handles updating transaction metadata
func (h *TransactionHandler) Update(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.transaction] Update: user_id=%d raw_id=%q", userID, c.Param("id"))

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("[handler.transaction] Update: invalid id user_id=%d raw=%q error=%v", userID, c.Param("id"), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}

	var in models.UpdateTransactionInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.transaction] Update: bind failed user_id=%d transaction_id=%d error=%v", userID, id, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.transaction] Update: bound input user_id=%d transaction_id=%d title=%q date=%q note_length=%d",
		userID, id, in.Title, in.Date, len(in.Note))

	detail, err := h.transactions.Update(c.Request().Context(), id, userID, in)
	if err != nil {
		log.Printf("[handler.transaction] Update: service error user_id=%d transaction_id=%d error=%v", userID, id, err)
		return respondError(c, err)
	}

	log.Printf("[handler.transaction] Update: OK user_id=%d transaction_id=%d", userID, id)
	return c.JSON(http.StatusOK, detail)
}

// CreateIncome creates an income transaction
func (h *TransactionHandler) CreateIncome(c echo.Context) error {
	userID := currentUserID(c)
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)

	log.Printf("[handler.transaction] CreateIncome HIT: req_id=%s user_id=%d remote_addr=%s idempotency_key=%q",
		reqID, userID, c.Request().RemoteAddr, c.Request().Header.Get("Idempotency-Key"))

	var in models.TransactionInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.transaction] CreateIncome: bind failed user_id=%d req_id=%s error=%v", userID, reqID, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.transaction] CreateIncome: bound input user_id=%d title=%q amount=%.2f account_id=%d category=%q date=%q",
		userID, in.Title, in.Amount, in.AccountID, in.Category, in.Date)

	idempotencyKey := c.Request().Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		log.Printf("[handler.transaction] CreateIncome: missing Idempotency-Key header user_id=%d", userID)
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Idempotency-Key header is required",
		})
	}

	in.Type = "income"

	log.Printf("[handler.transaction] CreateIncome: calling service user_id=%d req_id=%s idempotency_key=%s amount=%.2f account_id=%d title=%q",
		userID, reqID, idempotencyKey, in.Amount, in.AccountID, in.Title)

	detail, err := h.transactions.CreateIncome(c.Request().Context(), userID, in, idempotencyKey)
	if err != nil {
		log.Printf("[handler.transaction] CreateIncome: service error user_id=%d req_id=%s idempotency_key=%s error=%v",
			userID, reqID, idempotencyKey, err)
		return respondError(c, err)
	}

	log.Printf("[handler.transaction] CreateIncome: DONE user_id=%d req_id=%s transaction_id=%d idempotency_key=%s amount=%.2f entries=%d",
		userID, reqID, detail.Transaction.ID, idempotencyKey, detail.Entries[0].Amount, len(detail.Entries))
	return c.JSON(http.StatusCreated, detail)
}

// CreateExpense creates an expense transaction
func (h *TransactionHandler) CreateExpense(c echo.Context) error {
	userID := currentUserID(c)
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)

	log.Printf("[handler.transaction] CreateExpense HIT: req_id=%s user_id=%d remote_addr=%s idempotency_key=%q",
		reqID, userID, c.Request().RemoteAddr, c.Request().Header.Get("Idempotency-Key"))

	var in models.TransactionInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.transaction] CreateExpense: bind failed user_id=%d req_id=%s error=%v", userID, reqID, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.transaction] CreateExpense: bound input user_id=%d title=%q amount=%.2f account_id=%d category=%q date=%q",
		userID, in.Title, in.Amount, in.AccountID, in.Category, in.Date)

	idempotencyKey := c.Request().Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		log.Printf("[handler.transaction] CreateExpense: missing Idempotency-Key header user_id=%d", userID)
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Idempotency-Key header is required",
		})
	}

	in.Type = "expense"

	log.Printf("[handler.transaction] CreateExpense: calling service user_id=%d req_id=%s idempotency_key=%s amount=%.2f account_id=%d title=%q",
		userID, reqID, idempotencyKey, in.Amount, in.AccountID, in.Title)

	detail, err := h.transactions.CreateExpense(c.Request().Context(), userID, in, idempotencyKey)
	if err != nil {
		log.Printf("[handler.transaction] CreateExpense: service error user_id=%d req_id=%s idempotency_key=%s error=%v",
			userID, reqID, idempotencyKey, err)
		return respondError(c, err)
	}

	log.Printf("[handler.transaction] CreateExpense: DONE user_id=%d req_id=%s transaction_id=%d idempotency_key=%s amount=%.2f entries=%d",
		userID, reqID, detail.Transaction.ID, idempotencyKey, -detail.Entries[0].Amount, len(detail.Entries))
	return c.JSON(http.StatusCreated, detail)
}

// CreateTransfer creates a transfer between two accounts
func (h *TransactionHandler) CreateTransfer(c echo.Context) error {
	userID := currentUserID(c)
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)

	log.Printf("[handler.transaction] CreateTransfer HIT: req_id=%s user_id=%d remote_addr=%s idempotency_key=%q",
		reqID, userID, c.Request().RemoteAddr, c.Request().Header.Get("Idempotency-Key"))

	var in models.TransferInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.transaction] CreateTransfer: bind failed user_id=%d req_id=%s error=%v", userID, reqID, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.transaction] CreateTransfer: bound input user_id=%d title=%q amount=%.2f from=%d to=%d date=%q",
		userID, in.Title, in.Amount, in.FromAccountID, in.ToAccountID, in.Date)

	idempotencyKey := c.Request().Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		log.Printf("[handler.transaction] CreateTransfer: missing Idempotency-Key header user_id=%d", userID)
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "Idempotency-Key header is required",
		})
	}

	detail, err := h.transactions.CreateTransfer(c.Request().Context(), userID, in, idempotencyKey)
	if err != nil {
		log.Printf("[handler.transaction] CreateTransfer: service error user_id=%d req_id=%s idempotency_key=%s error=%v",
			userID, reqID, idempotencyKey, err)
		return respondError(c, err)
	}

	log.Printf("[handler.transaction] CreateTransfer: DONE user_id=%d req_id=%s transaction_id=%d amount=%.2f from=%d to=%d",
		userID, reqID, detail.Transaction.ID, in.Amount, in.FromAccountID, in.ToAccountID)
	return c.JSON(http.StatusCreated, detail)
}

// Delete removes a transaction
func (h *TransactionHandler) Delete(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.transaction] Delete: user_id=%d raw_id=%q", userID, c.Param("id"))

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("[handler.transaction] Delete: invalid id user_id=%d raw=%q error=%v", userID, c.Param("id"), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}

	if err := h.transactions.Delete(c.Request().Context(), id, userID); err != nil {
		log.Printf("[handler.transaction] Delete: service error user_id=%d transaction_id=%d error=%v", userID, id, err)
		return respondError(c, err)
	}

	log.Printf("[handler.transaction] Delete: OK user_id=%d transaction_id=%d", userID, id)
	return c.JSON(http.StatusOK, echo.Map{"message": "deleted"})
}
