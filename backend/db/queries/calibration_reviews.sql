-- name: ListCalibrationTurnPayloads :many
SELECT payload
FROM run_events
WHERE run_agent_id = @run_agent_id
  AND event_type = 'turn.completed'
  AND (payload -> 'turn_index' = to_jsonb(@turn_index::integer) OR payload ? '$ref');

-- name: InsertCalibrationReview :execrows
INSERT INTO calibration_reviews (workspace_id, run_agent_id, turn_index, reviewer_user_id, score, rubric_key, notes)
SELECT ra.workspace_id, ra.id, @turn_index, @reviewer_user_id, @score::double precision,
       NULLIF(@rubric_key::text, ''), NULLIF(@notes::text, '')
FROM run_agents ra
WHERE ra.id = @run_agent_id AND ra.workspace_id = @workspace_id
FOR KEY SHARE OF ra;

-- name: ListCalibrationReviews :many
SELECT cr.id, cr.run_agent_id, cr.turn_index, cr.reviewer_user_id, cr.score::double precision AS score,
       coalesce(cr.rubric_key, '') AS rubric_key, coalesce(cr.notes, '') AS notes, cr.created_at
FROM calibration_reviews cr
JOIN run_agents ra ON ra.id = cr.run_agent_id AND ra.workspace_id = cr.workspace_id
WHERE cr.workspace_id = @workspace_id
ORDER BY cr.created_at DESC
LIMIT 100;
