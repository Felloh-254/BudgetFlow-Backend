package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"budgetapp/internal/apperr"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"
)

type RecurringService struct {
	rules      *repository.RecurringRepository
	categories *repository.CategoryRepository
	txns       *TransactionService
	log        *slog.Logger
}

func NewRecurringService(
	rules *repository.RecurringRepository,
	categories *repository.CategoryRepository,
	txns *TransactionService,
	log *slog.Logger,
) *RecurringService {
	return &RecurringService{
		rules:      rules,
		categories: categories,
		txns:       txns,
		log:        log.With("component", "service.recurring"),
	}
}

func (s *RecurringService) List(ctx context.Context, userID int) ([]models.RecurringRule, error) {
	rs, err := s.rules.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list recurring rules (user=%d): %w", userID, err)
	}
	return rs, nil
}

func (s *RecurringService) Create(ctx context.Context, userID int, in models.RecurringRuleInput) (*models.RecurringRule, error) {
	if err := validateRecurringInput(in); err != nil {
		return nil, err
	}
	if in.IntervalCount <= 0 {
		in.IntervalCount = 1
	}
	if in.StartDate == "" {
		in.StartDate = time.Now().Format("2006-01-02")
	}

	var catID *int
	if in.Category != "" {
		catType := in.Type
		if catType == "transfer" {
			catType = "expense"
		}
		cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), catType)
		if err != nil {
			return nil, fmt.Errorf("find/create category: %w", err)
		}
		catID = &cat.ID
	}

	r, err := s.rules.Create(ctx, userID, in, catID)
	if err != nil {
		return nil, fmt.Errorf("create recurring rule: %w", err)
	}

	s.log.InfoContext(ctx, "recurring rule created",
		"user_id", userID,
		"rule_id", r.ID,
		"type", r.Type,
		"next_run", r.NextRunAt,
	)
	return r, nil
}

func (s *RecurringService) Update(ctx context.Context, id, userID int, in models.RecurringRuleInput) (*models.RecurringRule, error) {
	if err := validateRecurringInput(in); err != nil {
		return nil, err
	}
	if in.IntervalCount <= 0 {
		in.IntervalCount = 1
	}

	var catID *int
	if in.Category != "" {
		catType := in.Type
		if catType == "transfer" {
			catType = "expense"
		}
		cat, err := s.categories.FindOrCreate(ctx, userID, strings.TrimSpace(in.Category), catType)
		if err != nil {
			return nil, fmt.Errorf("find/create category: %w", err)
		}
		catID = &cat.ID
	}

	r, err := s.rules.Update(ctx, id, userID, in, catID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}

	s.log.InfoContext(ctx, "recurring rule updated",
		"user_id", userID,
		"rule_id", r.ID,
	)
	return r, nil
}

func (s *RecurringService) SetActive(ctx context.Context, id, userID int, active bool) (*models.RecurringRule, error) {
	r, err := s.rules.SetActive(ctx, id, userID, active)
	if err != nil {
		return nil, apperr.ErrNotFound
	}

	s.log.InfoContext(ctx, "recurring rule active status changed",
		"user_id", userID,
		"rule_id", id,
		"active", active,
	)
	return r, nil
}

func (s *RecurringService) Delete(ctx context.Context, id, userID int) error {
	ok, err := s.rules.Delete(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("delete recurring rule (id=%d): %w", id, err)
	}
	if !ok {
		return apperr.ErrNotFound
	}

	s.log.InfoContext(ctx, "recurring rule deleted",
		"user_id", userID,
		"rule_id", id,
	)
	return nil
}

