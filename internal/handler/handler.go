// Package handler is the HTTP layer only: bind request -> call service ->
// map result/error to a JSON response. No SQL, no business rules here.
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"budgetapp/internal/apperr"

	"github.com/labstack/echo/v4"
)

// respondError is the single place that translates a service-layer error
// into an HTTP status. Every handler funnels errors through this instead
// of each one re-deciding what a given error means.
func respondError(c echo.Context, err error) error {
	var (
		status int
		body   echo.Map
	)

	var vErr *apperr.ValidationError
	switch {
	case errors.As(err, &vErr):
		status = http.StatusBadRequest
		body = echo.Map{"error": vErr.Message}
	case errors.Is(err, apperr.ErrNotFound):
		status = http.StatusNotFound
		body = echo.Map{"error": "not found"}
	case errors.Is(err, apperr.ErrDuplicateEmail):
		status = http.StatusConflict
		body = echo.Map{"error": "email already registered"}
	case errors.Is(err, apperr.ErrInsufficientFunds):
		status = http.StatusConflict
		body = echo.Map{"error": apperr.ErrInsufficientFunds.Error()}
	case errors.Is(err, apperr.ErrInvalidCredentials):
		status = http.StatusUnauthorized
		body = echo.Map{"error": "invalid email or password"}
	case errors.Is(err, apperr.ErrForbidden):
		status = http.StatusForbidden
		body = echo.Map{"error": "forbidden"}
	case errors.Is(err, apperr.ErrAccountNameRequired),
		errors.Is(err, apperr.ErrInvalidAccountName),
		errors.Is(err, apperr.ErrInvalidAccountType),
		errors.Is(err, apperr.ErrUnsupportedAccountType),
		errors.Is(err, apperr.ErrInvalidBalance),
		errors.Is(err, apperr.ErrUnsupportedCurrency):
		status = http.StatusBadRequest
		body = echo.Map{"error": err.Error()}
	default:
		status = http.StatusInternalServerError
		body = echo.Map{"error": "internal server error"}
	}

	logError(c, status, err)
	return c.JSON(status, body)
}

// HTTPErrorHandler keeps framework-generated failures in the same JSON shape
// as handler/service errors, so clients can consistently read response.error.
func HTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	status := http.StatusInternalServerError
	message := "internal server error"
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		status = httpErr.Code
		if status < http.StatusInternalServerError {
			switch value := httpErr.Message.(type) {
			case string:
				message = value
			case error:
				message = value.Error()
			default:
				message = http.StatusText(status)
			}
		}
	}

	logError(c, status, err)
	_ = c.JSON(status, echo.Map{"error": message})
}

func respondClientError(c echo.Context, status int, message string, cause error) error {
	if cause != nil {
		logError(c, status, cause)
	}
	return c.JSON(status, echo.Map{"error": message})
}

func logError(c echo.Context, status int, err error) {
	userID, _ := c.Get("user_id").(int)
	ctx := c.Request().Context()
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)

	if status >= 500 {
		slog.ErrorContext(ctx, "request failed",
			"component", "handler",
			"status", status,
			"method", c.Request().Method,
			"path", c.Path(),
			"request_id", reqID,
			"user_id", userID,
			"error", err,
		)
	} else {
		slog.WarnContext(ctx, "client error",
			"component", "handler",
			"status", status,
			"method", c.Request().Method,
			"path", c.Path(),
			"request_id", reqID,
			"user_id", userID,
			"error", err,
		)
	}
}

func currentUserID(c echo.Context) int {
	uid, ok := c.Get("user_id").(int)
	if !ok {
		slog.WarnContext(c.Request().Context(), "user_id not found in context",
			"component", "handler",
			"value", c.Get("user_id"),
		)
		return 0
	}
	return uid
}
