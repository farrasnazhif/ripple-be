package repository

import (
	"context"
	"database/sql"
)

type StorageCleanupRepository struct{ db *sql.DB }

func NewStorageCleanupRepository(db *sql.DB) *StorageCleanupRepository {
	return &StorageCleanupRepository{db}
}
func (r *StorageCleanupRepository) Pending(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `SELECT q.object_key FROM storage_cleanup q WHERE q.not_before<=now() AND NOT EXISTS(SELECT 1 FROM brief_attachments a WHERE a.object_key=q.object_key) AND NOT EXISTS(SELECT 1 FROM project_final_outputs f WHERE f.object_key=q.object_key) ORDER BY q.not_before LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}
func (r *StorageCleanupRepository) Complete(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `DELETE FROM storage_cleanup WHERE object_key=$1`, key)
	return err
}
