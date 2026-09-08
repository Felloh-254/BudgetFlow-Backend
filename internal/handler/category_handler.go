package handler

import (
	"net/http"

	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type CategoryHandler struct {
	categories *service.CategoryService
}

func NewCategoryHandler(categories *service.CategoryService) *CategoryHandler {
	return &CategoryHandler{categories: categories}
}

// List returns all categories available to the user (both default and custom)
func (h *CategoryHandler) List(c echo.Context) error {
	cats, err := h.categories.List(c.Request().Context(), currentUserID(c))
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, cats)
}
