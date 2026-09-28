package handler

import (
	"log"
	"net/http"
	"strconv"

	"budgetapp/internal/models"
	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type BudgetHandler struct {
	budgets *service.BudgetService
}

func NewBudgetHandler(budgets *service.BudgetService) *BudgetHandler {
	log.Println("[handler.budget] NewBudgetHandler: created")
	return &BudgetHandler{budgets: budgets}
}

func (h *BudgetHandler) List(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.budget] List: user_id=%d", userID)

	budgets, err := h.budgets.List(c.Request().Context(), userID)
	if err != nil {
		log.Printf("[handler.budget] List: service error user_id=%d error=%v", userID, err)
		return respondError(c, err)
	}

	log.Printf("[handler.budget] List: OK user_id=%d count=%d", userID, len(budgets))
	return c.JSON(http.StatusOK, budgets)
}

func (h *BudgetHandler) Create(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.budget] Create: user_id=%d", userID)

	var in models.BudgetInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.budget] Create: bind failed user_id=%d error=%v", userID, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.budget] Create: bound input user_id=%d name=%q amount=%.2f category=%q",
		userID, in.Name, in.Amount, in.Category)

	b, err := h.budgets.Create(c.Request().Context(), userID, in)
	if err != nil {
		log.Printf("[handler.budget] Create: service error user_id=%d error=%v", userID, err)
		return respondError(c, err)
	}

	log.Printf("[handler.budget] Create: OK user_id=%d budget_id=%d name=%q", userID, b.ID, b.Name)
	return c.JSON(http.StatusCreated, b)
}

func (h *BudgetHandler) Update(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.budget] Update: user_id=%d raw_id=%q", userID, c.Param("id"))

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("[handler.budget] Update: invalid id user_id=%d raw=%q error=%v", userID, c.Param("id"), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}

	var in models.BudgetInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.budget] Update: bind failed user_id=%d budget_id=%d error=%v", userID, id, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.budget] Update: bound input user_id=%d budget_id=%d name=%q amount=%.2f",
		userID, id, in.Name, in.Amount)

	b, err := h.budgets.Update(c.Request().Context(), id, userID, in)
	if err != nil {
		log.Printf("[handler.budget] Update: service error user_id=%d budget_id=%d error=%v", userID, id, err)
		return respondError(c, err)
	}

	log.Printf("[handler.budget] Update: OK user_id=%d budget_id=%d", userID, id)
	return c.JSON(http.StatusOK, b)
}

func (h *BudgetHandler) Delete(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.budget] Delete: user_id=%d raw_id=%q", userID, c.Param("id"))

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("[handler.budget] Delete: invalid id user_id=%d raw=%q error=%v", userID, c.Param("id"), err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}

	if err := h.budgets.Delete(c.Request().Context(), id, userID); err != nil {
		log.Printf("[handler.budget] Delete: service error user_id=%d budget_id=%d error=%v", userID, id, err)
		return respondError(c, err)
	}

	log.Printf("[handler.budget] Delete: OK user_id=%d budget_id=%d", userID, id)
	return c.JSON(http.StatusOK, echo.Map{"message": "deleted"})
}
