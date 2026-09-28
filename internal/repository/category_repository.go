package repository

import (
	"context"
	"errors"
	"log"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryRepository struct {
	db *pgxpool.Pool
}

func NewCategoryRepository(db *pgxpool.Pool) *CategoryRepository {
	log.Println("[repo.category] NewCategoryRepository: created")
	return &CategoryRepository{db: db}
}

// FindOrCreate looks up a category by name+type, preferring one the user
// owns over a global default, and creates a user-owned one if neither exists.
func (r *CategoryRepository) FindOrCreate(ctx context.Context, userID int, name, catType string) (*models.Category, error) {
	log.Printf("[repo.category] FindOrCreate: user_id=%d name=%q type=%q", userID, name, catType)

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
		log.Printf("[repo.category] FindOrCreate: found existing category_id=%d name=%q", c.ID, c.Name)
		return &c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("[repo.category] FindOrCreate: lookup failed user_id=%d error=%v", userID, err)
		return nil, err
	}

	log.Printf("[repo.category] FindOrCreate: creating new category user_id=%d name=%q type=%q", userID, name, catType)

	err = r.db.QueryRow(ctx,
		`INSERT INTO categories (user_id, name, type)
		 VALUES ($1, $2, $3)
		 RETURNING id, user_id, name, type, color, created_at`,
		userID, name, catType,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Color, &c.CreatedAt)
	if err != nil {
		log.Printf("[repo.category] FindOrCreate: insert failed user_id=%d name=%q error=%v", userID, name, err)
		return nil, err
	}

	log.Printf("[repo.category] FindOrCreate: OK created category_id=%d name=%q", c.ID, c.Name)
	return &c, nil
}

func (r *CategoryRepository) ListByUser(ctx context.Context, userID int) ([]models.Category, error) {
	log.Printf("[repo.category] ListByUser: user_id=%d", userID)

	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, name, type, color, created_at
		 FROM categories WHERE user_id = $1 OR user_id IS NULL
		 ORDER BY name`,
		userID,
	)
	if err != nil {
		log.Printf("[repo.category] ListByUser: query failed user_id=%d error=%v", userID, err)
		return nil, err
	}
	defer rows.Close()

	categories := []models.Category{}
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.Color, &c.CreatedAt); err != nil {
			log.Printf("[repo.category] ListByUser: scan failed user_id=%d error=%v", userID, err)
			return nil, err
		}
		categories = append(categories, c)
	}

	log.Printf("[repo.category] ListByUser: OK user_id=%d count=%d", userID, len(categories))
	return categories, rows.Err()
}
