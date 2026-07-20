package service

import (
	"context"

	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type SummaryService struct {
	summary *repository.SummaryRepository
}

func NewSummaryService(summary *repository.SummaryRepository) *SummaryService {
	return &SummaryService{summary: summary}
}

func (s *SummaryService) Get(ctx context.Context, userID int) (*models.Summary, error) {
	income, expense, err := s.summary.Totals(ctx, userID)
	if err != nil {
		return nil, err
	}

	stats, err := s.summary.BudgetStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	monthly, err := s.summary.MonthlyData(ctx, userID, 6)
	if err != nil {
		return nil, err
	}

	return &models.Summary{
		TotalIncome:   income,
		TotalExpenses: expense,
		Balance:       income - expense,
		BudgetStats:   stats,
		MonthlyData:   monthly,
	}, nil
}
