package handler

import (
	"net/http"

	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type SummaryHandler struct {
	summary *service.SummaryService
}

func NewSummaryHandler(summary *service.SummaryService) *SummaryHandler {
	return &SummaryHandler{summary: summary}
}

func (h *SummaryHandler) Get(c echo.Context) error {
	s, err := h.summary.Get(c.Request().Context(), currentUserID(c))
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, s)
}
