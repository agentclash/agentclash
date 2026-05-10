-- name: GetRunAgentReplayByRunAgentID :one
SELECT
    id,
    run_agent_id,
    artifact_id,
    summary,
    latest_sequence_number,
    event_count,
    created_at,
    updated_at
FROM run_agent_replays
WHERE run_agent_id = @run_agent_id
LIMIT 1;

-- name: UpsertRunAgentReplayIndex :one
INSERT INTO run_agent_replays (
    run_agent_id,
    artifact_id,
    summary,
    latest_sequence_number,
    event_count
)
VALUES (
    @run_agent_id,
    sqlc.narg('artifact_id'),
    @summary,
    sqlc.narg('latest_sequence_number'),
    @event_count
)
ON CONFLICT (run_agent_id) DO UPDATE
SET artifact_id = EXCLUDED.artifact_id,
    summary = EXCLUDED.summary,
    latest_sequence_number = EXCLUDED.latest_sequence_number,
    event_count = EXCLUDED.event_count
RETURNING id, run_agent_id, artifact_id, summary, latest_sequence_number, event_count, created_at, updated_at;

-- name: GetRunAgentScorecardByRunAgentID :one
SELECT
    id,
    run_agent_id,
    evaluation_spec_id,
    overall_score,
    correctness_score,
    reliability_score,
    latency_score,
    cost_score,
    behavioral_score,
    scorecard_passed,
    scorecard,
    created_at,
    updated_at
FROM run_agent_scorecards
WHERE run_agent_id = @run_agent_id
LIMIT 1;

-- name: ListRunAgentScorecardsByRunID :many
SELECT
    sc.id,
    sc.run_agent_id,
    sc.evaluation_spec_id,
    sc.overall_score,
    sc.correctness_score,
    sc.reliability_score,
    sc.latency_score,
    sc.cost_score,
    sc.behavioral_score,
    sc.scorecard_passed,
    sc.scorecard,
    sc.created_at,
    sc.updated_at
FROM run_agent_scorecards sc
JOIN run_agents ra ON ra.id = sc.run_agent_id
WHERE ra.run_id = @run_id
ORDER BY ra.lane_index, ra.label, sc.run_agent_id;
