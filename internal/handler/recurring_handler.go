package handler

import (
	"log"
	"net/http"
	"strconv"

	"budgetapp/internal/models"
	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type RecurringHandler struct {
	recurring *service.RecurringService
}

func NewRecurringHandler(r *service.RecurringService) *RecurringHandler {
	log.Println("[handler.recurring] NewRecurringHandler: created")
	return &RecurringHandler{recurring: r}
}

func (h *RecurringHandler) List(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.recurring] List: user_id=%d", userID)
	rs, err := h.recurring.List(c.Request().Context(), userID)
	if err != nil {
		return respondError(c, err)
	}
	log.Printf("[handler.recurring] List: OK user_id=%d count=%d", userID, len(rs))
	return c.JSON(http.StatusOK, rs)
}

func (h *RecurringHandler) Create(c echo.Context) error {
	userID := currentUserID(c)
	var in models.RecurringRuleInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	log.Printf("[handler.recurring] Create: user_id=%d type=%s freq=%s amount=%.2f",
		userID, in.Type, in.Frequency, in.Amount)
	r, err := h.recurring.Create(c.Request().Context(), userID, in)
	if err != nil {
		return respondError(c, err)
	}
	log.Printf("[handler.recurring] Create: OK rule_id=%d next_run=%s", r.ID, r.NextRunAt)
	return c.JSON(http.StatusCreated, r)
}

func (h *RecurringHandler) Update(c echo.Context) error {
	userID := currentUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}

	var in models.RecurringRuleInput
	if err := c.Bind(&in); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	log.Printf("[handler.recurring] Update: user_id=%d rule_id=%d", userID, id)
	r, err := h.recurring.Update(c.Request().Context(), id, userID, in)
	if err != nil {
		return respondError(c, err)
	}
	log.Printf("[handler.recurring] Update: OK rule_id=%d", r.ID)
	return c.JSON(http.StatusOK, r)
}

func (h *RecurringHandler) SetActive(active bool) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := currentUserID(c)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
		}
		log.Printf("[handler.recurring] SetActive: user_id=%d rule_id=%d active=%v", userID, id, active)
		r, err := h.recurring.SetActive(c.Request().Context(), id, userID, active)
		if err != nil {
			return respondError(c, err)
		}
		log.Printf("[handler.recurring] SetActive: OK rule_id=%d active=%v", r.ID, r.Active)
		return c.JSON(http.StatusOK, r)
	}
}

func (h *RecurringHandler) Delete(c echo.Context) error {
	userID := currentUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}
	log.Printf("[handler.recurring] Delete: user_id=%d rule_id=%d", userID, id)
	if err := h.recurring.Delete(c.Request().Context(), id, userID); err != nil {
		return respondError(c, err)
	}
	log.Printf("[handler.recurring] Delete: OK rule_id=%d", id)
	return c.JSON(http.StatusOK, echo.Map{"message": "deleted"})
}

func (h *RecurringHandler) RunDue(c echo.Context) error {
	log.Printf("[handler.recurring] RunDue: manual trigger HIT remote_addr=%s", c.Request().RemoteAddr)
	success, failed, err := h.recurring.RunDue(c.Request().Context())
	if err != nil {
		log.Printf("[handler.recurring] RunDue: service error error=%v", err)
		return respondError(c, err)
	}
	log.Printf("[handler.recurring] RunDue: OK success=%d failed=%d", success, failed)
	return c.JSON(http.StatusOK, echo.Map{"success": success, "failed": failed})
}
