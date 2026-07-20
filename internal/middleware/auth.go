package middleware

import (
	"net/http"
	"strings"

	"budgetapp/internal/auth"

	"github.com/labstack/echo/v4"
)

// JWT returns an echo middleware that validates the Authorization header
// and stashes the user id on the context for handlers to read.
func JWT(tokens *auth.TokenManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing token"})
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")

			userID, err := tokens.Parse(tokenStr)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid or expired token"})
			}

			c.Set("user_id", userID)
			return next(c)
		}
	}
}
