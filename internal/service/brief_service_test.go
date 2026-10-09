package service

import (
	"context"
	"errors"
	"testing"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

type briefRepoStub struct {
	brief      entity.Brief
	attachment entity.BriefAttachment
	completed  bool
}

func (r *briefRepoStub) Create(context.Context, string, string) (*entity.Brief, error) {
	return &r.brief, nil
}
func (r *briefRepoStub) Get(_ context.Context, userID, briefID string) (*entity.Brief, error) {
	if userID != "owner" || briefID != r.brief.ID {
		return nil, entity.ErrNotFound
	}
	return &r.brief, nil
}
func (r *briefRepoStub) CreateAttachment(_ context.Context, userID string, a *entity.BriefAttachment) error {
	if userID != "owner" {
		return entity.ErrNotFound
	}
	a.ID, a.Status = "attachment", "pending"
	r.attachment = *a
	return nil
}
func (r *briefRepoStub) GetAttachment(_ context.Context, userID, briefID, attachmentID string) (*entity.BriefAttachment, error) {
	if userID != "owner" || briefID != r.brief.ID || attachmentID != r.attachment.ID {
		return nil, entity.ErrNotFound
	}
	return &r.attachment, nil
}
func (r *briefRepoStub) ListAttachments(context.Context, string, string) ([]entity.BriefAttachment, error) {
	return []entity.BriefAttachment{r.attachment}, nil
}
func (r *briefRepoStub) CompleteAttachment(_ context.Context, userID, briefID, attachmentID string, size int64) error {
	if userID != "owner" || briefID != r.brief.ID || attachmentID != r.attachment.ID {
		return entity.ErrNotFound
	}
	r.completed = true
	return nil
}
func (r *briefRepoStub) ConfirmSummary(context.Context, string, string, string) error { return nil }

type briefStorageStub struct {
	size    int64
	deleted bool
}

func (s *briefStorageStub) PresignUpload(context.Context, string, string) (string, error) {
	return "https://r2.example/upload", nil
}
func (s *briefStorageStub) PresignDownload(context.Context, string) (string, error) {
	return "https://r2.example/download", nil
}
func (s *briefStorageStub) Stat(context.Context, string) (int64, string, error) {
	return s.size, "image/png", nil
}
func (s *briefStorageStub) Delete(context.Context, string) error { s.deleted = true; return nil }

func TestBriefAttachmentVerification(t *testing.T) {
	repo := &briefRepoStub{brief: entity.Brief{ID: "brief"}}
	storage := &briefStorageStub{size: 10}
	svc := NewBriefService(repo, storage, nil)
	if _, _, err := svc.StartAttachment(context.Background(), "other", "brief", "image.png", "image/png", 10); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("other user started attachment: %v", err)
	}
	a, url, err := svc.StartAttachment(context.Background(), "owner", "brief", "image.png", "image/png", 10)
	if err != nil || url == "" || a.SizeBytes != 10 {
		t.Fatalf("start: %+v %q %v", a, url, err)
	}
	storage.size = 11
	if _, err := svc.CompleteAttachment(context.Background(), "owner", "brief", a.ID); !errors.Is(err, ErrInvalidAttachment) || !storage.deleted || repo.completed {
		t.Fatalf("mismatch accepted or not deleted: %v", err)
	}
	storage.size = 10
	if _, err := svc.CompleteAttachment(context.Background(), "owner", "brief", a.ID); err != nil || !repo.completed {
		t.Fatalf("valid attachment rejected: %v", err)
	}
}
