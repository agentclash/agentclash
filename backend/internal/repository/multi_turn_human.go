package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrHumanTurnNotAwaiting = errors.New("run agent is not awaiting a human turn")

type HumanTurnWaitParams struct {
	RunAgentID uuid.UUID
	TurnIndex  int
	PhaseID    string
	PromptHint string
	Timeout    time.Duration
}

type humanTurnRow struct {
	runAgentID uuid.UUID
	turnIndex  int
	phaseID    string
	promptHint string
	message    string
	status     string
}

// MultiTurnHumanTurnStore persists in-run human turn submissions.
type MultiTurnHumanTurnStore struct {
	pool *pgxpool.Pool

	mu    sync.Mutex
	local map[string]humanTurnRow
}

func NewMultiTurnHumanTurnStore(pool *pgxpool.Pool) *MultiTurnHumanTurnStore {
	return &MultiTurnHumanTurnStore{
		pool:  pool,
		local: make(map[string]humanTurnRow),
	}
}

func (s *MultiTurnHumanTurnStore) MarkAwaitingHuman(ctx context.Context, req HumanTurnWaitParams) error {
	key := humanTurnKey(req.RunAgentID, req.TurnIndex)
	if s.pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.local[key] = humanTurnRow{
			runAgentID: req.RunAgentID,
			turnIndex:  req.TurnIndex,
			phaseID:    req.PhaseID,
			promptHint: req.PromptHint,
			status:     "awaiting",
		}
		return nil
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO multi_turn_human_turns (run_agent_id, turn_index, phase_id, prompt_hint, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, 'awaiting', now(), now())
ON CONFLICT (run_agent_id, turn_index) DO UPDATE SET
  phase_id = EXCLUDED.phase_id,
  prompt_hint = EXCLUDED.prompt_hint,
  status = 'awaiting',
  message = NULL,
  submitted_at = NULL,
  updated_at = now()
`, req.RunAgentID, req.TurnIndex, req.PhaseID, nullIfEmpty(req.PromptHint))
	if err != nil {
		return fmt.Errorf("mark awaiting human turn: %w", err)
	}
	return nil
}

func (s *MultiTurnHumanTurnStore) PollHumanMessage(ctx context.Context, runAgentID uuid.UUID, turnIndex int) (string, bool, error) {
	key := humanTurnKey(runAgentID, turnIndex)
	if s.pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		row, ok := s.local[key]
		if !ok || row.status != "submitted" {
			return "", false, nil
		}
		return row.message, true, nil
	}
	var message *string
	var status string
	err := s.pool.QueryRow(ctx, `
SELECT status, message
FROM multi_turn_human_turns
WHERE run_agent_id = $1 AND turn_index = $2
`, runAgentID, turnIndex).Scan(&status, &message)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("poll human turn: %w", err)
	}
	if status != "submitted" || message == nil {
		return "", false, nil
	}
	return *message, true, nil
}

func (s *MultiTurnHumanTurnStore) SubmitHumanMessage(ctx context.Context, runAgentID uuid.UUID, turnIndex int, message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return fmt.Errorf("human turn message is required")
	}
	key := humanTurnKey(runAgentID, turnIndex)
	if s.pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		row, ok := s.local[key]
		if !ok || row.status != "awaiting" {
			return ErrHumanTurnNotAwaiting
		}
		row.status = "submitted"
		row.message = message
		s.local[key] = row
		return nil
	}
	tag, err := s.pool.Exec(ctx, `
UPDATE multi_turn_human_turns
SET status = 'submitted', message = $3, submitted_at = now(), updated_at = now()
WHERE run_agent_id = $1 AND turn_index = $2 AND status = 'awaiting'
`, runAgentID, turnIndex, message)
	if err != nil {
		return fmt.Errorf("submit human turn: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrHumanTurnNotAwaiting
	}
	return nil
}

func (s *MultiTurnHumanTurnStore) AwaitingTurn(ctx context.Context, runAgentID uuid.UUID) (*HumanTurnStatus, error) {
	if s.pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, row := range s.local {
			if row.runAgentID == runAgentID && row.status == "awaiting" {
				return &HumanTurnStatus{
					TurnIndex:  row.turnIndex,
					PhaseID:    row.phaseID,
					PromptHint: row.promptHint,
				}, nil
			}
		}
		return nil, nil
	}
	var turnIndex int
	var phaseID string
	var promptHint *string
	err := s.pool.QueryRow(ctx, `
SELECT turn_index, phase_id, prompt_hint
FROM multi_turn_human_turns
WHERE run_agent_id = $1 AND status = 'awaiting'
ORDER BY turn_index DESC
LIMIT 1
`, runAgentID).Scan(&turnIndex, &phaseID, &promptHint)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load awaiting human turn: %w", err)
	}
	status := &HumanTurnStatus{TurnIndex: turnIndex, PhaseID: phaseID}
	if promptHint != nil {
		status.PromptHint = *promptHint
	}
	return status, nil
}

func (s *MultiTurnHumanTurnStore) MarkHumanTurnExpired(ctx context.Context, runAgentID uuid.UUID, turnIndex int) error {
	key := humanTurnKey(runAgentID, turnIndex)
	if s.pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		row, ok := s.local[key]
		if !ok || row.status != "awaiting" {
			return nil
		}
		row.status = "expired"
		s.local[key] = row
		return nil
	}
	_, err := s.pool.Exec(ctx, `
UPDATE multi_turn_human_turns
SET status = 'expired', updated_at = now()
WHERE run_agent_id = $1 AND turn_index = $2 AND status = 'awaiting'
`, runAgentID, turnIndex)
	if err != nil {
		return fmt.Errorf("expire human turn: %w", err)
	}
	return nil
}

type HumanTurnStatus struct {
	TurnIndex  int
	PhaseID    string
	PromptHint string
}

func humanTurnKey(runAgentID uuid.UUID, turnIndex int) string {
	return runAgentID.String() + ":" + fmt.Sprintf("%d", turnIndex)
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
