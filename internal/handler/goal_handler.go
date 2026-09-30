package handler

import (
	"log"
	"net/http"
	"strconv"

	"budgetapp/internal/models"
	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type GoalHandler struct {
	goals *service.GoalService
}

func NewGoalHandler(goals *service.GoalService) *GoalHandler {
	log.Println("[handler.goal] NewGoalHandler: created")
	return &GoalHandler{goals: goals}
}

func (h *GoalHandler) List(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.goal] List: user_id=%d", userID)

	goals, err := h.goals.List(c.Request().Context(), userID)
	if err != nil {
		log.Printf("[handler.goal] List: service error user_id=%d error=%v", userID, err)
		return respondError(c, err)
	}
	log.Printf("[handler.goal] List: OK user_id=%d count=%d", userID, len(goals))
	return c.JSON(http.StatusOK, goals)
}

func (h *GoalHandler) Create(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.goal] Create: user_id=%d", userID)

	var in models.GoalInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.goal] Create: bind failed user_id=%d error=%v", userID, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	log.Printf("[handler.goal] Create: bound input user_id=%d name=%q target=%.2f",
		userID, in.Name, in.TargetAmount)

	g, err := h.goals.Create(c.Request().Context(), userID, in)
	if err != nil {
		log.Printf("[handler.goal] Create: service error user_id=%d error=%v", userID, err)
		return respondError(c, err)
	}
	log.Printf("[handler.goal] Create: OK user_id=%d goal_id=%d", userID, g.ID)
	return c.JSON(http.StatusCreated, g)
}

func (h *GoalHandler) Update(c echo.Context) error {
	userID := currentUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("[handler.goal] Update: invalid id user_id=%d raw=%q", userID, c.Param("id"))
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}

	var in models.GoalInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.goal] Update: bind failed user_id=%d goal_id=%d error=%v", userID, id, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	log.Printf("[handler.goal] Update: user_id=%d goal_id=%d name=%q target=%.2f",
		userID, id, in.Name, in.TargetAmount)

	g, err := h.goals.Update(c.Request().Context(), id, userID, in)
	if err != nil {
		log.Printf("[handler.goal] Update: service error user_id=%d goal_id=%d error=%v", userID, id, err)
		return respondError(c, err)
	}
	log.Printf("[handler.goal] Update: OK user_id=%d goal_id=%d", userID, id)
	return c.JSON(http.StatusOK, g)
}

func (h *GoalHandler) Contribute(c echo.Context) error {
	userID := currentUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("[handler.goal] Contribute: invalid id user_id=%d raw=%q", userID, c.Param("id"))
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}

	var in models.GoalContributeInput
	if err := c.Bind(&in); err != nil {
		log.Printf("[handler.goal] Contribute: bind failed user_id=%d goal_id=%d error=%v", userID, id, err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}
	log.Printf("[handler.goal] Contribute: user_id=%d goal_id=%d amount=%.2f", userID, id, in.Amount)

	g, err := h.goals.Contribute(c.Request().Context(), id, userID, in.Amount)
	if err != nil {
		log.Printf("[handler.goal] Contribute: service error user_id=%d goal_id=%d error=%v", userID, id, err)
		return respondError(c, err)
	}
	log.Printf("[handler.goal] Contribute: OK user_id=%d goal_id=%d new=%.2f status=%s",
		userID, id, g.CurrentAmount, g.Status)
	return c.JSON(http.StatusOK, g)
}

func (h *GoalHandler) Delete(c echo.Context) error {
	userID := currentUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		log.Printf("[handler.goal] Delete: invalid id user_id=%d raw=%q", userID, c.Param("id"))
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid id"})
	}
	log.Printf("[handler.goal] Delete: user_id=%d goal_id=%d", userID, id)

	if err := h.goals.Delete(c.Request().Context(), id, userID); err != nil {
		log.Printf("[handler.goal] Delete: service error user_id=%d goal_id=%d error=%v", userID, id, err)
		return respondError(c, err)
	}
	log.Printf("[handler.goal] Delete: OK user_id=%d goal_id=%d", userID, id)
	return c.JSON(http.StatusOK, echo.Map{"message": "deleted"})
}
