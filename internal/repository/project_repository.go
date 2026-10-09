package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

type ProjectRepository interface {
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
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return project, nil
}

func (r *projectRepository) List(ctx context.Context, userID string) ([]entity.Project, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT p.id, b.id, p.name, p.description, p.created_at FROM projects p JOIN briefs b ON b.project_id = p.id WHERE p.user_id = $1 ORDER BY p.created_at DESC, p.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []entity.Project{}
	for rows.Next() {
		var project entity.Project
		if err := rows.Scan(&project.ID, &project.BriefID, &project.Name, &project.Description, &project.CreatedAt); err != nil {
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
	err := r.db.QueryRowContext(ctx, `SELECT p.id, b.id, p.name, p.description, p.created_at FROM projects p JOIN briefs b ON b.project_id = p.id WHERE p.id = $1 AND p.user_id = $2`, projectID, userID).Scan(&project.ID, &project.BriefID, &project.Name, &project.Description, &project.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, entity.ErrNotFound
	}
	return project, err
}
