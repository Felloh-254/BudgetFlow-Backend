package handler

import (
	"log"
	"net/http"

	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type SummaryHandler struct {
	summary *service.SummaryService
}

func NewSummaryHandler(summary *service.SummaryService) *SummaryHandler {
	log.Println("[handler.summary] NewSummaryHandler: created")
	return &SummaryHandler{summary: summary}
}

func (h *SummaryHandler) Get(c echo.Context) error {
	userID := currentUserID(c)
	month := c.QueryParam("month")
	log.Printf("[handler.summary] Get: user_id=%d month=%q", userID, month)

	s, err := h.summary.Get(c.Request().Context(), userID, month)
	if err != nil {
		log.Printf("[handler.summary] Get: service error user_id=%d error=%v", userID, err)
		return respondError(c, err)
	}

	log.Printf("[handler.summary] Get: OK user_id=%d month=%q income=%.2f expenses=%.2f balance=%.2f budgets=%d monthly_points=%d",
		userID, month, s.TotalIncome, s.TotalExpenses, s.Balance, len(s.BudgetStats), len(s.MonthlyData))
	return c.JSON(http.StatusOK, s)
}
