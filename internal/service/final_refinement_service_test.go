package service

import (
	"context"
	"errors"
	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/higgsfield"
	"strings"
	"testing"
)

type finalRepoStub struct{ base, job *entity.FinalOutput }

func (r *finalRepoStub) Get(_ context.Context, user, project, id string) (*entity.FinalOutput, error) {
	if user != "owner" || project != "project" {
		return nil, entity.ErrNotFound
	}
	var f *entity.FinalOutput
	if id == "source" {
		f = r.base
	} else if r.job != nil && (id == r.job.ID || id == *r.job.RequestKey) {
		f = r.job
	}
	if f == nil {
		return nil, entity.ErrNotFound
	}
	copy := *f
	return &copy, nil
}
func (r *finalRepoStub) Claim(_ context.Context, user, project string, f *entity.FinalOutput) (bool, error) {
	if user != "owner" || project != "project" {
		return false, entity.ErrNotFound
	}
	f.ID = "candidate"
	copy := *f
	r.job = &copy
	return true, nil
}
func (r *finalRepoStub) Save(_ context.Context, f *entity.FinalOutput) error {
	copy := *f
	r.job = &copy
	return nil
}
func (r *finalRepoStub) Accept(context.Context, string, string, string) (*entity.FinalOutput, error) {
	return nil, nil
}

type finalEditorStub struct {
	calls     int
	reference string
	fail      bool
	prompt    string
}

func (p *finalEditorStub) Submit(_ context.Context, kind, prompt string) (higgsfield.Job, error) {
	p.calls++
	p.prompt = prompt
	return higgsfield.Job{RequestID: "request", Status: "queued"}, nil
}
func (p *finalEditorStub) Refine(_ context.Context, kind, prompt, reference string) (higgsfield.Job, error) {
	p.calls++
	p.reference = reference
	p.prompt = prompt
	if !strings.Contains(prompt, "Warm lighting") {
		return higgsfield.Job{}, errors.New("missing instructions")
	}
	if p.fail {
		return higgsfield.Job{}, errors.New("ambiguous timeout")
	}
	return higgsfield.Job{RequestID: "request", Status: "queued"}, nil
}
func (p *finalEditorStub) Status(context.Context, string) (higgsfield.Job, error) {
	return higgsfield.Job{RequestID: "request", Status: "completed", Images: []string{"https://example.com/refined.png"}, Video: "https://example.com/refined.mp4"}, nil
}
func TestFinalRefinementOwnershipIdempotencyAndCollection(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"image", "video"} {
		t.Run(kind, func(t *testing.T) {
			base := &entity.FinalOutput{ID: "source", Kind: kind, BriefVersion: 2, Status: "completed", Accepted: true, Outputs: []string{"https://example.com/one.png", "https://example.com/two.png"}}
			repo := &finalRepoStub{base: base}
			provider := &finalEditorStub{}
			s := NewFinalRefinementService(repo, provider, nil)
			index := 0
			if kind == "image" {
				index = 1
			}
			if _, err := s.Refine(ctx, "other", "project", "source", "key", index, "Warm lighting", "Director feedback"); !errors.Is(err, entity.ErrNotFound) || provider.calls != 0 {
				t.Fatal("cross-owner generation", err)
			}
			f, err := s.Refine(ctx, "owner", "project", "source", "key", index, "Warm lighting", "Director feedback")
			if err != nil || f.Status != "queued" || f.Accepted {
				t.Fatal("submission", f, err)
			}
			if provider.reference != base.Outputs[index] {
				t.Fatal("wrong source", provider.reference)
			}
			if _, err = s.Refine(ctx, "owner", "project", "source", "key", index, "Warm lighting", "Director feedback"); err != nil || provider.calls != 1 {
				t.Fatal("duplicate paid edit", err)
			}
			if _, err = s.Refine(ctx, "owner", "project", "source", "key", index, "Different", "Director feedback"); !errors.Is(err, ErrInvalidFinalRefinement) {
				t.Fatal("key payload changed", err)
			}
			f, err = s.Get(ctx, "owner", "project", f.ID)
			if err != nil || f.Status != "completed" || f.Accepted {
				t.Fatal("completion", f, err)
			}
			if kind == "image" && (len(f.Outputs) != 2 || f.Outputs[0] != base.Outputs[0] || f.Outputs[1] != "https://example.com/refined.png") {
				t.Fatal("unrelated image replaced", f.Outputs)
			}
			if kind == "video" && (len(f.Outputs) != 1 || f.Outputs[0] != "https://example.com/refined.mp4") {
				t.Fatal("video lost", f.Outputs)
			}
			if base.Outputs[1] != "https://example.com/two.png" {
				t.Fatal("source history mutated")
			}
		})
	}
}
func TestFinalRefinementUnknownAndInvalidSource(t *testing.T) {
	ctx := context.Background()
	repo := &finalRepoStub{base: &entity.FinalOutput{Kind: "image", Status: "completed", Outputs: []string{"https://example.com/a.png"}}}
	provider := &finalEditorStub{fail: true}
	s := NewFinalRefinementService(repo, provider, nil)
	f, err := s.Refine(ctx, "owner", "project", "source", "key", 0, "Warm lighting", "Reason")
	if err != nil || f.Status != "unknown" {
		t.Fatal(f, err)
	}
	if _, err = s.Refine(ctx, "owner", "project", "source", "key", 0, "Warm lighting", "Reason"); err != nil || provider.calls != 1 {
		t.Fatal("ambiguous submission resubmitted", err)
	}
	repo.job = nil
	repo.base.Outputs[0] = "/demo-assets/a.png"
	if _, err = s.Refine(ctx, "owner", "project", "source", "new", 0, "Warm lighting", "Reason"); !errors.Is(err, ErrFinalReference) || provider.calls != 1 {
		t.Fatal("local reference submitted", err)
	}
	if _, err = s.Refine(ctx, "owner", "project", "source", "new", 1, "Warm lighting", "Reason"); !errors.Is(err, ErrInvalidFinalRefinement) {
		t.Fatal("invalid image index", err)
	}
}

