package repository

import (
	"context"
	"database/sql"
	"log"

	"budgetapp/internal/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type GoalRepository struct {
	db *pgxpool.Pool
}

func NewGoalRepository(db *pgxpool.Pool) *GoalRepository {
	log.Println("[repo.goal] NewGoalRepository: created")
	return &GoalRepository{db: db}
}

func (r *GoalRepository) ListByUser(ctx context.Context, userID int) ([]models.Goal, error) {
	log.Printf("[repo.goal] ListByUser: user_id=%d", userID)
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, name, target_amount, current_amount,
		        to_char(target_date, 'YYYY-MM-DD'), color, status, created_at, updated_at
		 FROM goals WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		log.Printf("[repo.goal] ListByUser: query failed user_id=%d error=%v", userID, err)
		return nil, err
	}
	defer rows.Close()

	goals := []models.Goal{}
	for rows.Next() {
		var g models.Goal
		var td sql.NullString
		if err := rows.Scan(&g.ID, &g.UserID, &g.Name, &g.TargetAmount, &g.CurrentAmount,
			&td, &g.Color, &g.Status, &g.CreatedAt, &g.UpdatedAt); err != nil {
			log.Printf("[repo.goal] ListByUser: scan failed user_id=%d error=%v", userID, err)
			return nil, err
		}
		if td.Valid {
			g.TargetDate = &td.String
		}
		if g.TargetAmount > 0 {
			g.Progress = (g.CurrentAmount / g.TargetAmount) * 100
			if g.Progress > 100 {
				g.Progress = 100
			}
		}
		goals = append(goals, g)
	}
	log.Printf("[repo.goal] ListByUser: OK user_id=%d count=%d", userID, len(goals))
	return goals, rows.Err()
}

func (r *GoalRepository) Create(ctx context.Context, userID int, in models.GoalInput) (*models.Goal, error) {
	log.Printf("[repo.goal] Create: user_id=%d name=%q target=%.2f", userID, in.Name, in.TargetAmount)
	var g models.Goal
	var td sql.NullString
	var targetDate *string
	if in.TargetDate != "" {
		targetDate = &in.TargetDate
	}
	err := r.db.QueryRow(ctx,
		`INSERT INTO goals (user_id, name, target_amount, target_date, color)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, user_id, name, target_amount, current_amount,
		           to_char(target_date, 'YYYY-MM-DD'), color, status, created_at, updated_at`,
		userID, in.Name, in.TargetAmount, targetDate, in.Color,
	).Scan(&g.ID, &g.UserID, &g.Name, &g.TargetAmount, &g.CurrentAmount,
		&td, &g.Color, &g.Status, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		log.Printf("[repo.goal] Create: FAILED user_id=%d error=%v", userID, err)
		return nil, err
	}
	if td.Valid {
		g.TargetDate = &td.String
	}
	log.Printf("[repo.goal] Create: OK goal_id=%d", g.ID)
	return &g, nil
}

func (r *GoalRepository) Update(ctx context.Context, id, userID int, in models.GoalInput) (*models.Goal, error) {
	log.Printf("[repo.goal] Update: goal_id=%d user_id=%d", id, userID)
	var g models.Goal
	var td sql.NullString
	var targetDate *string
	if in.TargetDate != "" {
		targetDate = &in.TargetDate
	}
	err := r.db.QueryRow(ctx,
		`UPDATE goals SET name=$1, target_amount=$2, target_date=$3, color=$4, updated_at=now()
		 WHERE id=$5 AND user_id=$6
		 RETURNING id, user_id, name, target_amount, current_amount,
		           to_char(target_date, 'YYYY-MM-DD'), color, status, created_at, updated_at`,
		in.Name, in.TargetAmount, targetDate, in.Color, id, userID,
	).Scan(&g.ID, &g.UserID, &g.Name, &g.TargetAmount, &g.CurrentAmount,
		&td, &g.Color, &g.Status, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		log.Printf("[repo.goal] Update: FAILED goal_id=%d error=%v", id, err)
		return nil, err
	}
	if td.Valid {
		g.TargetDate = &td.String
	}
	log.Printf("[repo.goal] Update: OK goal_id=%d", g.ID)
	return &g, nil
}

func (r *GoalRepository) Contribute(ctx context.Context, id, userID int, amount float64) (*models.Goal, error) {
	log.Printf("[repo.goal] Contribute: goal_id=%d user_id=%d amount=%.2f", id, userID, amount)
	var g models.Goal
	var td sql.NullString
	err := r.db.QueryRow(ctx,
		`UPDATE goals
		 SET current_amount = current_amount + $1,
		     status = CASE WHEN current_amount + $1 >= target_amount THEN 'completed' ELSE status END,
		     updated_at = now()
		 WHERE id = $2 AND user_id = $3
		 RETURNING id, user_id, name, target_amount, current_amount,
		           to_char(target_date, 'YYYY-MM-DD'), color, status, created_at, updated_at`,
		amount, id, userID,
	).Scan(&g.ID, &g.UserID, &g.Name, &g.TargetAmount, &g.CurrentAmount,
		&td, &g.Color, &g.Status, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		log.Printf("[repo.goal] Contribute: FAILED goal_id=%d error=%v", id, err)
		return nil, err
	}
	if td.Valid {
		g.TargetDate = &td.String
	}
	if g.TargetAmount > 0 {
		g.Progress = (g.CurrentAmount / g.TargetAmount) * 100
		if g.Progress > 100 {
			g.Progress = 100
		}
	}
	log.Printf("[repo.goal] Contribute: OK goal_id=%d new_amount=%.2f status=%s", g.ID, g.CurrentAmount, g.Status)
	return &g, nil
}

func (r *GoalRepository) Delete(ctx context.Context, id, userID int) (bool, error) {
	log.Printf("[repo.goal] Delete: goal_id=%d user_id=%d", id, userID)
	tag, err := r.db.Exec(ctx, `DELETE FROM goals WHERE id=$1 AND user_id=$2`, id, userID)
	if err != nil {
		return false, err
	}
	deleted := tag.RowsAffected() > 0
	log.Printf("[repo.goal] Delete: goal_id=%d deleted=%v", id, deleted)
	return deleted, nil
}