func (s *RecurringService) RunDue(ctx context.Context) (int, int, error) {
	rules, err := s.rules.DueRules(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch due rules: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	success, failed := 0, 0

	for _, rule := range rules {
		if rule.NextRunAt > today {
			continue
		}

		txnID, err := s.materialize(ctx, rule)
		if err != nil {
			s.log.ErrorContext(ctx, "recurring materialize failed",
				"rule_id", rule.ID,
				"user_id", rule.UserID,
				"error", err,
			)
			_ = s.rules.RecordRunFailure(ctx, rule.ID, rule.NextRunAt, err.Error())
			failed++
			continue
		}

		nextRun, done := s.advanceDate(rule.NextRunAt, rule.Frequency, rule.IntervalCount, rule.EndDate)

		if err := s.rules.AdvanceRun(ctx, rule.ID, rule.NextRunAt, nextRun, txnID, done); err != nil {
			s.log.ErrorContext(ctx, "recurring advance failed",
				"rule_id", rule.ID,
				"user_id", rule.UserID,
				"error", err,
			)
			failed++
			continue
		}
		success++
	}

	if success > 0 || failed > 0 {
		s.log.InfoContext(ctx, "recurring run completed", "success", success, "failed", failed)
	}
	return success, failed, nil
}

func (s *RecurringService) materialize(ctx context.Context, rule models.RecurringRule) (int, error) {
	key := fmt.Sprintf("recurring-%d-%s", rule.ID, rule.NextRunAt)
	catName := categoryName(ctx, s.categories, rule.UserID, rule.CategoryID)

	switch rule.Type {
	case "income":
		detail, err := s.txns.CreateIncome(ctx, rule.UserID, models.TransactionInput{
			Title: rule.Title, Amount: rule.Amount, Type: "income",
			AccountID: deref(rule.AccountID), Date: rule.NextRunAt, Note: rule.Note,
			Category: catName,
		}, key)
		if err != nil {
			return 0, err
		}
		return detail.Transaction.ID, nil

	case "expense":
		detail, err := s.txns.CreateExpense(ctx, rule.UserID, models.TransactionInput{
			Title: rule.Title, Amount: rule.Amount, Type: "expense",
			AccountID: deref(rule.AccountID), Date: rule.NextRunAt, Note: rule.Note,
			Category: catName,
		}, key)
		if err != nil {
			return 0, err
		}
		return detail.Transaction.ID, nil

	case "transfer":
		detail, err := s.txns.CreateTransfer(ctx, rule.UserID, models.TransferInput{
			Title: rule.Title, Amount: rule.Amount,
			FromAccountID: deref(rule.FromAccountID),
			ToAccountID:   deref(rule.ToAccountID),
			Date:          rule.NextRunAt, Note: rule.Note,
		}, key)
		if err != nil {
			return 0, err
		}
		return detail.Transaction.ID, nil
	}
	return 0, fmt.Errorf("unsupported recurring type: %s", rule.Type)
}

func deref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func categoryName(ctx context.Context, cats *repository.CategoryRepository, userID int, catID *int) string {
	if catID == nil {
		return ""
	}
	all, err := cats.ListByUser(ctx, userID)
	if err != nil {
		return ""
	}
	for _, c := range all {
		if c.ID == *catID {
			return c.Name
		}
	}
	return ""
}

func (s *RecurringService) advanceDate(from, freq string, n int, endDate *string) (string, bool) {
	t, err := time.Parse("2006-01-02", from)
	if err != nil {
		s.log.Warn("advanceDate: parse error", "from", from, "error", err)
		return from, true
	}

	switch freq {
	case "daily":
		t = t.AddDate(0, 0, n)
	case "weekly":
		t = t.AddDate(0, 0, 7*n)
	case "monthly":
		t = t.AddDate(0, n, 0)
	default:
		s.log.Warn("advanceDate: unknown frequency, deactivating", "frequency", freq)
		return from, true
	}

	next := t.Format("2006-01-02")
	if endDate != nil && *endDate != "" && next > *endDate {
		return next, true
	}
	return next, false
}

func validateRecurringInput(in models.RecurringRuleInput) error {
	if in.Type != "income" && in.Type != "expense" && in.Type != "transfer" {
		return apperr.Validation("type must be income, expense or transfer")
	}
	if strings.TrimSpace(in.Title) == "" {
		return apperr.Validation("title is required")
	}
	if in.Amount <= 0 {
		return apperr.Validation("amount must be greater than 0")
	}
	if in.Frequency != "daily" && in.Frequency != "weekly" && in.Frequency != "monthly" {
		return apperr.Validation("frequency must be daily, weekly or monthly")
	}
	switch in.Type {
	case "income", "expense":
		if in.AccountID <= 0 {
			return apperr.Validation("account_id is required")
		}
	case "transfer":
		if in.FromAccountID <= 0 || in.ToAccountID <= 0 {
			return apperr.Validation("from_account_id and to_account_id are required")
		}
		if in.FromAccountID == in.ToAccountID {
			return apperr.Validation("cannot transfer to the same account")
		}
	}
	return nil
}
