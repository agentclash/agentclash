package vibe

import (
	"context"

	"github.com/google/uuid"
)

// Administrative inventory contains no user content. Formats, not authoring
// versions, define deletion eligibility; current Improve v15 is preserved.
type RetirementInventory struct {
	PreV1             int `json:"pre_v1_sessions"`
	V1                int `json:"v1_sessions"`
	PendingOperations int `json:"pending_retiring_operations"`
	PendingOutbox     int `json:"pending_retiring_outbox"`
}

const retiringOperation = `(s.document->>'format_version' IS DISTINCT FROM '1' OR (o.kind IN ('message','build') AND COALESCE(o.input->>'authoring_version','0') NOT IN ('15','18','20')))`

func (s *Store) RetirementInventory(ctx context.Context) (RetirementInventory, error) {
	var out RetirementInventory
	err := s.DB.QueryRow(ctx, `SELECT count(*) FILTER(WHERE document->>'format_version' IS DISTINCT FROM '1'),count(*) FILTER(WHERE document->>'format_version'='1') FROM vibe_sessions WHERE deleted_at IS NULL`).Scan(&out.PreV1, &out.V1)
	if err != nil {
		return out, err
	}
	err = s.DB.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE b.operation_id IS NOT NULL AND b.delivered_at IS NULL) FROM vibe_operations o JOIN vibe_sessions s ON s.id=o.session_id LEFT JOIN vibe_outbox b ON b.operation_id=o.id WHERE o.state NOT IN ('COMPLETED','PARTIAL','FAILED','CANCELLED','EXPIRED') AND `+retiringOperation).Scan(&out.PendingOperations, &out.PendingOutbox)
	return out, err
}

// RetirePreV1 is intentionally an internal administrative operation, not an
// HTTP endpoint. Call only after checking Temporal and draining old contracts.
func (s *Store) RetirePreV1(ctx context.Context) (int, error) {
	inventory, err := s.RetirementInventory(ctx)
	if err != nil {
		return 0, err
	}
	if inventory.PendingOperations != 0 || inventory.PendingOutbox != 0 {
		return 0, fault("retirement_pending", "Drain retiring operations before content cleanup.")
	}
	rows, err := s.DB.Query(ctx, `SELECT id,actor,revision FROM vibe_sessions WHERE deleted_at IS NULL AND document->>'format_version' IS DISTINCT FROM '1' ORDER BY id`)
	if err != nil {
		return 0, err
	}
	type item struct {
		id       uuid.UUID
		actor    string
		revision int64
	}
	items := []item{}
	for rows.Next() {
		var i item
		if err = rows.Scan(&i.id, &i.actor, &i.revision); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for index, i := range items {
		if _, err = s.deleteProject(ctx, i.actor, i.id, i.revision, true); err != nil {
			return index, err
		}
	}
	var pending int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM vibe_sessions WHERE `+cleanupEligible).Scan(&pending); err != nil {
		return len(items), err
	}
	for batch := 0; batch <= pending/20; batch++ {
		if err = s.CleanupProjects(ctx); err != nil {
			return len(items), err
		}
	}
	return len(items), nil
}
