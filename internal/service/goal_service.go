package service

import (
	"context"
	"log"
	"strings"

	"budgetapp/internal/apperr"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type GoalService struct {
	goals *repository.GoalRepository
}

func NewGoalService(goals *repository.GoalRepository) *GoalService {
	log.Println("[service.goal] NewGoalService: created")
	return &GoalService{goals: goals}
}

func (s *GoalService) List(ctx context.Context, userID int) ([]models.Goal, error) {
	log.Printf("[service.goal] List: user_id=%d", userID)
	goals, err := s.goals.ListByUser(ctx, userID)
	if err != nil {
		log.Printf("[service.goal] List: repo error user_id=%d error=%v", userID, err)
		return nil, err
	}
	log.Printf("[service.goal] List: OK user_id=%d count=%d", userID, len(goals))
	return goals, nil
}

func (s *GoalService) Create(ctx context.Context, userID int, in models.GoalInput) (*models.Goal, error) {
	log.Printf("[service.goal] Create: user_id=%d name=%q target=%.2f", userID, in.Name, in.TargetAmount)
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, apperr.Validation("goal name is required")
	}
	if in.TargetAmount <= 0 {
		return nil, apperr.Validation("target_amount must be greater than 0")
	}
	if in.Color == "" {
		in.Color = "#10b981"
	}
	g, err := s.goals.Create(ctx, userID, in)
	if err != nil {
		log.Printf("[service.goal] Create: repo error user_id=%d error=%v", userID, err)
		return nil, err
	}
	log.Printf("[service.goal] Create: OK user_id=%d goal_id=%d", userID, g.ID)
	return g, nil
}

func (s *GoalService) Update(ctx context.Context, id, userID int, in models.GoalInput) (*models.Goal, error) {
	log.Printf("[service.goal] Update: goal_id=%d user_id=%d", id, userID)
	if strings.TrimSpace(in.Name) == "" {
		return nil, apperr.Validation("goal name is required")
	}
	if in.TargetAmount <= 0 {
		return nil, apperr.Validation("target_amount must be greater than 0")
	}
	if in.Color == "" {
		in.Color = "#10b981"
	}
	g, err := s.goals.Update(ctx, id, userID, in)
	if err != nil {
		return nil, err
	}
	log.Printf("[service.goal] Update: OK goal_id=%d", id)
	return g, nil
}

func (s *GoalService) Contribute(ctx context.Context, id, userID int, amount float64) (*models.Goal, error) {
	log.Printf("[service.goal] Contribute: goal_id=%d user_id=%d amount=%.2f", id, userID, amount)
	if amount <= 0 {
		return nil, apperr.Validation("amount must be greater than 0")
	}
	g, err := s.goals.Contribute(ctx, id, userID, amount)
	if err != nil {
		log.Printf("[service.goal] Contribute: repo error goal_id=%d error=%v", id, err)
		return nil, err
	}
	log.Printf("[service.goal] Contribute: OK goal_id=%d status=%s progress=%.1f", id, g.Status, g.Progress)
	return g, nil
}

func (s *GoalService) Delete(ctx context.Context, id, userID int) error {
	log.Printf("[service.goal] Delete: goal_id=%d user_id=%d", id, userID)
	ok, err := s.goals.Delete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrNotFound
	}
	log.Printf("[service.goal] Delete: OK goal_id=%d", id)
	return nil
}
