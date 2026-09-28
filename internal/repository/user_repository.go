package repository

import (
	"context"
	"errors"
	"log"

	"budgetapp/internal/apperr"
	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	log.Println("[repo.user] NewUserRepository: created")
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash, name string) (*models.User, error) {
	log.Printf("[repo.user] Create: email=%q name=%q", email, name)

	var u models.User
	err := r.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, name)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, password_hash, name, created_at`,
		email, passwordHash, name,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			log.Printf("[repo.user] Create: duplicate email email=%q", email)
			return nil, apperr.ErrDuplicateEmail
		}
		log.Printf("[repo.user] Create: FAILED email=%q error=%v", email, err)
		return nil, err
	}

	log.Printf("[repo.user] Create: OK user_id=%d email=%q", u.ID, u.Email)
	return &u, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	log.Printf("[repo.user] FindByEmail: email=%q", email)

	var u models.User
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, name, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[repo.user] FindByEmail: not found email=%q", email)
			return nil, apperr.ErrNotFound
		}
		log.Printf("[repo.user] FindByEmail: FAILED email=%q error=%v", email, err)
		return nil, err
	}

	log.Printf("[repo.user] FindByEmail: OK user_id=%d email=%q", u.ID, u.Email)
	return &u, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int) (*models.User, error) {
	log.Printf("[repo.user] FindByID: user_id=%d", id)

	var u models.User
	err := r.db.QueryRow(ctx,
		`SELECT id, email, password_hash, name, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[repo.user] FindByID: not found user_id=%d", id)
			return nil, apperr.ErrNotFound
		}
		log.Printf("[repo.user] FindByID: FAILED user_id=%d error=%v", id, err)
		return nil, err
	}

	log.Printf("[repo.user] FindByID: OK user_id=%d email=%q", u.ID, u.Email)
	return &u, nil
}

func (r *UserRepository) ResetPassword(ctx context.Context, id int, newPasswordHash string) error {
	log.Printf("[repo.user] ResetPassword: user_id=%d", id)

	result, err := r.db.Exec(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2`,
		newPasswordHash, id,
	)
	if err != nil {
		log.Printf("[repo.user] ResetPassword: FAILED user_id=%d error=%v", id, err)
		return err
	}
	if result.RowsAffected() == 0 {
		log.Printf("[repo.user] ResetPassword: not found user_id=%d", id)
		return apperr.ErrNotFound
	}

	log.Printf("[repo.user] ResetPassword: OK user_id=%d", id)
	return nil
}
