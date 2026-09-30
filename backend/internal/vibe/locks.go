package vibe

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Database writers acquire sessions, counting scopes, operations, attempts,
// then funding accounts. Each set is sorted; provider I/O never holds a lock.
type operationLock uint8

const (
	projectWrite operationLock = iota
	capacityChange
	attemptDispatch
)

func lockScopes(ctx context.Context, tx pgx.Tx, keys ...string) error {
	sort.Strings(keys)
	for i, key := range keys {
		if i > 0 && keys[i-1] == key {
			continue
		}
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,8318071246))", key); err != nil {
			return err
		}
	}
	return nil
}

func lockCapacity(ctx context.Context, tx pgx.Tx, v Session) error {
	keys := []string{"capacity:actor:" + v.Actor, "capacity:pool:workspace"}
	if v.Anonymous {
		keys[1] = "capacity:pool:guest"
	}
	if v.WorkspaceID != nil {
		keys = append(keys, "capacity:workspace:"+v.WorkspaceID.String())
	}
	if v.Anonymous {
		var trial string
		if err := tx.QueryRow(ctx, "SELECT trial_key FROM vibe_sessions WHERE id=$1", v.ID).Scan(&trial); err != nil {
			return err
		}
		keys = append(keys, "capacity:trial:"+trial)
	}
	return lockScopes(ctx, tx, keys...)
}

func lockOperation(ctx context.Context, tx pgx.Tx, id uuid.UUID, purpose operationLock) (Operation, error) {
	var session uuid.UUID
	if err := tx.QueryRow(ctx, "SELECT session_id FROM vibe_operations WHERE id=$1", id).Scan(&session); err != nil {
		return Operation{}, err
	}
	var actor string
	var workspace *uuid.UUID
	var trial *string
	// Tombstones remain lockable for accounting after project deletion.
	if err := tx.QueryRow(ctx, "SELECT actor,workspace_id,trial_key FROM vibe_sessions WHERE id=$1 FOR UPDATE", session).Scan(&actor, &workspace, &trial); err != nil {
		return Operation{}, err
	}
	if purpose == capacityChange {
		if err := lockCapacity(ctx, tx, Session{ID: session, Actor: actor, WorkspaceID: workspace, Anonymous: workspace == nil}); err != nil {
			return Operation{}, err
		}
	} else if purpose == attemptDispatch {
		var input []byte
		if err := tx.QueryRow(ctx, "SELECT input FROM vibe_operations WHERE id=$1", id).Scan(&input); err != nil {
			return Operation{}, err
		}
		var p Plan
		if err := json.Unmarshal(input, &p); err != nil {
			return Operation{}, err
		}
		keys := []string{}
		if !p.LocalTesting {
			if workspace == nil && trial != nil {
				keys = append(keys, "capacity:trial:"+*trial)
			}
			if p.Free {
				var day string
				if err := tx.QueryRow(ctx, "SELECT (now() AT TIME ZONE 'UTC')::date::text").Scan(&day); err != nil {
					return Operation{}, err
				}
				keys = append(keys, "capacity:free-day:"+day)
			}
		}
		if err := lockScopes(ctx, tx, keys...); err != nil {
			return Operation{}, err
		}
	}
	return scanOperation(tx.QueryRow(ctx, operationSelect+" WHERE id=$1 FOR UPDATE", id))
}

func lockFunding(ctx context.Context, tx pgx.Tx, ids []string) error {
	sort.Strings(ids)
	for i, id := range ids {
		if i > 0 && ids[i-1] == id {
			continue
		}
		var locked string
		if err := tx.QueryRow(ctx, "SELECT id FROM vibe_accounts WHERE id=$1 FOR UPDATE", id).Scan(&locked); err != nil {
			return err
		}
	}
	return nil
}

func operationFunding(ctx context.Context, tx pgx.Tx, ops []uuid.UUID) error {
	rows, err := tx.Query(ctx, "SELECT DISTINCT account_id FROM vibe_reservations WHERE operation_id=ANY($1)", ops)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	return lockFunding(ctx, tx, ids)
}
