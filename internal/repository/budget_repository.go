package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

var ErrBudgetUnset = errors.New("set a project generation budget before generating media")
var ErrBudgetExceeded = errors.New("insufficient project budget; increase the budget or wait for reserved requests to finish")
var ErrBudgetRate = errors.New("generation cost estimate is not configured; contact the administrator")
var ErrBudgetLimit = errors.New("budget must be between 0 and 100000000 USD cents and cover existing spending and reservations")

type BudgetRepository interface {
	Get(context.Context, string, string) (*entity.ProjectBudget, error)
	Set(context.Context, string, string, int64) error
}
type budgetRepository struct {
	db    *sql.DB
	rates entity.GenerationRates
}

func NewBudgetRepository(db *sql.DB, rates entity.GenerationRates) BudgetRepository {
	return &budgetRepository{db, rates}
}

func (r *budgetRepository) Get(ctx context.Context, user, project string) (*entity.ProjectBudget, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	b := &entity.ProjectBudget{Rates: r.rates, Usage: []entity.GenerationUsage{}}
	err := r.db.QueryRowContext(ctx, `SELECT b.limit_cents,COALESCE(SUM(u.estimate_cents) FILTER(WHERE u.state='reserved'),0),COALESCE(SUM(u.estimate_cents) FILTER(WHERE u.state='spent'),0) FROM projects p LEFT JOIN project_budgets b ON b.project_id=p.id LEFT JOIN generation_usage u ON u.project_id=p.id WHERE p.id=$1 AND p.user_id=$2 GROUP BY p.id,b.limit_cents`, project, user).Scan(&b.LimitCents, &b.ReservedCents, &b.SpentCents)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, entity.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if b.LimitCents != nil {
		b.AvailableCents = *b.LimitCents - b.ReservedCents - b.SpentCents
	}
	rows, err := r.db.QueryContext(ctx, `SELECT u.operation_id,u.operation,u.description,a.username,u.estimate_cents,u.state,COALESCE(g.status,f.status,'unavailable'),u.created_at FROM generation_usage u JOIN users a ON a.id=u.actor_id LEFT JOIN generation_jobs g ON g.id=u.operation_id LEFT JOIN project_final_outputs f ON f.id=u.operation_id WHERE u.project_id=$1 ORDER BY u.created_at DESC,u.operation_id LIMIT 100`, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry entity.GenerationUsage
		if err := rows.Scan(&entry.OperationID, &entry.Operation, &entry.Description, &entry.Actor, &entry.EstimateCents, &entry.State, &entry.Status, &entry.CreatedAt); err != nil {
			return nil, err
		}
		b.Usage = append(b.Usage, entry)
	}
	return b, rows.Err()
}

func (r *budgetRepository) Set(ctx context.Context, user, project string, limit int64) error {
	if limit < 0 || limit > 100000000 {
		return ErrBudgetLimit
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner string
	if err = tx.QueryRowContext(ctx, `SELECT user_id FROM projects WHERE id=$1 AND user_id=$2 FOR NO KEY UPDATE`, project, user).Scan(&owner); errors.Is(err, sql.ErrNoRows) {
		return entity.ErrNotFound
	}
	if err != nil {
		return err
	}
	var held int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(estimate_cents),0) FROM generation_usage WHERE project_id=$1 AND state IN ('reserved','spent')`, project).Scan(&held); err != nil {
		return err
	}
	if limit < held {
		return ErrBudgetLimit
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO project_budgets(project_id,limit_cents)VALUES($1,$2) ON CONFLICT(project_id)DO UPDATE SET limit_cents=excluded.limit_cents`, project, limit); err != nil {
		return err
	}
	return tx.Commit()
}

// The project lock serializes reservations and cap edits across every media route.
func reserveBudget(ctx context.Context, tx *sql.Tx, rates entity.GenerationRates, project, id, operation, description string) error {
	estimate := rates[operation]
	if estimate <= 0 {
		return ErrBudgetRate
	}
	var owner string
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM projects WHERE id=$1 FOR NO KEY UPDATE`, project).Scan(&owner); err != nil {
		return err
	}
	var limit int64
	err := tx.QueryRowContext(ctx, `SELECT limit_cents FROM project_budgets WHERE project_id=$1`, project).Scan(&limit)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrBudgetUnset
	}
	if err != nil {
		return err
	}
	var held int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(estimate_cents),0) FROM generation_usage WHERE project_id=$1 AND state IN ('reserved','spent')`, project).Scan(&held); err != nil {
		return err
	}
	if estimate > limit-held {
		return ErrBudgetExceeded
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO generation_usage(operation_id,project_id,actor_id,operation,description,estimate_cents)VALUES($1,$2,$3,$4,$5,$6)`, id, project, owner, operation, description, estimate)
	return err
}

func settleBudget(ctx context.Context, tx *sql.Tx, id, status string, rejected bool) error {
	state := "reserved"
	if rejected && status == "failed" {
		state = "released"
	} else if status == "completed" || status == "failed" || status == "nsfw" || status == "canceled" {
		state = "spent"
	}
	// shortcut: providers do not return invoice amounts here; retain estimates for accepted failures until billing reconciliation exists.
	_, err := tx.ExecContext(ctx, `UPDATE generation_usage SET state=$2 WHERE operation_id=$1 AND state='reserved'`, id, state)
	return err
}
