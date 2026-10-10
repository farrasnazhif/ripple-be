package service

import (
	"context"
	"errors"
	"strings"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

var ErrFinalRegeneration = errors.New("choose a completed generated version with a saved prompt; combined outputs must be regenerated from their source clips")

// Regenerate repeats the saved request, including the original reference for edits.
func (s *FinalRefinementService) Regenerate(ctx context.Context, user, project, source, key string, index int) (*entity.FinalOutput, error) {
	base, err := s.repo.Get(ctx, user, project, source)
	if err != nil {
		return nil, err
	}
	if base.Status != "completed" || strings.TrimSpace(base.Prompt) == "" || index < 0 {
		return nil, ErrFinalRegeneration
	}
	reason := "Regenerated from " + source + " using the same prompt"
	if base.ParentID != nil {
		if index != base.ImageIndex {
			return nil, ErrInvalidFinalRefinement
		}
		return s.Refine(ctx, user, project, *base.ParentID, key, index, base.Prompt, reason)
	}
	if base.Kind != "image" || len(base.JobIDs) != 0 || index >= len(base.Outputs) {
		return nil, ErrFinalRegeneration
	}
	return s.generateImage(ctx, user, project, key, base.BriefVersion, base.Prompt, reason)
}
