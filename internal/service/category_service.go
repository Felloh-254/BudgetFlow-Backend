package service

import (
	"context"
	"log"

	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type CategoryService struct {
	categories *repository.CategoryRepository
}

func NewCategoryService(categories *repository.CategoryRepository) *CategoryService {
	log.Println("[service.category] NewCategoryService: created")
	return &CategoryService{categories: categories}
}

func (s *CategoryService) List(ctx context.Context, userID int) ([]models.Category, error) {
	log.Printf("[service.category] List: user_id=%d", userID)

	cats, err := s.categories.ListByUser(ctx, userID)
	if err != nil {
		log.Printf("[service.category] List: repo error user_id=%d error=%v", userID, err)
		return nil, err
	}

	log.Printf("[service.category] List: OK user_id=%d count=%d", userID, len(cats))
	return cats, nil
}
