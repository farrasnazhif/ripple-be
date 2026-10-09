package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

var ErrWorkflowConflict = errors.New("storyboard already saved or summary changed; reload the project")

type StoryboardRepository interface {
	List(context.Context, string, string) ([]entity.StoryboardShot, error)
	Create(context.Context, string, string, string, []entity.StoryboardShot) error
	GetShot(context.Context, string, string, string) (*entity.StoryboardShot, error)
}
type storyboardRepository struct{ db *sql.DB }

func NewStoryboardRepository(db *sql.DB) StoryboardRepository { return &storyboardRepository{db} }
func (r *storyboardRepository) List(ctx context.Context, userID, projectID string) ([]entity.StoryboardShot, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT s.id,s.title,s.prompt,s.position,ARRAY(SELECT point_key FROM shot_requirements WHERE shot_id=s.id ORDER BY point_key) FROM storyboard_shots s JOIN projects p ON p.id=s.project_id WHERE p.id=$1 AND p.user_id=$2 ORDER BY s.position`, projectID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	shots := []entity.StoryboardShot{}
	for rows.Next() {
		var s entity.StoryboardShot
		if err := rows.Scan(&s.ID, &s.Title, &s.Prompt, &s.Position, pq.Array(&s.RequirementKeys)); err != nil {
			return nil, err
		}
		shots = append(shots, s)
	}
	return shots, rows.Err()
}
func (r *storyboardRepository) Create(ctx context.Context, userID, projectID, summary string, shots []entity.StoryboardShot) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var saved string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(b.summary,'') FROM briefs b JOIN projects p ON p.id=b.project_id WHERE p.id=$1 AND p.user_id=$2 FOR UPDATE OF b`, projectID, userID).Scan(&saved)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.ErrNotFound
	}
	if err != nil {
		return err
	}
	if saved != summary {
		return ErrWorkflowConflict
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM storyboard_shots WHERE project_id=$1)`, projectID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrWorkflowConflict
	}
	for i, s := range shots {
		var id string
		if err = tx.QueryRowContext(ctx, `INSERT INTO storyboard_shots(project_id,position,title,prompt) VALUES($1,$2,$3,$4) RETURNING id`, projectID, i+1, s.Title, s.Prompt).Scan(&id); err != nil {
			return err
		}
		for _, key := range s.RequirementKeys {
			result, e := tx.ExecContext(ctx, `INSERT INTO shot_requirements(shot_id,point_key) SELECT $1,bp.key FROM brief_points bp JOIN briefs b ON b.id=bp.brief_id WHERE b.project_id=$2 AND bp.key=$3`, id, projectID, key)
			if e != nil {
				return e
			}
			if n, _ := result.RowsAffected(); n != 1 {
				return ErrWorkflowConflict
			}
		}
	}
	return tx.Commit()
}
func (r *storyboardRepository) GetShot(ctx context.Context, userID, projectID, shotID string) (*entity.StoryboardShot, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	s := &entity.StoryboardShot{}
	err := r.db.QueryRowContext(ctx, `SELECT s.id,s.title,s.prompt,s.position,ARRAY(SELECT point_key FROM shot_requirements WHERE shot_id=s.id ORDER BY point_key) FROM storyboard_shots s JOIN projects p ON p.id=s.project_id WHERE p.id=$1 AND p.user_id=$2 AND s.id=$3`, projectID, userID, shotID).Scan(&s.ID, &s.Title, &s.Prompt, &s.Position, pq.Array(&s.RequirementKeys))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, entity.ErrNotFound
	}
	return s, err
}
