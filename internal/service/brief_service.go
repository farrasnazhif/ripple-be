package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"strings"

	"github.com/farrasnazhif/ripple-be/internal/entity"
	"github.com/farrasnazhif/ripple-be/internal/repository"
)

const maxBriefAttachmentBytes = 25 << 20

var ErrInvalidAttachment = errors.New("invalid attachment")
var ErrStorageUnavailable = errors.New("attachment storage unavailable")
var ErrEmptySummary = errors.New("summary is empty")
var ErrAIUnavailable = errors.New("ChatGPT API is not configured")
var ErrNoBriefText = errors.New("brief text is empty")

type BriefStorage interface {
	PresignUpload(ctx context.Context, key, contentType string) (string, error)
	PresignDownload(ctx context.Context, key string) (string, error)
	Stat(ctx context.Context, key string) (int64, string, error)
	Delete(ctx context.Context, key string) error
}

type BriefSummarizer interface {
	SummarizeBrief(ctx context.Context, rawText string) (string, entity.TokenUsage, error)
}

type BriefService struct {
	repo       repository.BriefRepository
	storage    BriefStorage
	summarizer BriefSummarizer
}

func NewBriefService(repo repository.BriefRepository, storage BriefStorage, summarizer BriefSummarizer) *BriefService {
	return &BriefService{repo: repo, storage: storage, summarizer: summarizer}
}

func (s *BriefService) Create(ctx context.Context, userID, rawText string) (*entity.Brief, error) {
	return s.repo.Create(ctx, userID, rawText)
}

func (s *BriefService) Get(ctx context.Context, userID, briefID string) (*entity.Brief, error) {
	return s.repo.Get(ctx, userID, briefID)
}

func (s *BriefService) DraftSummary(ctx context.Context, userID, briefID string) (string, entity.TokenUsage, error) {
	if s.summarizer == nil {
		return "", entity.TokenUsage{}, ErrAIUnavailable
	}
	brief, err := s.repo.Get(ctx, userID, briefID)
	if err != nil {
		return "", entity.TokenUsage{}, err
	}
	if strings.TrimSpace(brief.RawText) == "" {
		return "", entity.TokenUsage{}, ErrNoBriefText
	}
	return s.summarizer.SummarizeBrief(ctx, brief.RawText)
}

func (s *BriefService) ConfirmSummary(ctx context.Context, userID, briefID, summary string) error {
	if strings.TrimSpace(summary) == "" {
		return ErrEmptySummary
	}
	return s.repo.ConfirmSummary(ctx, userID, briefID, summary)
}

func (s *BriefService) StartAttachment(ctx context.Context, userID, briefID, fileName, contentType string, size int64) (*entity.BriefAttachment, string, error) {
	if s.storage == nil {
		return nil, "", ErrStorageUnavailable
	}
	if strings.TrimSpace(fileName) == "" || len(fileName) > 255 || strings.ContainsAny(fileName, "/\\") {
		return nil, "", ErrInvalidAttachment
	}
	if size <= 0 || size > maxBriefAttachmentBytes {
		return nil, "", ErrInvalidAttachment
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType == "" || mediaType == "*/*" {
		return nil, "", ErrInvalidAttachment
	}
	if _, err := s.repo.Get(ctx, userID, briefID); err != nil {
		return nil, "", err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, "", err
	}
	a := &entity.BriefAttachment{
		BriefID: briefID, ObjectKey: "briefs/" + briefID + "/" + hex.EncodeToString(random[:]),
		FileName: fileName, ContentType: mediaType, SizeBytes: size,
	}
	url, err := s.storage.PresignUpload(ctx, a.ObjectKey, a.ContentType)
	if err != nil {
		return nil, "", err
	}
	if err := s.repo.CreateAttachment(ctx, userID, a); err != nil {
		return nil, "", err
	}
	// shortcut: abandoned pending uploads are not swept, add R2 lifecycle cleanup before production.
	return a, url, nil
}

func (s *BriefService) CompleteAttachment(ctx context.Context, userID, briefID, attachmentID string) (*entity.BriefAttachment, error) {
	if s.storage == nil {
		return nil, ErrStorageUnavailable
	}
	a, err := s.repo.GetAttachment(ctx, userID, briefID, attachmentID)
	if err != nil {
		return nil, err
	}
	if a.Status == "completed" {
		return a, nil
	}
	size, contentType, err := s.storage.Stat(ctx, a.ObjectKey)
	if err != nil {
		return nil, err
	}
	if size != a.SizeBytes || contentType != a.ContentType {
		if err := s.storage.Delete(ctx, a.ObjectKey); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: size or content type mismatch", ErrInvalidAttachment)
	}
	if err := s.repo.CompleteAttachment(ctx, userID, briefID, attachmentID, size); err != nil {
		return nil, err
	}
	a.Status, a.SizeBytes = "completed", size
	return a, nil
}

func (s *BriefService) ListAttachments(ctx context.Context, userID, briefID string) ([]entity.BriefAttachment, error) {
	if _, err := s.repo.Get(ctx, userID, briefID); err != nil {
		return nil, err
	}
	return s.repo.ListAttachments(ctx, userID, briefID)
}

func (s *BriefService) DownloadAttachment(ctx context.Context, userID, briefID, attachmentID string) (string, error) {
	if s.storage == nil {
		return "", ErrStorageUnavailable
	}
	a, err := s.repo.GetAttachment(ctx, userID, briefID, attachmentID)
	if err != nil {
		return "", err
	}
	if a.Status != "completed" {
		return "", ErrInvalidAttachment
	}
	return s.storage.PresignDownload(ctx, a.ObjectKey)
}
