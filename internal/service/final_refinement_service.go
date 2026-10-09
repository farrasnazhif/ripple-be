package service

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/higgsfield"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

var ErrInvalidFinalRefinement = errors.New("choose a completed final output, a valid image, and provide refinement instructions (up to 10000 characters) and a reason (up to 2000 characters)")
var ErrFinalReference = errors.New("the final output needs a publicly reachable HTTPS reference; local demo assets cannot be sent to Higgsfield")

type FinalMediaEditor interface {
	Refine(context.Context, string, string, string) (higgsfield.Job, error)
	Status(context.Context, string) (higgsfield.Job, error)
}
type FinalRefinementService struct {
	repo     repository.FinalRefinementRepository
	provider FinalMediaEditor
	storage  BriefStorage
}

func NewFinalRefinementService(r repository.FinalRefinementRepository, p FinalMediaEditor, storage BriefStorage) *FinalRefinementService {
	return &FinalRefinementService{r, p, storage}
}
func (s *FinalRefinementService) Refine(ctx context.Context, user, project, source, key string, index int, prompt, reason string) (*entity.FinalOutput, error) {
	prompt, reason = strings.TrimSpace(prompt), strings.TrimSpace(reason)
	if prompt == "" || len([]rune(prompt)) > 10000 || reason == "" || len([]rune(reason)) > 2000 || index < 0 {
		return nil, ErrInvalidFinalRefinement
	}
	base, err := s.repo.Get(ctx, user, project, source)
	if err != nil {
		return nil, err
	}
	if base.Status != "completed" || (base.Kind == "image" && index >= len(base.Outputs)) || (base.Kind == "video" && index != 0) {
		return nil, ErrInvalidFinalRefinement
	}
	matches := func(f *entity.FinalOutput) bool {
		return f.ParentID != nil && *f.ParentID == source && f.ImageIndex == index && f.Prompt == prompt && f.Reason == reason
	}
	if saved, e := s.repo.Get(ctx, user, project, key); e == nil {
		if !matches(saved) {
			return nil, ErrInvalidFinalRefinement
		}
		return saved, nil
	} else if !errors.Is(e, entity.ErrNotFound) {
		return nil, e
	}
	if s.provider == nil {
		return nil, ErrGenerationUnavailable
	}
	reference := ""
	if base.ObjectKey != "" {
		if s.storage == nil {
			return nil, ErrStorageUnavailable
		}
		reference, err = s.storage.PresignDownload(ctx, base.ObjectKey)
		if err != nil {
			return nil, err
		}
	} else if len(base.Outputs) > index {
		reference = base.Outputs[index]
	}
	parsed, e := url.Parse(reference)
	if e != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, ErrFinalReference
	}
	f := &entity.FinalOutput{ParentID: &source, RequestKey: &key, ImageIndex: index, Prompt: prompt, Reason: reason, Kind: base.Kind, BriefVersion: base.BriefVersion, JobIDs: base.JobIDs, Outputs: []string{}, Status: "submitting"}
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
			return nil, ErrInvalidFinalRefinement
		}
		return saved, nil
	}
	// shortcut: ambiguous paid submissions need provider-dashboard reconciliation; never resubmit automatically.
	result, submitErr := s.provider.Refine(ctx, f.Kind, "Apply only these adjustments to the supplied final output. Preserve unrelated content and constraints.\n"+prompt, reference)
	if submitErr != nil {
		f.Status = "unknown"
		if errors.Is(submitErr, higgsfield.ErrSubmissionRejected) {
			f.Status = "failed"
		}
	} else {
		applyFinalProvider(f, base, result)
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), querySaveTimeout)
	defer cancel()
	if err = s.repo.Save(saveCtx, f); err != nil {
		return nil, err
	}
	return f, nil
}
func applyFinalProvider(f, base *entity.FinalOutput, result higgsfield.Job) {
	j := &entity.GenerationJob{Kind: f.Kind}
	applyProvider(j, result)
	f.ProviderID, f.Status = result.RequestID, j.Status
	f.Outputs = []string{}
	if f.Status == "completed" {
		if f.Kind == "image" {
			f.Outputs = append([]string{}, base.Outputs...)
			f.Outputs[f.ImageIndex] = j.Outputs[0]
		} else {
			f.Outputs = j.Outputs
		}
	}
}
func (s *FinalRefinementService) Get(ctx context.Context, user, project, id string) (*entity.FinalOutput, error) {
	f, err := s.repo.Get(ctx, user, project, id)
	if err != nil {
		return nil, err
	}
	if s.provider != nil && f.ParentID != nil && f.ProviderID != "" && generationPending(f.Status) {
		base, err := s.repo.Get(ctx, user, project, *f.ParentID)
		if err != nil {
			return nil, err
		}
		result, err := s.provider.Status(ctx, f.ProviderID)
		if err != nil {
			return nil, err
		}
		applyFinalProvider(f, base, result)
		if err = s.repo.Save(ctx, f); err != nil {
			return nil, err
		}
	}
	if f.ObjectKey != "" && s.storage != nil {
		f.DownloadURL, err = s.storage.PresignDownload(ctx, f.ObjectKey)
	}
	return f, err
}
func (s *FinalRefinementService) Accept(ctx context.Context, user, project, id string) (*entity.FinalOutput, error) {
	return s.repo.Accept(ctx, user, project, id)
}
