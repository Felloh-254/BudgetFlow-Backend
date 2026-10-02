package service

import (
	"context"
	"fmt"
	"log/slog"

	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type CategoryService struct {
	categories *repository.CategoryRepository
	log        *slog.Logger
}

func NewCategoryService(categories *repository.CategoryRepository, log *slog.Logger) *CategoryService {
	return &CategoryService{
		categories: categories,
		log:        log.With("component", "service.category"),
	}
}

func (s *CategoryService) List(ctx context.Context, userID int) ([]models.Category, error) {
	cats, err := s.categories.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories (user=%d): %w", userID, err)
	}
	return cats, nil
}
