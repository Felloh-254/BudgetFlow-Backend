// Package handler is the HTTP layer only: bind request -> call service ->
// map result/error to a JSON response. No SQL, no business rules here.
package handler

import (
	"errors"
	"net/http"

	"budgetapp/internal/apperr"

	"github.com/labstack/echo/v4"
)

// respondError is the single place that translates a service-layer error
// into an HTTP status. Every handler funnels errors through this instead
// of each one re-deciding what a given error means.
func respondError(c echo.Context, err error) error {
	var vErr *apperr.ValidationError
	switch {
	case errors.As(err, &vErr):
		return c.JSON(http.StatusBadRequest, echo.Map{"error": vErr.Message})
	case errors.Is(err, apperr.ErrNotFound):
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not found"})
	case errors.Is(err, apperr.ErrDuplicateEmail):
		return c.JSON(http.StatusConflict, echo.Map{"error": "email already registered"})
	case errors.Is(err, apperr.ErrInvalidCredentials):
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid email or password"})
	case errors.Is(err, apperr.ErrForbidden):
		return c.JSON(http.StatusForbidden, echo.Map{"error": "forbidden"})
	default:
		c.Logger().Error(err)
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal server error"})
	}
}

func currentUserID(c echo.Context) int {
	return c.Get("user_id").(int)
}
