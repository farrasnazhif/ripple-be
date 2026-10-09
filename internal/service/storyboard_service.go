package service

import (
	"context"
	"errors"
	"strings"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

var ErrInvalidStoryboard = errors.New("confirm a summary and provide 1–50 shots, each with a title (up to 200 characters) and prompt (up to 10000 characters)")

type StoryboardService struct {
	projects repository.ProjectRepository
	briefs   repository.BriefRepository
	repo     repository.StoryboardRepository
}

func NewStoryboardService(p repository.ProjectRepository, b repository.BriefRepository, r repository.StoryboardRepository) *StoryboardService {
	return &StoryboardService{p, b, r}
}
func (s *StoryboardService) List(ctx context.Context, user, project string) ([]entity.StoryboardShot, error) {
	if _, err := s.projects.Get(ctx, user, project); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, user, project)
}
func (s *StoryboardService) Create(ctx context.Context, user, project, summary string, shots []entity.StoryboardShot) error {
	p, err := s.projects.Get(ctx, user, project)
	if err != nil {
		return err
	}
	b, err := s.briefs.Get(ctx, user, p.BriefID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(summary) == "" || b.Summary == nil || summary != *b.Summary || len(shots) < 1 || len(shots) > 50 {
		return ErrInvalidStoryboard
	}
	for _, shot := range shots {
		seen := map[string]bool{}
		if len(shot.RequirementKeys) > 50 {
			return ErrInvalidStoryboard
		}
		for _, key := range shot.RequirementKeys {
			if !pointKey.MatchString(key) || seen[key] {
				return ErrInvalidStoryboard
			}
			seen[key] = true
		}
		if strings.TrimSpace(shot.Title) == "" || len([]rune(shot.Title)) > 200 || strings.TrimSpace(shot.Prompt) == "" || len([]rune(shot.Prompt)) > 10000 {
			return ErrInvalidStoryboard
		}
	}
	return s.repo.Create(ctx, user, project, summary, shots)
}