func TestFinalVideoUsesFreshR2Reference(t *testing.T) {
	repo := &finalRepoStub{base: &entity.FinalOutput{Kind: "video", Status: "completed", ObjectKey: "private/final.mp4", Outputs: []string{}}}
	provider := &finalEditorStub{}
	service := NewFinalRefinementService(repo, provider, &briefStorageStub{})
	if _, err := service.Refine(context.Background(), "owner", "project", "source", "key", 0, "Warm lighting", "Reason"); err != nil || provider.reference != "https://r2.example/download" {
		t.Fatal("R2 source not resolved", provider.reference, err)
	}
}

func TestDirectImageGenerationUsesFinalHistory(t *testing.T) {
	ctx := context.Background()
	repo := &finalRepoStub{}
	provider := &finalEditorStub{}
	service := NewFinalRefinementService(repo, provider, nil)
	if _, err := service.GenerateImage(ctx, "other", "project", "key", 1); !errors.Is(err, entity.ErrNotFound) || provider.calls != 0 {
		t.Fatal("cross-owner direct image", err)
	}
	f, err := service.GenerateImage(ctx, "owner", "project", "key", 1)
	if err != nil || f.Status != "queued" || f.ParentID != nil || len(f.JobIDs) != 0 || f.Accepted {
		t.Fatal("direct image incorrectly requires a plan", f, err)
	}
	if _, err = service.GenerateImage(ctx, "owner", "project", "key", 1); err != nil || provider.calls != 1 {
		t.Fatal("duplicate image charge", err)
	}
	if _, err = service.GenerateImage(ctx, "owner", "project", "key", 2); !errors.Is(err, ErrInvalidDirectImage) {
		t.Fatal("key reused for a different brief", err)
	}
	f, err = service.Get(ctx, "owner", "project", f.ID)
	if err != nil || f.Status != "completed" || len(f.Outputs) != 1 || f.Outputs[0] != "https://example.com/refined.png" || f.Accepted {
		t.Fatal("direct image result", f, err)
	}
}
