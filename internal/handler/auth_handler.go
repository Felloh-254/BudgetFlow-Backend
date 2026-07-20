package handler

import (
	"net/http"

	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler {
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

func (h *AuthHandler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	user, token, err := h.auth.Register(c.Request().Context(), req.Email, req.Password, req.Name)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusCreated, authResponse{Token: token, User: user.Public()})
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request body"})
	}

	user, token, err := h.auth.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, authResponse{Token: token, User: user.Public()})
}

func (h *AuthHandler) Me(c echo.Context) error {
	user, err := h.auth.GetByID(c.Request().Context(), currentUserID(c))
	if err != nil {
		return respondError(c, err)
	}
	return c.JSON(http.StatusOK, user.Public())
}
