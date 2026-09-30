package inputs

import (
	"context"
	"errors"
	"github.com/agentclash/agentclash/backend/internal/storage"
	"time"
)

// Staging is recorded before object storage. An uncertain database commit can
// therefore be reconciled without deleting a blob that actually committed.
func (s *Repository) cleanupStaging(ctx context.Context) error {
	rows, err := s.DB.Query(ctx, `SELECT object_key,EXISTS(SELECT 1 FROM vibe_inputs i WHERE i.object_key=t.object_key) FROM vibe_input_staging t WHERE created_at<now()-interval '1 hour' LIMIT 50`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var committed bool
		if err = rows.Scan(&key, &committed); err != nil {
			return err
		}
		if !committed {
			if err = s.deleteBlob(ctx, key); err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
				return err
			}
		}
		if _, err = s.DB.Exec(ctx, `DELETE FROM vibe_input_staging WHERE object_key=$1`, key); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Repository) deleteBlob(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.Blobs.DeleteObject(ctx, key)
}
