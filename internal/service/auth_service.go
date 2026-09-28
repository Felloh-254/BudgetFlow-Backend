// Package service holds business logic: validation, orchestration across
// repositories, and translating low-level errors into apperr sentinels.
// Handlers stay thin and never touch a repository directly.
package service

import (
	"context"
	"log"
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
	log.Println("[service.auth] NewAuthService: created")
	return &AuthService{users: users, tokens: tokens}
}

func (s *AuthService) Register(ctx context.Context, email, password, name string) (*models.User, string, error) {
	log.Printf("[service.auth] Register: email=%q name=%q password_length=%d", email, name, len(password))

	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)

	if email == "" || password == "" || name == "" {
		log.Printf("[service.auth] Register: validation failed email=%q name=%q password_set=%v", email, name, password != "")
		return nil, "", apperr.Validation("email, password, and name are required")
	}
	if len(password) < 8 {
		log.Printf("[service.auth] Register: password too short length=%d", len(password))
		return nil, "", apperr.Validation("password must be at least 8 characters")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Printf("[service.auth] Register: hash failed email=%q error=%v", email, err)
		return nil, "", err
	}

	user, err := s.users.Create(ctx, email, hash, name)
	if err != nil {
		log.Printf("[service.auth] Register: user creation failed email=%q error=%v", email, err)
		return nil, "", err
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		log.Printf("[service.auth] Register: token generation failed user_id=%d error=%v", user.ID, err)
		return nil, "", err
	}

	log.Printf("[service.auth] Register: OK user_id=%d email=%q", user.ID, user.Email)
	return user, token, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*models.User, string, error) {
	log.Printf("[service.auth] Login: email=%q password_length=%d", email, len(password))

	email = strings.TrimSpace(strings.ToLower(email))

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		// Deliberately the same error whether the email doesn't exist or
		// the password is wrong — don't leak which one it was.
		log.Printf("[service.auth] Login: user not found email=%q (returning invalid credentials)", email)
		return nil, "", apperr.ErrInvalidCredentials
	}

	if !auth.CheckPassword(password, user.PasswordHash) {
		log.Printf("[service.auth] Login: password mismatch user_id=%d email=%q", user.ID, email)
		return nil, "", apperr.ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		log.Printf("[service.auth] Login: token generation failed user_id=%d error=%v", user.ID, err)
		return nil, "", err
	}

	log.Printf("[service.auth] Login: OK user_id=%d email=%q", user.ID, user.Email)
	return user, token, nil
}

func (s *AuthService) GetByID(ctx context.Context, id int) (*models.User, error) {
	log.Printf("[service.auth] GetByID: user_id=%d", id)

	user, err := s.users.FindByID(ctx, id)
	if err != nil {
		log.Printf("[service.auth] GetByID: repo error user_id=%d error=%v", id, err)
		return nil, err
	}

	log.Printf("[service.auth] GetByID: OK user_id=%d email=%q", user.ID, user.Email)
	return user, nil
}

func (s *AuthService) ResetPassword(ctx context.Context, email, newPassword string) error {
	log.Printf("[service.auth] ResetPassword: email=%q new_password_length=%d", email, len(newPassword))

	email = strings.TrimSpace(strings.ToLower(email))

	if newPassword == "" {
		log.Printf("[service.auth] ResetPassword: validation failed - empty password email=%q", email)
		return apperr.Validation("New password is required")
	}
	if len(newPassword) < 8 {
		log.Printf("[service.auth] ResetPassword: validation failed - password too short email=%q", email)
		return apperr.Validation("New password must be at least 8 characters")
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		log.Printf("[service.auth] ResetPassword: user not found email=%q error=%v", email, err)
		return err
	}

	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		log.Printf("[service.auth] ResetPassword: hash failed user_id=%d error=%v", user.ID, err)
		return err
	}

	err = s.users.ResetPassword(ctx, user.ID, hash)
	if err != nil {
		log.Printf("[service.auth] ResetPassword: repo error user_id=%d error=%v", user.ID, err)
		return err
	}

	log.Printf("[service.auth] ResetPassword: OK user_id=%d email=%q", user.ID, email)
	return nil
}

func (s *AuthService) ForgetPassword(ctx context.Context, email string) error {
	log.Printf("[service.auth] ForgetPassword: email=%q", email)

	email = strings.TrimSpace(strings.ToLower(email))

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		log.Printf("[service.auth] ForgetPassword: user not found email=%q error=%v", email, err)
		return err
	}

	// Here i will implement the generate a password reset token and send it via email.
	// For simplicity, i will just return nil to indicate success.
	log.Printf("[service.auth] ForgetPassword: OK user_id=%d email=%q (email would be sent)", user.ID, email)
	return nil
}
