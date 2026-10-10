package service

import (
	"context"
	"errors"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

var ErrInvalidDirectImage = errors.New("confirm a saved brief summary before generating its final image")

// GenerateImage submits the persisted summary directly; it never creates storyboard shots.
func (s *FinalRefinementService) GenerateImage(ctx context.Context, user, project, key string, version int) (*entity.FinalOutput, error) {
	return s.generateImage(ctx, user, project, key, version, "", "Generated directly from the confirmed brief summary")
}

func (s *FinalRefinementService) generateImage(ctx context.Context, user, project, key string, version int, expectedPrompt, reason string) (*entity.FinalOutput, error) {
	if version < 1 {
		return nil, ErrInvalidDirectImage
	}
	matches := func(f *entity.FinalOutput) bool {
		return f.Kind == "image" && f.ParentID == nil && len(f.JobIDs) == 0 && f.BriefVersion == version && (expectedPrompt == "" || (f.Prompt == expectedPrompt && f.Reason == reason))
	}
	if saved, err := s.repo.Get(ctx, user, project, key); err == nil {
		if !matches(saved) {
			return nil, ErrInvalidDirectImage
		}
		return saved, nil
	} else if !errors.Is(err, entity.ErrNotFound) {
		return nil, err
	}
	if s.provider == nil {
		return nil, ErrGenerationUnavailable
	}
	f := &entity.FinalOutput{RequestKey: &key, BriefVersion: version, Kind: "image", JobIDs: []string{}, Outputs: []string{}, Status: "submitting", Prompt: expectedPrompt, Reason: reason}
	claimed, err := s.repo.Claim(ctx, user, project, f)
	if err != nil {
		return nil, err
	}
	if !claimed {
		saved, err := s.repo.Get(ctx, user, project, key)
		if err != nil {
			return nil, err
		}
		if !matches(saved) {
			return nil, ErrInvalidDirectImage
		}
		return saved, nil
	}
	result, submitErr := s.provider.Submit(ctx, "image", f.Prompt)
	return s.saveSubmission(ctx, f, f, result, submitErr)
}
