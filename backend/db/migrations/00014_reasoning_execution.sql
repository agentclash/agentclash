-- Migration 00014: Reasoning execution lane
--
-- Adds the reasoning_v1 agent kind and the reasoning_run_executions table
-- for tracking control-plane state of reasoning-lane runs.

-- Allow reasoning_v1 as a valid agent_kind value.
ALTER TABLE agent_build_versions
    DROP CONSTRAINT agent_build_versions_agent_kind_check,
    ADD CONSTRAINT agent_build_versions_agent_kind_check
        CHECK (agent_kind IN (
            'llm_agent',
            'workflow_agent',
            'programmatic_agent',
            'multi_agent_system',
            'hosted_external',
            'reasoning_v1'
        ));

-- Control-plane mutable state for reasoning-lane runs.
-- Mirrors hosted_run_executions with additions for sandbox lifecycle
-- and pending tool-proposal tracking.
CREATE TABLE reasoning_run_executions (
    id                        uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id                    uuid        NOT NULL REFERENCES runs(id),
    run_agent_id              uuid        NOT NULL UNIQUE REFERENCES run_agents(id),
    reasoning_run_id          text,
    endpoint_url              text        NOT NULL,
    status                    text        NOT NULL CHECK (status IN (
                                  'starting', 'accepted', 'running',
                                  'completed', 'failed', 'timed_out', 'cancelled'
                              )),
    sandbox_metadata          jsonb,
    pending_proposal_event_id text,
    pending_proposal_payload  jsonb,
    last_event_type           text,
    last_event_payload        jsonb       NOT NULL DEFAULT '{}'::jsonb,
    result_payload            jsonb       NOT NULL DEFAULT '{}'::jsonb,
    error_message             text,
    deadline_at               timestamptz NOT NULL,
    accepted_at               timestamptz,
    started_at                timestamptz,
    finished_at               timestamptz,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (run_agent_id, run_id) REFERENCES run_agents(id, run_id)
);
