package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/higgsfield"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

var ErrInvalidGeneration = errors.New("invalid generation kind or idempotency key")
var ErrGenerationUnavailable = errors.New("Higgsfield is not configured")

type MediaGenerator interface {
	Submit(context.Context, string, string) (higgsfield.Job, error)
	Status(context.Context, string) (higgsfield.Job, error)
}
type GenerationService struct {
	shots    repository.StoryboardRepository
	repo     repository.GenerationRepository
	provider MediaGenerator
}

func NewGenerationService(s repository.StoryboardRepository, r repository.GenerationRepository, p MediaGenerator) *GenerationService {
	return &GenerationService{s, r, p}
}
func (s *GenerationService) Submit(ctx context.Context, user, project, shot, kind, key string, prompts ...string) (*entity.GenerationJob, error) {
	if kind != "image" && kind != "video" {
		return nil, ErrInvalidGeneration
	}
	source, err := s.shots.GetShot(ctx, user, project, shot)
	if err != nil {
		return nil, err
	}
	prompt := source.Prompt
	if len(prompts) > 0 && prompts[0] != "" {
		prompt = strings.TrimSpace(prompts[0])
	}
	if prompt == "" || len([]rune(prompt)) > 10000 {
		return nil, ErrInvalidGeneration
	}
	// Check ownership and the original payload before returning an idempotent result.
	if existing, err := s.repo.Get(ctx, user, project, key); err == nil {
		if existing.ShotID != shot || existing.Kind != kind || existing.Prompt != prompt {
			return nil, ErrInvalidGeneration
		}
		return existing, nil
	} else if !errors.Is(err, entity.ErrNotFound) {
		return nil, err
	}
	if s.provider == nil {
		return nil, ErrGenerationUnavailable
	}
	j := &entity.GenerationJob{ShotID: shot, Kind: kind, RequestKey: key, Prompt: prompt, Status: "submitting", Outputs: []string{}}
	claimed, err := s.repo.Claim(ctx, j)
	if err != nil {
		return nil, err
	}
	if !claimed {
		existing, err := s.repo.Get(ctx, user, project, key)
		if err != nil {
			return nil, err
		}
		if existing.ShotID != shot || existing.Kind != kind || existing.Prompt != prompt {
			return nil, ErrInvalidGeneration
		}
		return existing, nil
	}
	// shortcut: uncertain submissions require provider dashboard reconciliation, add a recovery UI before production.
	// Submission is never automatically retried: a timeout can still incur a provider charge.
	result, submitErr := s.provider.Submit(ctx, kind, j.Prompt)
	if submitErr != nil {
		j.Status = "unknown"
		if errors.Is(submitErr, higgsfield.ErrSubmissionRejected) {
			j.Status = "failed"
			j.SubmissionRejected = true
		}
	} else {
		applyProvider(j, result)
	}
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), querySaveTimeout)
	defer cancel()
	if err := s.repo.Save(saveCtx, j); err != nil {
		return nil, err
	}
	return j, nil
}

const querySaveTimeout = 5 * time.Second

func (s *GenerationService) Get(ctx context.Context, user, project, id string) (*entity.GenerationJob, error) {
	j, err := s.repo.Get(ctx, user, project, id)
	if err != nil {
		return nil, err
	}
	if s.provider == nil || j.ProviderID == "" || !generationPending(j.Status) {
		return j, nil
	}
	result, err := s.provider.Status(ctx, j.ProviderID)
	if err != nil {
		return nil, err
	}
	applyProvider(j, result)
	if err := s.repo.Save(ctx, j); err != nil {
		return nil, err
	}
	return j, nil
}
func (s *GenerationService) List(ctx context.Context, user, project string) ([]entity.GenerationJob, error) {
	return s.repo.List(ctx, user, project)
}
func generationPending(status string) bool { return status == "queued" || status == "in_progress" }
func applyProvider(j *entity.GenerationJob, p higgsfield.Job) {
	j.ProviderID = p.RequestID
	j.Status = strings.ToLower(p.Status)
	switch j.Status {
	case "completed", "failed", "nsfw", "canceled", "queued", "in_progress":
	default:
		j.Status = "queued"
	}
	outputs := p.Images
	if j.Kind == "video" {
		outputs = []string{p.Video}
	}
	j.Outputs = []string{}
	for _, value := range outputs {
		u, err := url.Parse(value)
		if err == nil && u.Scheme == "https" && u.Host != "" {
			j.Outputs = append(j.Outputs, value)
		}
	}
	if j.Status == "completed" && len(j.Outputs) == 0 {
		j.Status = "failed"
	}
}
