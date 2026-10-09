package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/farrasnazhif/ripple-be/internal/entity"
)

type BriefRepository interface {
	Create(ctx context.Context, userID, rawText string) (*entity.Brief, error)
	Get(ctx context.Context, userID, briefID string) (*entity.Brief, error)
	CreateAttachment(ctx context.Context, userID string, attachment *entity.BriefAttachment) error
	GetAttachment(ctx context.Context, userID, briefID, attachmentID string) (*entity.BriefAttachment, error)
	ListAttachments(ctx context.Context, userID, briefID string) ([]entity.BriefAttachment, error)
	CompleteAttachment(ctx context.Context, userID, briefID, attachmentID string, size int64) error
	ConfirmSummary(ctx context.Context, userID, briefID, summary string) error
}

type briefRepository struct{ db *sql.DB }

func NewBriefRepository(db *sql.DB) BriefRepository { return &briefRepository{db: db} }

func (r *briefRepository) Create(ctx context.Context, userID, rawText string) (*entity.Brief, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	b := &entity.Brief{RawText: rawText}
	err := r.db.QueryRowContext(ctx, `INSERT INTO briefs (user_id, raw_text) VALUES ($1, $2) RETURNING id, version`, userID, rawText).Scan(&b.ID, &b.Version)
	return b, err
}

func (r *briefRepository) Get(ctx context.Context, userID, briefID string) (*entity.Brief, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	b := &entity.Brief{}
	err := r.db.QueryRowContext(ctx, `SELECT id, raw_text, version, summary FROM briefs WHERE id = $1 AND user_id = $2`, briefID, userID).Scan(&b.ID, &b.RawText, &b.Version, &b.Summary)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, entity.ErrNotFound
	}
	return b, err
}

func (r *briefRepository) ConfirmSummary(ctx context.Context, userID, briefID, summary string) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	result, err := r.db.ExecContext(ctx, `UPDATE briefs SET summary = $3 WHERE id = $1 AND user_id = $2`, briefID, userID, summary)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return entity.ErrNotFound
	}
	return nil
}

func (r *briefRepository) CreateAttachment(ctx context.Context, userID string, a *entity.BriefAttachment) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	err := r.db.QueryRowContext(ctx, `INSERT INTO brief_attachments (brief_id, object_key, file_name, content_type, size_bytes)
		SELECT id, $3, $4, $5, $6 FROM briefs WHERE id = $1 AND user_id = $2
		RETURNING id, status`, a.BriefID, userID, a.ObjectKey, a.FileName, a.ContentType, a.SizeBytes).Scan(&a.ID, &a.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.ErrNotFound
	}
	return err
}

func (r *briefRepository) ListAttachments(ctx context.Context, userID, briefID string) ([]entity.BriefAttachment, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT a.id, a.brief_id, a.object_key, a.file_name, a.content_type, a.size_bytes, a.status
		FROM brief_attachments a JOIN briefs b ON b.id = a.brief_id
		WHERE a.brief_id = $1 AND b.user_id = $2 AND a.status = 'completed' ORDER BY a.created_at`, briefID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attachments := []entity.BriefAttachment{}
	for rows.Next() {
		var a entity.BriefAttachment
		if err := rows.Scan(&a.ID, &a.BriefID, &a.ObjectKey, &a.FileName, &a.ContentType, &a.SizeBytes, &a.Status); err != nil {
			return nil, err
		}
		attachments = append(attachments, a)
	}
	return attachments, rows.Err()
}

func (r *briefRepository) GetAttachment(ctx context.Context, userID, briefID, attachmentID string) (*entity.BriefAttachment, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	a := &entity.BriefAttachment{}
	err := r.db.QueryRowContext(ctx, `SELECT a.id, a.brief_id, a.object_key, a.file_name, a.content_type, COALESCE(a.size_bytes, 0), a.status
		FROM brief_attachments a JOIN briefs b ON b.id = a.brief_id
		WHERE a.id = $1 AND a.brief_id = $2 AND b.user_id = $3`, attachmentID, briefID, userID).
		Scan(&a.ID, &a.BriefID, &a.ObjectKey, &a.FileName, &a.ContentType, &a.SizeBytes, &a.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, entity.ErrNotFound
	}
	return a, err
}

func (r *briefRepository) CompleteAttachment(ctx context.Context, userID, briefID, attachmentID string, size int64) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	result, err := r.db.ExecContext(ctx, `UPDATE brief_attachments a SET status = 'completed', size_bytes = $4
		FROM briefs b WHERE a.id = $1 AND a.brief_id = $2 AND b.id = a.brief_id AND b.user_id = $3 AND a.status = 'pending'`, attachmentID, briefID, userID, size)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return entity.ErrNotFound
	}
	return nil
}
