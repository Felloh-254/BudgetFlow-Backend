package handler

import (
	"log"
	"net/http"

	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
	log.Println("[handler.auth] NewAuthHandler: created")
	return &AuthHandler{auth: auth}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  interface{} `json:"user"`
}

type resetPasswordRequest struct {
	Email       string `json:"email"`
	NewPassword string `json:"new_password"`
}

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

func (h *AuthHandler) Register(c echo.Context) error {
	log.Printf("[handler.auth] Register: HIT remote_addr=%s", c.Request().RemoteAddr)

	var req registerRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("[handler.auth] Register: bind failed error=%v", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.auth] Register: binding OK email=%q name=%q password_length=%d",
		req.Email, req.Name, len(req.Password))

	user, token, err := h.auth.Register(c.Request().Context(), req.Email, req.Password, req.Name)
	if err != nil {
		log.Printf("[handler.auth] Register: service error email=%q error=%v", req.Email, err)
		return respondError(c, err)
	}

	log.Printf("[handler.auth] Register: OK user_id=%d email=%q", user.ID, user.Email)
	return c.JSON(http.StatusCreated, authResponse{Token: token, User: user.Public()})
}

func (h *AuthHandler) Login(c echo.Context) error {
	log.Printf("[handler.auth] Login: HIT remote_addr=%s", c.Request().RemoteAddr)

	var req loginRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("[handler.auth] Login: bind failed error=%v", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.auth] Login: binding OK email=%q", req.Email)

	user, token, err := h.auth.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		log.Printf("[handler.auth] Login: service error email=%q error=%v", req.Email, err)
		return respondError(c, err)
	}

	log.Printf("[handler.auth] Login: OK user_id=%d email=%q", user.ID, user.Email)
	return c.JSON(http.StatusOK, authResponse{Token: token, User: user.Public()})
}

func (h *AuthHandler) Me(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.auth] Me: user_id=%d", userID)

	user, err := h.auth.GetByID(c.Request().Context(), userID)
	if err != nil {
		log.Printf("[handler.auth] Me: service error user_id=%d error=%v", userID, err)
		return respondError(c, err)
	}

	log.Printf("[handler.auth] Me: OK user_id=%d email=%q", user.ID, user.Email)
	return c.JSON(http.StatusOK, user.Public())
}

func (h *AuthHandler) Logout(c echo.Context) error {
	userID := currentUserID(c)
	log.Printf("[handler.auth] Logout: user_id=%d (no server-side action)", userID)

	// Server side logout logic can be implemented here if needed, such as invalidating tokens or clearing session data.
	// For this example, we'll just return a success response.
	return c.JSON(http.StatusOK, echo.Map{"message": "logged out successfully"})
}

func (h *AuthHandler) ResetPassword(c echo.Context) error {
	log.Printf("[handler.auth] ResetPassword: HIT remote_addr=%s", c.Request().RemoteAddr)

	var req resetPasswordRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("[handler.auth] ResetPassword: bind failed error=%v", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.auth] ResetPassword: binding OK email=%q new_password_length=%d",
		req.Email, len(req.NewPassword))

	err := h.auth.ResetPassword(c.Request().Context(), req.Email, req.NewPassword)
	if err != nil {
		log.Printf("[handler.auth] ResetPassword: service error email=%q error=%v", req.Email, err)
		return respondError(c, err)
	}

	log.Printf("[handler.auth] ResetPassword: OK email=%q", req.Email)
	return c.JSON(http.StatusOK, echo.Map{"message": "password reset successfully"})
}

func (h *AuthHandler) ForgotPassword(c echo.Context) error {
	log.Printf("[handler.auth] ForgotPassword: HIT remote_addr=%s", c.Request().RemoteAddr)

	var req forgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("[handler.auth] ForgotPassword: bind failed error=%v", err)
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	log.Printf("[handler.auth] ForgotPassword: binding OK email=%q", req.Email)

	err := h.auth.ForgetPassword(c.Request().Context(), req.Email)
	if err != nil {
		log.Printf("[handler.auth] ForgotPassword: service error email=%q error=%v", req.Email, err)
		return respondError(c, err)
	}

	log.Printf("[handler.auth] ForgotPassword: OK email=%q (email would be sent)", req.Email)
	return c.JSON(http.StatusOK, echo.Map{"message": "password reset email sent successfully"})
}
