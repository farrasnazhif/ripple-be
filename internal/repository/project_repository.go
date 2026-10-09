package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

type ProjectRepository interface {
	Update(context.Context, string, string, string, string) (*entity.Project, error)
	Delete(context.Context, string, string) error
	Create(ctx context.Context, userID, name, description string) (*entity.Project, error)
	List(ctx context.Context, userID string) ([]entity.Project, error)
	Get(ctx context.Context, userID, projectID string) (*entity.Project, error)
}

type projectRepository struct{ db *sql.DB }

func NewProjectRepository(db *sql.DB) ProjectRepository { return &projectRepository{db: db} }

func (r *projectRepository) Create(ctx context.Context, userID, name, description string) (*entity.Project, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	project := &entity.Project{Name: name, Description: description}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = tx.QueryRowContext(ctx, `INSERT INTO projects (user_id, name, description) VALUES ($1, $2, $3) RETURNING id, created_at`, userID, name, description).Scan(&project.ID, &project.CreatedAt); err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, `INSERT INTO briefs (user_id, project_id) VALUES ($1, $2) RETURNING id`, userID, project.ID).Scan(&project.BriefID); err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT username FROM users WHERE id=$1`, userID).Scan(&project.CreatedBy); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return project, nil
}

func (r *projectRepository) List(ctx context.Context, userID string) ([]entity.Project, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT p.id, b.id, p.name, p.description, p.created_at,u.username FROM projects p JOIN users u ON u.id=p.user_id JOIN briefs b ON b.project_id = p.id WHERE p.user_id = $1 ORDER BY p.created_at DESC, p.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []entity.Project{}
	for rows.Next() {
		var project entity.Project
		if err := rows.Scan(&project.ID, &project.BriefID, &project.Name, &project.Description, &project.CreatedAt, &project.CreatedBy); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (r *projectRepository) Get(ctx context.Context, userID, projectID string) (*entity.Project, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	project := &entity.Project{}
	err := r.db.QueryRowContext(ctx, `SELECT p.id, b.id, p.name, p.description, p.created_at,u.username FROM projects p JOIN users u ON u.id=p.user_id JOIN briefs b ON b.project_id = p.id WHERE p.id = $1 AND p.user_id = $2`, projectID, userID).Scan(&project.ID, &project.BriefID, &project.Name, &project.Description, &project.CreatedAt, &project.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, entity.ErrNotFound
	}
	return project, err
}

func (r *projectRepository) Update(ctx context.Context, user, project, name, description string) (*entity.Project, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	result, err := r.db.ExecContext(ctx, `UPDATE projects SET name=$3,description=$4 WHERE id=$1 AND user_id=$2`, project, user, name, description)
	if err != nil {
		return nil, err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return nil, entity.ErrNotFound
	}
	return r.Get(ctx, user, project)
}
func (r *projectRepository) Delete(ctx context.Context, user, project string) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `SELECT id FROM projects WHERE id=$1 AND user_id=$2 FOR UPDATE`, project, user).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.ErrNotFound
	}
	if err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM briefs WHERE project_id=$1 FOR UPDATE`, project).Scan(&id); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO storage_cleanup(object_key) SELECT a.object_key FROM brief_attachments a JOIN briefs b ON b.id=a.brief_id WHERE b.project_id=$1 UNION SELECT object_key FROM project_final_outputs WHERE project_id=$1 AND object_key<>'' ON CONFLICT(object_key) DO UPDATE SET not_before=EXCLUDED.not_before`, project)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM projects WHERE id=$1 AND user_id=$2`, project, user); err != nil {
		return err
	}
	return tx.Commit()
}
