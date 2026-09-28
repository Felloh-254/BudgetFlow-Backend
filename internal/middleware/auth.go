package middleware

import (
	"log"
	"net/http"
	"strings"

	"budgetapp/internal/auth"

	"github.com/labstack/echo/v4"
)

// JWT returns an echo middleware that validates the Authorization header
// and stashes the user id on the context for handlers to read.
func JWT(tokens *auth.TokenManager) echo.MiddlewareFunc {
	log.Println("[middleware.jwt] JWT middleware initialized")

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")

			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				log.Printf("[middleware.jwt] missing or malformed token: method=%s path=%s header=%q",
					c.Request().Method, c.Request().URL.Path, header)
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing token"})
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			log.Printf("[middleware.jwt] validating token: method=%s path=%s token_length=%d",
				c.Request().Method, c.Request().URL.Path, len(tokenStr))

			userID, err := tokens.Parse(tokenStr)
			if err != nil {
				log.Printf("[middleware.jwt] invalid token: method=%s path=%s error=%v",
					c.Request().Method, c.Request().URL.Path, err)
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid or expired token"})
			}

			log.Printf("[middleware.jwt] OK: method=%s path=%s user_id=%d",
				c.Request().Method, c.Request().URL.Path, userID)

			c.Set("user_id", userID)
			return next(c)
		}
	}
}
