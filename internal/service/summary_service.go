package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type SummaryService struct {
	summary *repository.SummaryRepository
	log     *slog.Logger
}

func NewSummaryService(summary *repository.SummaryRepository, log *slog.Logger) *SummaryService {
	return &SummaryService{
		summary: summary,
		log:     log.With("component", "service.summary"),
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

	s.log.DebugContext(ctx, "building summary", "user_id", userID, "month", month)

	// Errors are wrapped with context and returned, not logged here.
	// The handler / Echo error middleware logs them once at the boundary.
	income, expense, err := s.summary.Totals(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("summary totals (user=%d): %w", userID, err)
	}

	stats, err := s.summary.BudgetStats(ctx, userID, month)
	if err != nil {
		return nil, fmt.Errorf("budget stats (user=%d, month=%s): %w", userID, month, err)
	}

	monthly, err := s.summary.MonthlyData(ctx, userID, 6)
	if err != nil {
		return nil, fmt.Errorf("monthly data (user=%d): %w", userID, err)
	}

	return &models.Summary{
		TotalIncome:   income,
		TotalExpenses: expense,
		Balance:       income - expense,
		BudgetStats:   stats,
		MonthlyData:   monthly,
	}, nil
}
