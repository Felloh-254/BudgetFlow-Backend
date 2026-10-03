package handler

import (
	"log/slog"
	"net/http"

	"budgetapp/internal/models"
	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	auth *service.AuthService
	log  *slog.Logger
}

func NewAuthHandler(auth *service.AuthService, log *slog.Logger) *AuthHandler {
	return &AuthHandler{
		auth: auth,
		log:  log.With("component", "handler.auth"),
	}
}

func (h *AuthHandler) Register(c echo.Context) error {
	ctx := c.Request().Context()
	h.log.InfoContext(ctx, "register request received", "remote_addr", c.Request().RemoteAddr)

	var req models.RegisterRequest
	if err := c.Bind(&req); err != nil {
		h.log.WarnContext(ctx, "register request body invalid", "error", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	h.log.DebugContext(ctx, "register request bound", "email", req.Email, "name", req.Name)

	user, token, err := h.auth.Register(c.Request().Context(), req.Email, req.Password, req.Name)
	if err != nil {
		h.log.WarnContext(ctx, "register failed", "email", req.Email, "error", err)
		return respondError(c, err)
	}

	h.log.InfoContext(ctx, "register succeeded", "user_id", user.ID, "email", user.Email)
	return c.JSON(http.StatusCreated, models.AuthResponse{Token: token, User: user.Public()})
}

func (h *AuthHandler) Login(c echo.Context) error {
	ctx := c.Request().Context()
	h.log.InfoContext(ctx, "login request received", "remote_addr", c.Request().RemoteAddr)

	var req models.LoginRequest
	if err := c.Bind(&req); err != nil {
		h.log.WarnContext(ctx, "login request body invalid", "error", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	h.log.DebugContext(ctx, "login request bound", "email", req.Email)

	user, token, err := h.auth.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		h.log.WarnContext(ctx, "login failed", "email", req.Email, "error", err)
		return respondError(c, err)
	}

	h.log.InfoContext(ctx, "login succeeded", "user_id", user.ID, "email", user.Email)
	return c.JSON(http.StatusOK, models.AuthResponse{Token: token, User: user.Public()})
}

func (h *AuthHandler) Me(c echo.Context) error {
	userID := currentUserID(c)
	ctx := c.Request().Context()
	h.log.DebugContext(ctx, "current user requested", "user_id", userID)

	user, err := h.auth.GetByID(c.Request().Context(), userID)
	if err != nil {
		h.log.WarnContext(ctx, "current user lookup failed", "user_id", userID, "error", err)
		return respondError(c, err)
	}

	h.log.DebugContext(ctx, "current user returned", "user_id", user.ID)
	return c.JSON(http.StatusOK, user.Public())
}

func (h *AuthHandler) Logout(c echo.Context) error {
	userID := currentUserID(c)
	h.log.InfoContext(c.Request().Context(), "logout requested", "user_id", userID)

	// Server side logout logic can be implemented here if needed, such as invalidating tokens or clearing session data.
	// For this example, we'll just return a success response.
	return c.JSON(http.StatusOK, echo.Map{"message": "logged out successfully"})
}

func (h *AuthHandler) ResetPassword(c echo.Context) error {
	ctx := c.Request().Context()
	h.log.InfoContext(ctx, "password reset request received", "remote_addr", c.Request().RemoteAddr)

	var req models.ResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		h.log.WarnContext(ctx, "password reset request body invalid", "error", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	h.log.DebugContext(ctx, "password reset request bound", "email", req.Email)

	err := h.auth.ResetPassword(c.Request().Context(), req.Email, req.NewPassword)
	if err != nil {
		h.log.WarnContext(ctx, "password reset failed", "email", req.Email, "error", err)
		return respondError(c, err)
	}

	h.log.InfoContext(ctx, "password reset succeeded", "email", req.Email)
	return c.JSON(http.StatusOK, echo.Map{"message": "password reset successfully"})
}

func (h *AuthHandler) ForgotPassword(c echo.Context) error {
	ctx := c.Request().Context()
	h.log.InfoContext(ctx, "forgot password request received", "remote_addr", c.Request().RemoteAddr)

	var req models.ForgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		h.log.WarnContext(ctx, "forgot password request body invalid", "error", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	h.log.DebugContext(ctx, "forgot password request bound", "email", req.Email)

	err := h.auth.ForgetPassword(c.Request().Context(), req.Email)
	if err != nil {
		h.log.WarnContext(ctx, "forgot password request failed", "email", req.Email, "error", err)
		return respondError(c, err)
	}

	h.log.InfoContext(ctx, "forgot password request accepted", "email", req.Email)
	return c.JSON(http.StatusOK, echo.Map{"message": "password reset email sent successfully"})
}
