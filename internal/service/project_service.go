package service

import (
	"context"
	"errors"
	"strings"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

var ErrInvalidProject = errors.New("project name must be 1–100 characters and description at most 500 characters")

type ProjectService struct{ repo repository.ProjectRepository }

func NewProjectService(repo repository.ProjectRepository) *ProjectService {
	return &ProjectService{repo: repo}
}

func (s *ProjectService) Create(ctx context.Context, userID, name, description string) (*entity.Project, error) {
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if name == "" || len([]rune(name)) > 100 || len([]rune(description)) > 500 {
		return nil, ErrInvalidProject
	}
	return s.repo.Create(ctx, userID, name, description)
}

func (s *ProjectService) List(ctx context.Context, userID string) ([]entity.Project, error) {
	return s.repo.List(ctx, userID)
}

func (s *ProjectService) Get(ctx context.Context, userID, projectID string) (*entity.Project, error) {
	return s.repo.Get(ctx, userID, projectID)
}

func (s *ProjectService) Update(ctx context.Context, user, project, name, description string) (*entity.Project, error) {
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	if name == "" || len([]rune(name)) > 100 || len([]rune(description)) > 500 {
		return nil, ErrInvalidProject
	}
	return s.repo.Update(ctx, user, project, name, description)
}
func (s *ProjectService) Delete(ctx context.Context, user, project string) error {
	return s.repo.Delete(ctx, user, project)
}
