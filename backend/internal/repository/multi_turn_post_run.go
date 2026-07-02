package repository

import (
	"context"
	"fmt"

	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/google/uuid"
)

type UpsertMultiTurnRunAgentFlagsParams struct {
	RunAgentID           uuid.UUID
	WorkspaceID          uuid.UUID
	RunID                uuid.UUID
	CaseKey              string
	CalibrationCandidate bool
	ArenaEligible        bool
}

type MultiTurnRunAgentFlags struct {
	RunAgentID           uuid.UUID
	CaseKey              string
	CalibrationCandidate bool
	ArenaEligible        bool
}

// UpsertMultiTurnRunAgentFlagsFromExecution records calibration/arena flags for a multi_turn run agent.
func (r *Repository) UpsertMultiTurnRunAgentFlagsFromExecution(ctx context.Context, executionContext RunAgentExecutionContext) error {
	if r.db == nil {
		return nil
	}
	if executionContext.ChallengeInputSet == nil || len(executionContext.ChallengeInputSet.Cases) == 0 {
		return nil
	}
	firstCase := executionContext.ChallengeInputSet.Cases[0]
	spec := challengepack.CloneUserSimulatorSpec(firstCase.UserSimulator)
	if spec == nil {
		return nil
	}

	calibrationCandidate := false
	if spec.Calibration != nil && spec.Calibration.Enabled {
		calibrationCandidate = challengepack.ShouldSampleCalibration(executionContext.RunAgent.ID, spec.Calibration.SampleRate)
	}

	return r.UpsertMultiTurnRunAgentFlags(ctx, UpsertMultiTurnRunAgentFlagsParams{
		RunAgentID:           executionContext.RunAgent.ID,
		WorkspaceID:          executionContext.Run.WorkspaceID,
		RunID:                executionContext.Run.ID,
		CaseKey:              firstCase.CaseKey,
		CalibrationCandidate: calibrationCandidate,
		ArenaEligible:        challengepack.ArenaEligibleFromSpec(spec),
	})
}

func (r *Repository) UpsertMultiTurnRunAgentFlags(ctx context.Context, params UpsertMultiTurnRunAgentFlagsParams) error {
	if r.db == nil {
		return fmt.Errorf("database is not configured")
	}
	_, err := r.db.Exec(ctx, `
INSERT INTO multi_turn_run_agent_flags (
  run_agent_id, workspace_id, run_id, case_key, calibration_candidate, arena_eligible
) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (run_agent_id) DO UPDATE SET
  case_key = EXCLUDED.case_key,
  calibration_candidate = EXCLUDED.calibration_candidate,
  arena_eligible = EXCLUDED.arena_eligible
`, params.RunAgentID, params.WorkspaceID, params.RunID, params.CaseKey, params.CalibrationCandidate, params.ArenaEligible)
	return err
}

// FinalizeMultiTurnPostRunForRun creates pending arena tasks for eligible agent pairs on a completed run.
func (r *Repository) FinalizeMultiTurnPostRunForRun(ctx context.Context, runID uuid.UUID) (int, error) {
	if r.db == nil {
		return 0, nil
	}

	run, err := r.GetRunByID(ctx, runID)
	if err != nil {
		return 0, err
	}

	rows, err := r.db.Query(ctx, `
SELECT f.run_agent_id, f.case_key, ra.lane_index
FROM multi_turn_run_agent_flags f
JOIN run_agents ra ON ra.id = f.run_agent_id
WHERE f.run_id = $1 AND f.arena_eligible = true
ORDER BY f.case_key ASC, ra.lane_index ASC
`, runID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	eligible := []challengepack.ArenaEligibleAgent{}
	for rows.Next() {
		var agentID uuid.UUID
		var caseKey string
		var laneIndex int32
		if err := rows.Scan(&agentID, &caseKey, &laneIndex); err != nil {
			return 0, err
		}
		eligible = append(eligible, challengepack.ArenaEligibleAgent{
			RunAgentID: agentID,
			CaseKey:    caseKey,
			LaneIndex:  laneIndex,
		})
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	created := 0
	for _, pair := range challengepack.PairArenaAgents(eligible) {
		tag, err := r.db.Exec(ctx, `
INSERT INTO workspace_arena_tasks (workspace_id, case_key, left_run_agent_id, right_run_agent_id, status)
VALUES ($1, $2, $3, $4, 'pending')
ON CONFLICT DO NOTHING
`, run.WorkspaceID, arenaCaseKeyForPair(eligible, pair[0]), pair[0], pair[1])
		if err != nil {
			return created, err
		}
		created += int(tag.RowsAffected())
	}
	return created, nil
}

func arenaCaseKeyForPair(agents []challengepack.ArenaEligibleAgent, runAgentID uuid.UUID) string {
	for _, agent := range agents {
		if agent.RunAgentID == runAgentID {
			return agent.CaseKey
		}
	}
	return ""
}

func (r *Repository) CreateArenaTask(ctx context.Context, workspaceID uuid.UUID, caseKey string, leftRunAgentID, rightRunAgentID uuid.UUID) error {
	if r.db == nil {
		return fmt.Errorf("database is not configured")
	}
	_, err := r.db.Exec(ctx, `
INSERT INTO workspace_arena_tasks (workspace_id, case_key, left_run_agent_id, right_run_agent_id, status)
VALUES ($1, $2, $3, $4, 'pending')
ON CONFLICT DO NOTHING
`, workspaceID, caseKey, leftRunAgentID, rightRunAgentID)
	return err
}
