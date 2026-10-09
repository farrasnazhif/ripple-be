package service

import (
	"context"
	"errors"
	"testing"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/higgsfield"
)

type shotRepoStub struct{}

func (shotRepoStub) List(context.Context, string, string) ([]entity.StoryboardShot, error) {
	return nil, nil
}
func (shotRepoStub) Create(context.Context, string, string, string, []entity.StoryboardShot) error {
	return nil
}
func (shotRepoStub) GetShot(_ context.Context, user, project, shot string) (*entity.StoryboardShot, error) {
	if user != "owner" {
		return nil, entity.ErrNotFound
	}
	return &entity.StoryboardShot{ID: shot, Prompt: "Saved prompt"}, nil
}

type jobRepoStub struct{ job *entity.GenerationJob }

func (r *jobRepoStub) Claim(_ context.Context, j *entity.GenerationJob) (bool, error) {
	if r.job != nil {
		return false, nil
	}
	j.ID = "job"
	copy := *j
	r.job = &copy
	return true, nil
}
func (r *jobRepoStub) Save(_ context.Context, j *entity.GenerationJob) error {
	copy := *j
	r.job = &copy
	return nil
}
func (r *jobRepoStub) Get(_ context.Context, user, project, id string) (*entity.GenerationJob, error) {
	if user != "owner" || r.job == nil {
		return nil, entity.ErrNotFound
	}
	copy := *r.job
	return &copy, nil
}
func (r *jobRepoStub) List(context.Context, string, string) ([]entity.GenerationJob, error) {
	return []entity.GenerationJob{*r.job}, nil
}

type generatorStub struct {
	calls int
	fail  bool
}

func (p *generatorStub) Submit(context.Context, string, string) (higgsfield.Job, error) {
	p.calls++
	if p.fail {
		return higgsfield.Job{}, errors.New("timeout")
	}
	return higgsfield.Job{RequestID: "provider", Status: "queued"}, nil
}
func (p *generatorStub) Status(context.Context, string) (higgsfield.Job, error) {
	return higgsfield.Job{RequestID: "provider", Status: "completed", Images: []string{"https://example.com/image.png"}, Video: "https://example.com/video.mp4"}, nil
}
func TestGenerationOwnershipIdempotencyAndCompletion(t *testing.T) {
	for _, kind := range []string{"image", "video"} {
		t.Run(kind, func(t *testing.T) {
			repo := &jobRepoStub{}
			provider := &generatorStub{}
			s := NewGenerationService(shotRepoStub{}, repo, provider)
			ctx := context.Background()
			if _, err := s.Submit(ctx, "other", "project", "shot", kind, "key"); !errors.Is(err, entity.ErrNotFound) || provider.calls != 0 {
				t.Fatal("unauthorized submission")
			}
			j, err := s.Submit(ctx, "owner", "project", "shot", kind, "key")
			if err != nil || j.Status != "queued" {
				t.Fatalf("submit: %+v %v", j, err)
			}
			if _, err = s.Submit(ctx, "owner", "project", "shot", kind, "key"); err != nil || provider.calls != 1 {
				t.Fatal("duplicate paid submission")
			}
			if _, err = s.Submit(ctx, "owner", "project", "different-shot", kind, "key"); !errors.Is(err, ErrInvalidGeneration) {
				t.Fatal("key reused for another shot")
			}
			j, err = s.Get(ctx, "owner", "project", "job")
			if err != nil || j.Status != "completed" || len(j.Outputs) != 1 {
				t.Fatalf("completion: %+v %v", j, err)
			}
		})
	}
}
func TestAmbiguousGenerationNeverResubmits(t *testing.T) {
	repo := &jobRepoStub{}
	provider := &generatorStub{fail: true}
	s := NewGenerationService(shotRepoStub{}, repo, provider)
	ctx := context.Background()
	j, err := s.Submit(ctx, "owner", "project", "shot", "image", "key")
	if err != nil || j.Status != "unknown" {
		t.Fatalf("unknown request not persisted: %+v %v", j, err)
	}
	_, err = s.Submit(ctx, "owner", "project", "shot", "image", "key")
	if err != nil || provider.calls != 1 {
		t.Fatal("ambiguous request retried")
	}
}

func TestRefinementPromptAndIdempotency(t *testing.T) {
	repo := &jobRepoStub{}
	provider := &generatorStub{}
	s := NewGenerationService(shotRepoStub{}, repo, provider)
	ctx := context.Background()
	j, err := s.Submit(ctx, "owner", "project", "shot", "image", "refinement-key", "Evening light, preserve the greens")
	if err != nil || j.Prompt != "Evening light, preserve the greens" {
		t.Fatal(j, err)
	}
	if _, err = s.Submit(ctx, "owner", "project", "shot", "image", "refinement-key", "Evening light, preserve the greens"); err != nil || provider.calls != 1 {
		t.Fatal("refinement double charged", err)
	}
	if _, err = s.Submit(ctx, "owner", "project", "shot", "image", "refinement-key", "Different prompt"); !errors.Is(err, ErrInvalidGeneration) || provider.calls != 1 {
		t.Fatal("idempotency key accepted changed prompt", err)
	}
}
