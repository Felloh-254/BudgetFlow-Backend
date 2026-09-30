package service

import (
	"context"
	"fmt"
	"log"
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
}

func NewRecurringService(
	rules *repository.RecurringRepository,
	categories *repository.CategoryRepository,
	txns *TransactionService,
) *RecurringService {
	log.Println("[service.recurring] NewRecurringService: created")
	return &RecurringService{rules: rules, categories: categories, txns: txns}
}

func (s *RecurringService) List(ctx context.Context, userID int) ([]models.RecurringRule, error) {
	log.Printf("[service.recurring] List: user_id=%d", userID)
	rs, err := s.rules.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	log.Printf("[service.recurring] List: OK user_id=%d count=%d", userID, len(rs))
	return rs, nil
}

func (s *RecurringService) Create(ctx context.Context, userID int, in models.RecurringRuleInput) (*models.RecurringRule, error) {
	log.Printf("[service.recurring] Create: user_id=%d type=%s freq=%s amount=%.2f",
		userID, in.Type, in.Frequency, in.Amount)

	if err := validateRecurringInput(in); err != nil {
		log.Printf("[service.recurring] Create: validation failed user_id=%d error=%v", userID, err)
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
			log.Printf("[service.recurring] Create: category find/create failed error=%v", err)
			return nil, err
		}
		catID = &cat.ID
	}

	r, err := s.rules.Create(ctx, userID, in, catID)
	if err != nil {
		log.Printf("[service.recurring] Create: repo error user_id=%d error=%v", userID, err)
		return nil, err
	}
	log.Printf("[service.recurring] Create: OK rule_id=%d next_run=%s", r.ID, r.NextRunAt)
	return r, nil
}

func (s *RecurringService) Update(ctx context.Context, id, userID int, in models.RecurringRuleInput) (*models.RecurringRule, error) {
	log.Printf("[service.recurring] Update: rule_id=%d user_id=%d", id, userID)
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
			return nil, err
		}
		catID = &cat.ID
	}

	r, err := s.rules.Update(ctx, id, userID, in, catID)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	log.Printf("[service.recurring] Update: OK rule_id=%d", r.ID)
	return r, nil
}

func (s *RecurringService) SetActive(ctx context.Context, id, userID int, active bool) (*models.RecurringRule, error) {
	log.Printf("[service.recurring] SetActive: rule_id=%d user_id=%d active=%v", id, userID, active)
	r, err := s.rules.SetActive(ctx, id, userID, active)
	if err != nil {
		return nil, apperr.ErrNotFound
	}
	return r, nil
}

func (s *RecurringService) Delete(ctx context.Context, id, userID int) error {
	log.Printf("[service.recurring] Delete: rule_id=%d user_id=%d", id, userID)
	ok, err := s.rules.Delete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrNotFound
	}
	return nil
}

func (s *RecurringService) RunDue(ctx context.Context) (int, int, error) {
	log.Printf("[service.recurring] RunDue: starting scan")

	rules, err := s.rules.DueRules(ctx)
	if err != nil {
		log.Printf("[service.recurring] RunDue: DueRules failed error=%v", err)
		return 0, 0, err
	}
	log.Printf("[service.recurring] RunDue: found %d due rules", len(rules))

	today := time.Now().Format("2006-01-02")
	success, failed := 0, 0

	for _, rule := range rules {
		log.Printf("[service.recurring] RunDue: processing rule_id=%d user_id=%d type=%s title=%q amount=%.2f next_run=%s",
			rule.ID, rule.UserID, rule.Type, rule.Title, rule.Amount, rule.NextRunAt)

		if rule.NextRunAt > today {
			log.Printf("[service.recurring] RunDue: rule_id=%d skipped (next_run in future)", rule.ID)
			continue
		}

		txnID, err := s.materialize(ctx, rule)
		if err != nil {
			log.Printf("[service.recurring] RunDue: rule_id=%d MATERIALIZE FAILED error=%v", rule.ID, err)
			_ = s.rules.RecordRunFailure(ctx, rule.ID, rule.NextRunAt, err.Error())
			failed++
			continue
		}
		log.Printf("[service.recurring] RunDue: rule_id=%d materialized transaction_id=%d", rule.ID, txnID)

		nextRun, done := advanceDate(rule.NextRunAt, rule.Frequency, rule.IntervalCount, rule.EndDate)
		log.Printf("[service.recurring] RunDue: rule_id=%d advancing next_run=%s done=%v", rule.ID, nextRun, done)

		if err := s.rules.AdvanceRun(ctx, rule.ID, rule.NextRunAt, nextRun, txnID, done); err != nil {
			log.Printf("[service.recurring] RunDue: rule_id=%d advance FAILED error=%v", rule.ID, err)
			failed++
			continue
		}
		success++
	}

	log.Printf("[service.recurring] RunDue: DONE success=%d failed=%d", success, failed)
	return success, failed, nil
}

func (s *RecurringService) materialize(ctx context.Context, rule models.RecurringRule) (int, error) {
	log.Printf("[service.recurring] materialize: rule_id=%d type=%s", rule.ID, rule.Type)

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

func advanceDate(from, freq string, n int, endDate *string) (string, bool) {
	t, err := time.Parse("2006-01-02", from)
	if err != nil {
		log.Printf("[service.recurring] advanceDate: parse error from=%q error=%v", from, err)
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
		log.Printf("[service.recurring] advanceDate: unknown frequency=%q — deactivating", freq)
		return from, true
	}

	next := t.Format("2006-01-02")
	if endDate != nil && *endDate != "" && next > *endDate {
		log.Printf("[service.recurring] advanceDate: next=%s past end_date=%s — deactivating", next, *endDate)
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
