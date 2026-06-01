-- +goose Up
CREATE TABLE dataset_generation_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id uuid NOT NULL REFERENCES datasets (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    strategy text NOT NULL,
    status text NOT NULL DEFAULT 'queued',
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    summary jsonb NOT NULL DEFAULT '{}'::jsonb,
    target_count integer NOT NULL DEFAULT 0,
    generated_count integer NOT NULL DEFAULT 0,
    accepted_count integer NOT NULL DEFAULT 0,
    rejected_count integer NOT NULL DEFAULT 0,
    total_input_tokens bigint NOT NULL DEFAULT 0,
    total_output_tokens bigint NOT NULL DEFAULT 0,
    total_cost_usd numeric(12, 6) NOT NULL DEFAULT 0,
    version_id uuid REFERENCES dataset_versions (id),
    temporal_workflow_id text,
    temporal_run_id text,
    error_message text,
    created_by uuid NOT NULL REFERENCES users (id),
    queued_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz,
    failed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT dataset_generation_jobs_status_check CHECK (
        status IN ('queued', 'running', 'completed', 'failed')
    )
);

CREATE INDEX dataset_generation_jobs_dataset_id_idx ON dataset_generation_jobs (dataset_id, created_at DESC);
CREATE INDEX dataset_generation_jobs_workspace_id_idx ON dataset_generation_jobs (workspace_id, created_at DESC);

CREATE TABLE dataset_generation_rejections (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id uuid NOT NULL REFERENCES dataset_generation_jobs (id) ON DELETE CASCADE,
    reason_code text NOT NULL,
    reason_detail text,
    candidate_input jsonb,
    candidate_expected jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX dataset_generation_rejections_job_id_idx ON dataset_generation_rejections (job_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS dataset_generation_rejections;
DROP TABLE IF EXISTS dataset_generation_jobs;
