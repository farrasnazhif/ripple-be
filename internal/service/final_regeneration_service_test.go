package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

func TestRefinementReasonIsOptional(t *testing.T) {
	repo := &finalRepoStub{base: &entity.FinalOutput{Kind: "image", Status: "completed", Outputs: []string{"https://example.com/image.png"}}}
	provider := &finalEditorStub{}
	s := NewFinalRefinementService(repo, provider, nil)
	f, err := s.Refine(context.Background(), "owner", "project", "source", "key", 0, "Warm lighting", "  ")
	if err != nil || f.Reason != "" || provider.calls != 1 {
		t.Fatal("optional reason rejected", f, err)
	}
	if _, err := s.Refine(context.Background(), "owner", "project", "source", "new", 0, "Warm lighting", strings.Repeat("a", 2001)); !errors.Is(err, ErrInvalidFinalRefinement) {
		t.Fatal("reason length limit bypassed", err)
	}
}

func TestRegenerationUsesExactSavedPromptAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	const prompt = "One blue bottle.\nConfirmed requirements: keep soft morning light."
	repo := &finalRepoStub{base: &entity.FinalOutput{ID: "source", Kind: "image", BriefVersion: 2, Status: "completed", Prompt: prompt, Outputs: []string{"https://example.com/original.png"}}}
	provider := &finalEditorStub{}
	s := NewFinalRefinementService(repo, provider, nil)
	if _, err := s.Regenerate(ctx, "other", "project", "source", "key", 0); !errors.Is(err, entity.ErrNotFound) || provider.calls != 0 {
		t.Fatal("cross-owner regeneration", err)
	}
	f, err := s.Regenerate(ctx, "owner", "project", "source", "key", 0)
	if err != nil || f.Prompt != prompt || provider.prompt != prompt || f.Accepted || repo.base.Outputs[0] != "https://example.com/original.png" {
		t.Fatal("prompt or history changed", f, err)
	}
	if _, err := s.Regenerate(ctx, "owner", "project", "source", "key", 0); err != nil || provider.calls != 1 {
		t.Fatal("duplicate paid regeneration", err)
	}
	if _, err := s.Regenerate(ctx, "owner", "project", "source", "second-key", 0); err != nil || provider.calls != 2 {
		t.Fatal("new iteration was not submitted", err)
	}
	repo.base.Prompt = ""
	if _, err := s.Regenerate(ctx, "owner", "project", "source", "third-key", 0); !errors.Is(err, ErrFinalRegeneration) || provider.calls != 2 {
		t.Fatal("missing saved prompt submitted", err)
	}
}

func TestRegenerationRepeatsRefinementWithOriginalReference(t *testing.T) {
	ctx := context.Background()
	repo := &finalRepoStub{base: &entity.FinalOutput{ID: "source", Kind: "video", BriefVersion: 2, Status: "completed", Outputs: []string{"https://example.com/original.mp4"}}}
	provider := &finalEditorStub{}
	s := NewFinalRefinementService(repo, provider, nil)
	f, err := s.Refine(ctx, "owner", "project", "source", "first-key", 0, "Warm lighting", "")
	if err != nil {
		t.Fatal(err)
	}
	originalPrompt := provider.prompt
	if _, err := s.Get(ctx, "owner", "project", f.ID); err != nil {
		t.Fatal(err)
	}
	f, err = s.Regenerate(ctx, "owner", "project", "candidate", "second-key", 0)
	if err != nil || provider.prompt != originalPrompt || provider.reference != "https://example.com/original.mp4" || *f.ParentID != "source" {
		t.Fatal("saved refinement request changed", f, err)
	}
}
