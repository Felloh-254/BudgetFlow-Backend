package handler

import (
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
	return &BudgetHandler{budgets: budgets}
}

func (h *BudgetHandler) List(c echo.Context) error {
	budgets, err := h.budgets.List(c.Request().Context(), currentUserID(c))
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, budgets)
}

func (h *BudgetHandler) Create(c echo.Context) error {
	var in models.BudgetInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	b, err := h.budgets.Create(c.Request().Context(), currentUserID(c), in)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, b)
}

func (h *BudgetHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}
	var in models.BudgetInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	b, err := h.budgets.Update(c.Request().Context(), id, currentUserID(c), in)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, b)
}

func (h *BudgetHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}
	if err := h.budgets.Delete(c.Request().Context(), id, currentUserID(c)); err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"message": "deleted"})
}
