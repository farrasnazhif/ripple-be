package service

import (
	"context"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

type BudgetService struct{ repo repository.BudgetRepository }

func NewBudgetService(r repository.BudgetRepository) *BudgetService { return &BudgetService{r} }
func (s *BudgetService) Get(ctx context.Context, user, project string) (*entity.ProjectBudget, error) {
	return s.repo.Get(ctx, user, project)
}
func (s *BudgetService) Set(ctx context.Context, user, project string, limit int64) error {
	return s.repo.Set(ctx, user, project, limit)
}
