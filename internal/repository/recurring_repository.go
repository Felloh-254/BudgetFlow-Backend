package repository

import (
	"context"
	"database/sql"
	"log"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RecurringRepository struct {
	db *pgxpool.Pool
}

func NewRecurringRepository(db *pgxpool.Pool) *RecurringRepository {
	log.Println("[repo.recurring] NewRecurringRepository: created")
	return &RecurringRepository{db: db}
}

const recurringSelect = `
	SELECT id, user_id, type, title, amount, note,
	       category_id, account_id, from_account_id, to_account_id,
	       frequency, interval_count,
	       to_char(start_date, 'YYYY-MM-DD'),
	       to_char(end_date,   'YYYY-MM-DD'),
	       to_char(next_run_at,'YYYY-MM-DD'),
	       to_char(last_run_at,'YYYY-MM-DD'),
	       active, created_at, updated_at
	FROM recurring_rules`

func scanRecurring(row interface{ Scan(dest ...any) error }) (*models.RecurringRule, error) {
	var r models.RecurringRule
	var catID, accID, fromID, toID sql.NullInt64
	var endDate, lastRun sql.NullString
	err := row.Scan(
		&r.ID, &r.UserID, &r.Type, &r.Title, &r.Amount, &r.Note,
		&catID, &accID, &fromID, &toID,
		&r.Frequency, &r.IntervalCount,
		&r.StartDate, &endDate, &r.NextRunAt, &lastRun,
		&r.Active, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if catID.Valid {
		v := int(catID.Int64)
		r.CategoryID = &v
	}
	if accID.Valid {
		v := int(accID.Int64)
		r.AccountID = &v
	}
	if fromID.Valid {
		v := int(fromID.Int64)
		r.FromAccountID = &v
	}
	if toID.Valid {
		v := int(toID.Int64)
		r.ToAccountID = &v
	}
	if endDate.Valid {
		r.EndDate = &endDate.String
	}
	if lastRun.Valid {
		r.LastRunAt = &lastRun.String
	}
	return &r, nil
}

func (r *RecurringRepository) ListByUser(ctx context.Context, userID int) ([]models.RecurringRule, error) {
	log.Printf("[repo.recurring] ListByUser: user_id=%d", userID)
	rows, err := r.db.Query(ctx, recurringSelect+` WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		log.Printf("[repo.recurring] ListByUser: FAILED user_id=%d error=%v", userID, err)
		return nil, err
	}
	defer rows.Close()

	out := []models.RecurringRule{}
	for rows.Next() {
		rule, err := scanRecurring(rows)
		if err != nil {
			log.Printf("[repo.recurring] ListByUser: scan FAILED user_id=%d error=%v", userID, err)
			return nil, err
		}
		out = append(out, *rule)
	}
	log.Printf("[repo.recurring] ListByUser: OK user_id=%d count=%d", userID, len(out))
	return out, rows.Err()
}

func (r *RecurringRepository) GetByID(ctx context.Context, id, userID int) (*models.RecurringRule, error) {
	log.Printf("[repo.recurring] GetByID: rule_id=%d user_id=%d", id, userID)
	row := r.db.QueryRow(ctx, recurringSelect+` WHERE id=$1 AND user_id=$2`, id, userID)
	rule, err := scanRecurring(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			log.Printf("[repo.recurring] GetByID: not found rule_id=%d user_id=%d", id, userID)
			return nil, nil
		}
		return nil, err
	}
	log.Printf("[repo.recurring] GetByID: OK rule_id=%d next_run=%s", rule.ID, rule.NextRunAt)
	return rule, nil
}

func (r *RecurringRepository) Create(ctx context.Context, userID int, in models.RecurringRuleInput, categoryID *int) (*models.RecurringRule, error) {
	log.Printf("[repo.recurring] Create: user_id=%d type=%s frequency=%s amount=%.2f next_run=%s",
		userID, in.Type, in.Frequency, in.Amount, in.StartDate)

	var endDate *string
	if in.EndDate != "" {
		endDate = &in.EndDate
	}
	var accID, fromID, toID *int
	if in.AccountID > 0 {
		accID = &in.AccountID
	}
	if in.FromAccountID > 0 {
		fromID = &in.FromAccountID
	}
	if in.ToAccountID > 0 {
		toID = &in.ToAccountID
	}

	row := r.db.QueryRow(ctx,
		`INSERT INTO recurring_rules
		   (user_id, type, title, amount, note, category_id,
		    account_id, from_account_id, to_account_id,
		    frequency, interval_count, start_date, end_date, next_run_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$12)
		 RETURNING id, user_id, type, title, amount, note,
		           category_id, account_id, from_account_id, to_account_id,
		           frequency, interval_count,
		           to_char(start_date,'YYYY-MM-DD'),
		           to_char(end_date,'YYYY-MM-DD'),
		           to_char(next_run_at,'YYYY-MM-DD'),
		           to_char(last_run_at,'YYYY-MM-DD'),
		           active, created_at, updated_at`,
		userID, in.Type, in.Title, in.Amount, in.Note, categoryID,
		accID, fromID, toID,
		in.Frequency, in.IntervalCount, in.StartDate, endDate,
	)
	rule, err := scanRecurring(row)
	if err != nil {
		log.Printf("[repo.recurring] Create: FAILED user_id=%d error=%v", userID, err)
		return nil, err
	}
	log.Printf("[repo.recurring] Create: OK rule_id=%d next_run=%s", rule.ID, rule.NextRunAt)
	return rule, nil
}

func (r *RecurringRepository) Update(ctx context.Context, id, userID int, in models.RecurringRuleInput, categoryID *int) (*models.RecurringRule, error) {
	log.Printf("[repo.recurring] Update: rule_id=%d user_id=%d", id, userID)

	var endDate *string
	if in.EndDate != "" {
		endDate = &in.EndDate
	}
	var accID, fromID, toID *int
	if in.AccountID > 0 {
		accID = &in.AccountID
	}
	if in.FromAccountID > 0 {
		fromID = &in.FromAccountID
	}
	if in.ToAccountID > 0 {
		toID = &in.ToAccountID
	}

	row := r.db.QueryRow(ctx,
		`UPDATE recurring_rules SET
		    type=$1, title=$2, amount=$3, note=$4, category_id=$5,
		    account_id=$6, from_account_id=$7, to_account_id=$8,
		    frequency=$9, interval_count=$10,
		    start_date=$11, end_date=$12, updated_at=now()
		 WHERE id=$13 AND user_id=$14
		 RETURNING id, user_id, type, title, amount, note,
		           category_id, account_id, from_account_id, to_account_id,
		           frequency, interval_count,
		           to_char(start_date,'YYYY-MM-DD'),
		           to_char(end_date,'YYYY-MM-DD'),
		           to_char(next_run_at,'YYYY-MM-DD'),
		           to_char(last_run_at,'YYYY-MM-DD'),
		           active, created_at, updated_at`,
		in.Type, in.Title, in.Amount, in.Note, categoryID,
		accID, fromID, toID,
		in.Frequency, in.IntervalCount, in.StartDate, endDate,
		id, userID,
	)
	rule, err := scanRecurring(row)
	if err != nil {
		return nil, err
	}
	log.Printf("[repo.recurring] Update: OK rule_id=%d", rule.ID)
	return rule, nil
}

func (r *RecurringRepository) SetActive(ctx context.Context, id, userID int, active bool) (*models.RecurringRule, error) {
	log.Printf("[repo.recurring] SetActive: rule_id=%d user_id=%d active=%v", id, userID, active)
	row := r.db.QueryRow(ctx,
		`UPDATE recurring_rules SET active=$1, updated_at=now()
		 WHERE id=$2 AND user_id=$3
		 RETURNING id, user_id, type, title, amount, note,
		           category_id, account_id, from_account_id, to_account_id,
		           frequency, interval_count,
		           to_char(start_date,'YYYY-MM-DD'),
		           to_char(end_date,'YYYY-MM-DD'),
		           to_char(next_run_at,'YYYY-MM-DD'),
		           to_char(last_run_at,'YYYY-MM-DD'),
		           active, created_at, updated_at`,
		active, id, userID,
	)
	rule, err := scanRecurring(row)
	if err != nil {
		return nil, err
	}
	log.Printf("[repo.recurring] SetActive: OK rule_id=%d active=%v", rule.ID, rule.Active)
	return rule, nil
}

func (r *RecurringRepository) Delete(ctx context.Context, id, userID int) (bool, error) {
	log.Printf("[repo.recurring] Delete: rule_id=%d user_id=%d", id, userID)
	tag, err := r.db.Exec(ctx, `DELETE FROM recurring_rules WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return false, err
	}
	ok := tag.RowsAffected() > 0
	log.Printf("[repo.recurring] Delete: rule_id=%d deleted=%v", id, ok)
	return ok, nil
}

func (r *RecurringRepository) DueRules(ctx context.Context) ([]models.RecurringRule, error) {
	log.Printf("[repo.recurring] DueRules: scanning all users for due rules")
	rows, err := r.db.Query(ctx,
		recurringSelect+` WHERE active=true AND next_run_at <= CURRENT_DATE ORDER BY next_run_at ASC`)
	if err != nil {
		log.Printf("[repo.recurring] DueRules: FAILED error=%v", err)
		return nil, err
	}
	defer rows.Close()

	out := []models.RecurringRule{}
	for rows.Next() {
		rule, err := scanRecurring(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rule)
	}
	log.Printf("[repo.recurring] DueRules: OK count=%d", len(out))
	return out, rows.Err()
}

func (r *RecurringRepository) AdvanceRun(ctx context.Context, ruleID int, ranForDate, nextRunDate string, transactionID int, done bool) error {
	log.Printf("[repo.recurring] AdvanceRun: rule_id=%d ran_for=%s next=%s txn_id=%d done=%v",
		ruleID, ranForDate, nextRunDate, transactionID, done)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO recurring_rule_runs (rule_id, ran_for_date, transaction_id, status)
		 VALUES ($1,$2,$3,'success')
		 ON CONFLICT (rule_id, ran_for_date) DO NOTHING`,
		ruleID, ranForDate, transactionID,
	)
	if err != nil {
		log.Printf("[repo.recurring] AdvanceRun: insert run FAILED rule_id=%d error=%v", ruleID, err)
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE recurring_rules
		 SET last_run_at=$1,
		     next_run_at=$2,
		     active = CASE WHEN $3 THEN false ELSE active END,
		     updated_at=now()
		 WHERE id=$4`,
		ranForDate, nextRunDate, done, ruleID,
	)
	if err != nil {
		log.Printf("[repo.recurring] AdvanceRun: update rule FAILED rule_id=%d error=%v", ruleID, err)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("[repo.recurring] AdvanceRun: COMMIT FAILED rule_id=%d error=%v", ruleID, err)
		return err
	}
	log.Printf("[repo.recurring] AdvanceRun: OK rule_id=%d", ruleID)
	return nil
}

func (r *RecurringRepository) RecordRunFailure(ctx context.Context, ruleID int, ranForDate string, errMsg string) error {
	log.Printf("[repo.recurring] RecordRunFailure: rule_id=%d date=%s error=%s", ruleID, ranForDate, errMsg)
	_, err := r.db.Exec(ctx,
		`INSERT INTO recurring_rule_runs (rule_id, ran_for_date, status, error)
		 VALUES ($1,$2,'failed',$3)
		 ON CONFLICT (rule_id, ran_for_date) DO NOTHING`,
		ruleID, ranForDate, errMsg,
	)
	return err
}
