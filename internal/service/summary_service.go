package service

import (
	"context"
	"time"

	"log/slog"

	"budgetapp/internal/logger"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type SummaryService struct {
	summary *repository.SummaryRepository
	log     *slog.Logger
}

func NewSummaryService(summary *repository.SummaryRepository) *SummaryService {
	return &SummaryService{
		summary: summary,
		log:     logger.Logger,
	}
}

func (s *SummaryService) Get(
	ctx context.Context,
	userID int,
	month string,
) (*models.Summary, error) {

	if month == "" {
		month = time.Now().Format("2006-01")
	}

	income, expense, err := s.summary.Totals(ctx, userID)
	if err != nil {
		s.log.Error(
			"failed to get summary totals",
			"user_id", userID,
			"month", month,
			"error", err,
		)

		return nil, err
	}

	stats, err := s.summary.BudgetStats(ctx, userID, month)
	if err != nil {
		s.log.Error(
			"failed to get budget stats",
			"user_id", userID,
			"month", month,
			"error", err,
		)

		return nil, err
	}

	monthly, err := s.summary.MonthlyData(ctx, userID, 6)
	if err != nil {
		s.log.Error(
			"failed to get monthly data",
			"user_id", userID,
			"error", err,
		)

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
