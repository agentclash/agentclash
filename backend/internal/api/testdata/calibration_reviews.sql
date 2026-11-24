-- org
INSERT INTO organizations (id, name, slug) VALUES ($1, 'Calibration fixture', $2);

-- user
INSERT INTO users (id, workos_user_id, email) VALUES ($1, $2, $3);

-- workspace
INSERT INTO workspaces (id, organization_id, name, slug) VALUES ($1, $2, 'Calibration fixture', $3);

-- run
INSERT INTO runs (id, organization_id, workspace_id, name, source_type)
VALUES ($1, $2, $3, 'Calibration fixture', 'agent_harness');

-- agent
INSERT INTO run_agents (id, organization_id, workspace_id, run_id, lane_index, label, source_type)
VALUES ($1, $2, $3, $4, 0, 'Calibration fixture', 'agent_harness');

-- turns
INSERT INTO run_events (run_id, run_agent_id, sequence_number, event_type, actor_type, occurred_at, payload)
VALUES ($1, $2, 1, 'turn.completed', 'multi_turn_engine', now(), '{"turn_index":0}'),
       ($1, $2, 2, 'turn.completed', 'multi_turn_engine', now(), '{"$ref":"run-events/calibration/turn-1.json","bytes":33,"type":"turn.completed"}'),
       ($1, $2, 3, 'turn.awaiting_human', 'multi_turn_engine', now(), '{"turn_index":2}');

-- legacy reviews
INSERT INTO calibration_reviews (workspace_id, run_agent_id, turn_index, reviewer_user_id, score)
VALUES ($1, $2, 1, $5, 4.5), ($1, $4, 1, $5, 3), ($3, $4, 1, $5, 2), ($3, $2, 1, $5, 1);

-- count reviews
SELECT count(*) FROM calibration_reviews WHERE workspace_id = $1;

-- delete org
DELETE FROM organizations WHERE id = $1;

-- delete user
DELETE FROM users WHERE id = $1;
