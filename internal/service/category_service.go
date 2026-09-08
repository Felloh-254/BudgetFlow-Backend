package service

import (
	"context"

	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type CategoryService struct {
	categories *repository.CategoryRepository
}

func NewCategoryService(categories *repository.CategoryRepository) *CategoryService {
	return &CategoryService{categories: categories}
}

func (s *CategoryService) List(ctx context.Context, userID int) ([]models.Category, error) {
	return s.categories.ListByUser(ctx, userID)
}
