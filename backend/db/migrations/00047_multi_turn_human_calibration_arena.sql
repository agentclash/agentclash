-- +goose Up
CREATE TABLE multi_turn_human_turns (
    run_agent_id uuid NOT NULL REFERENCES run_agents (id) ON DELETE CASCADE,
    turn_index integer NOT NULL CHECK (turn_index >= 0),
    phase_id text NOT NULL,
    prompt_hint text,
    status text NOT NULL CHECK (status IN ('awaiting', 'submitted', 'expired')),
    message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    submitted_at timestamptz,
    PRIMARY KEY (run_agent_id, turn_index)
);

CREATE INDEX multi_turn_human_turns_awaiting_idx
ON multi_turn_human_turns (run_agent_id)
WHERE status = 'awaiting';

CREATE TABLE calibration_reviews (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    run_agent_id uuid NOT NULL REFERENCES run_agents (id) ON DELETE CASCADE,
    turn_index integer NOT NULL CHECK (turn_index >= 0),
    reviewer_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    score numeric(4,2) NOT NULL CHECK (score >= 1 AND score <= 5),
    rubric_key text,
    notes text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX calibration_reviews_workspace_idx ON calibration_reviews (workspace_id, created_at DESC);

CREATE TABLE workspace_arena_tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    case_key text NOT NULL,
    left_run_agent_id uuid NOT NULL REFERENCES run_agents (id) ON DELETE CASCADE,
    right_run_agent_id uuid NOT NULL REFERENCES run_agents (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed')),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX workspace_arena_tasks_pending_idx
ON workspace_arena_tasks (workspace_id, status, created_at);

CREATE TABLE workspace_arena_votes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id uuid NOT NULL REFERENCES workspace_arena_tasks (id) ON DELETE CASCADE,
    voter_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    winner_run_agent_id uuid NOT NULL REFERENCES run_agents (id) ON DELETE CASCADE,
    rubric_scores jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (task_id, voter_user_id)
);

CREATE TABLE multi_turn_run_agent_flags (
    run_agent_id uuid PRIMARY KEY REFERENCES run_agents (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    run_id uuid NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    case_key text NOT NULL DEFAULT '',
    calibration_candidate boolean NOT NULL DEFAULT false,
    arena_eligible boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX multi_turn_run_agent_flags_run_idx ON multi_turn_run_agent_flags (run_id);
CREATE INDEX multi_turn_run_agent_flags_arena_idx
ON multi_turn_run_agent_flags (workspace_id, arena_eligible)
WHERE arena_eligible;

CREATE UNIQUE INDEX workspace_arena_tasks_pair_idx
ON workspace_arena_tasks (
    LEAST(left_run_agent_id, right_run_agent_id),
    GREATEST(left_run_agent_id, right_run_agent_id)
);

-- +goose Down
DROP TABLE IF EXISTS workspace_arena_votes;
DROP TABLE IF EXISTS workspace_arena_tasks;
DROP TABLE IF EXISTS calibration_reviews;
DROP TABLE IF EXISTS multi_turn_run_agent_flags;
DROP TABLE IF EXISTS multi_turn_human_turns;
