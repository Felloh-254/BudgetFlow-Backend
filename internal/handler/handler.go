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
	logError(c, err)
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
	case errors.Is(err, apperr.ErrAccountNameRequired),
		errors.Is(err, apperr.ErrInvalidAccountName),
		errors.Is(err, apperr.ErrInvalidAccountType),
		errors.Is(err, apperr.ErrUnsupportedAccountType),
		errors.Is(err, apperr.ErrInvalidBalance),
		errors.Is(err, apperr.ErrUnsupportedCurrency):
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	default:
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "internal server error"})
	}
}

func respondClientError(c echo.Context, status int, message string, cause error) error {
	if cause != nil {
		logError(c, cause)
	}
	return c.JSON(status, echo.Map{"error": message})
}

func logError(c echo.Context, err error) {
	userID, _ := c.Get("user_id").(int)
	c.Logger().Errorf("request failed: method=%s path=%s request_id=%s user_id=%d error=%v", c.Request().Method, c.Path(), c.Response().Header().Get(echo.HeaderXRequestID), userID, err)
}

func currentUserID(c echo.Context) int {
	return c.Get("user_id").(int)
}
