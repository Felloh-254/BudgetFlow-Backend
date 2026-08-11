// Package service holds business logic: validation, orchestration across
// repositories, and translating low-level errors into apperr sentinels.
// Handlers stay thin and never touch a repository directly.
package service

import (
	"context"
	"strings"

	"budgetapp/internal/apperr"
	"budgetapp/internal/auth"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type AuthService struct {
	users  *repository.UserRepository
	tokens *auth.TokenManager
}

func NewAuthService(users *repository.UserRepository, tokens *auth.TokenManager) *AuthService {
	return &AuthService{users: users, tokens: tokens}
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

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, "", err
	}

	user, err := s.users.Create(ctx, email, hash, name)
	if err != nil {
		return nil, "", err
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		// Deliberately the same error whether the email doesn't exist or
		// the password is wrong — don't leak which one it was.
		return nil, "", apperr.ErrInvalidCredentials
	}
	if !auth.CheckPassword(password, user.PasswordHash) {
		return nil, "", apperr.ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (s *AuthService) GetByID(ctx context.Context, id int) (*models.User, error) {
	return s.users.FindByID(ctx, id)
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
		return err
	}

	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.users.ResetPassword(ctx, user.ID, hash)
}

func (s *AuthService) ForgetPassword(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	_, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return err
	}

	// Here i will implement the generate a password reset token and send it via email.
	// For simplicity, i will just return nil to indicate success.
	return nil
}
