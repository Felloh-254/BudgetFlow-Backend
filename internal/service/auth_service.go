// Package service holds business logic: validation, orchestration across
// repositories, and translating low-level errors into apperr sentinels.
// Handlers stay thin and never touch a repository directly.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"budgetapp/internal/apperr"
	"budgetapp/internal/auth"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type AuthService struct {
	users  *repository.UserRepository
	tokens *auth.TokenManager
	log    *slog.Logger
}

func NewAuthService(users *repository.UserRepository, tokens *auth.TokenManager, log *slog.Logger) *AuthService {
	return &AuthService{
		users:  users,
		tokens: tokens,
		log:    log.With("component", "service.auth"),
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, name string) (*models.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)

	if email == "" || password == "" || name == "" {
		return nil, "", apperr.Validation("email, password, and name are required")
	}
	if len(password) < 8 {
		return nil, "", apperr.Validation("password must be at least 8 characters")
	}

	hash, err := auth.HashPassword(password, s.log)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}

	user, err := s.users.Create(ctx, email, hash, name)
	if err != nil {
		return nil, "", fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}

	s.log.InfoContext(ctx, "user registered",
		"user_id", user.ID,
		"email", user.Email,
	)
	return user, token, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		// Deliberately the same error whether the email doesn't exist or
		// the password is wrong don't leak which one it was.
		s.log.WarnContext(ctx, "login failed", "email", email, "reason", "user not found")
		return nil, "", apperr.ErrInvalidCredentials
	}

	if !auth.CheckPassword(password, user.PasswordHash) {
		s.log.WarnContext(ctx, "login failed", "email", email, "reason", "password mismatch")
		return nil, "", apperr.ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}

	s.log.InfoContext(ctx, "user logged in",
		"user_id", user.ID,
		"email", user.Email,
	)
	return user, token, nil
}

func (s *AuthService) GetByID(ctx context.Context, id int) (*models.User, error) {
	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user by id %d: %w", id, err)
	}
	return user, nil
}

func (s *AuthService) ResetPassword(ctx context.Context, email, newPassword string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	if newPassword == "" {
		return apperr.Validation("New password is required")
	}
	if len(newPassword) < 8 {
		return apperr.Validation("New password must be at least 8 characters")
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("reset password: find user %q: %w", email, err)
	}

	hash, err := auth.HashPassword(newPassword, s.log)
	if err != nil {
		return fmt.Errorf("reset password: hash password: %w", err)
	}

	err = s.users.ResetPassword(ctx, user.ID, hash)
	if err != nil {
		return fmt.Errorf("reset password: save: %w", err)
	}

	s.log.InfoContext(ctx, "password reset",
		"user_id", user.ID,
		"email", user.Email,
	)
	return nil
}

func (s *AuthService) ForgetPassword(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("forget password: find user %q: %w", email, err)
	}

	// Password reset token and email sending will be implemented here.
	s.log.InfoContext(ctx, "password reset requested",
		"user_id", user.ID,
		"email", user.Email,
	)
	return nil
}
