package service

import (
	"context"
	"log"
	"time"

	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type SummaryService struct {
	summary *repository.SummaryRepository
}

func NewSummaryService(summary *repository.SummaryRepository) *SummaryService {
	log.Println("[service.summary] NewSummaryService: created")
	return &SummaryService{summary: summary}
}

func (s *SummaryService) Get(ctx context.Context, userID int, month string) (*models.Summary, error) {
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	log.Printf("[service.summary] Get: user_id=%d month=%q", userID, month)

	income, expense, err := s.summary.Totals(ctx, userID)
	if err != nil {
		log.Printf("[service.summary] Get: Totals failed user_id=%d error=%v", userID, err)
		return nil, err
	}
	log.Printf("[service.summary] Get: Totals OK user_id=%d income=%.2f expense=%.2f", userID, income, expense)

	stats, err := s.summary.BudgetStats(ctx, userID, month)
	if err != nil {
		log.Printf("[service.summary] Get: BudgetStats failed user_id=%d error=%v", userID, err)
		return nil, err
	}
	log.Printf("[service.summary] Get: BudgetStats OK user_id=%d month=%q count=%d", userID, month, len(stats))

	monthly, err := s.summary.MonthlyData(ctx, userID, 6)
	if err != nil {
		log.Printf("[service.summary] Get: MonthlyData failed user_id=%d error=%v", userID, err)
		return nil, err
	}
	log.Printf("[service.summary] Get: MonthlyData OK user_id=%d count=%d", userID, len(monthly))

	summary := &models.Summary{
		TotalIncome:   income,
		TotalExpenses: expense,
		Balance:       income - expense,
		BudgetStats:   stats,
		MonthlyData:   monthly,
	}

	log.Printf("[service.summary] Get: OK user_id=%d month=%q balance=%.2f", userID, month, summary.Balance)
	return summary, nil
}
