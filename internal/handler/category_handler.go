package handler

import (
	"log"
	"net/http"

	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type CategoryHandler struct {
	categories *service.CategoryService
}

func NewCategoryHandler(categories *service.CategoryService) *CategoryHandler {
	log.Println("[handler.category] NewCategoryHandler: created")
	return &CategoryHandler{categories: categories}
}

// List returns all categories available to the user (both default and custom)
func (h *CategoryHandler) List(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.category] List: user_id=%d", userID)

	cats, err := h.categories.List(c.Request().Context(), userID)
	if err != nil {
		log.Printf("[handler.category] List: service error user_id=%d error=%v", userID, err)
		return respondError(c, err)
	}

	log.Printf("[handler.category] List: OK user_id=%d count=%d", userID, len(cats))
	return c.JSON(http.StatusOK, cats)
}
