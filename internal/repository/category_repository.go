package repository

import (
	"context"
	"errors"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(db *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{db: db}
}

// FindOrCreate looks up a category by name+type, preferring one the user
// owns over a global default, and creates a user-owned one if neither
// exists. This is what replaces the old free-text `category` column: the
// API still accepts a plain string, but it's resolved to a real row here.
func (r *CategoryRepository) FindOrCreate(ctx context.Context, userID int, name, catType string) (*models.Category, error) {
	var c models.Category
	err := r.db.QueryRow(ctx,
		`SELECT id, user_id, name, type, color, created_at
		 FROM categories
		 WHERE name = $1 AND type = $2 AND (user_id = $3 OR user_id IS NULL)
		 ORDER BY user_id NULLS LAST
		 LIMIT 1`,
		name, catType, userID,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Color, &c.CreatedAt)
	if err == nil {
		return &c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	err = r.db.QueryRow(ctx,
		`INSERT INTO categories (user_id, name, type)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, name, type, color, created_at`,
		userID, name, catType,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Color, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CategoryRepository) ListByUser(ctx context.Context, userID int) ([]models.Category, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, name, type, color, created_at
		 FROM categories WHERE user_id = $1 OR user_id IS NULL
		 ORDER BY name`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Color, &c.CreatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}
