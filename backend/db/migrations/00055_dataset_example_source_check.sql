-- +goose Up
-- Production was observed rejecting non-manual example sources (import/trace/synthetic)
-- with a check-constraint violation surfaced as HTTP 500. Re-assert the intended enum.
ALTER TABLE dataset_examples DROP CONSTRAINT IF EXISTS dataset_examples_source_check;
ALTER TABLE dataset_examples ADD CONSTRAINT dataset_examples_source_check
    CHECK (source IN ('manual', 'import', 'trace', 'synthetic', 'promotion'));

-- +goose Down
ALTER TABLE dataset_examples DROP CONSTRAINT IF EXISTS dataset_examples_source_check;
ALTER TABLE dataset_examples ADD CONSTRAINT dataset_examples_source_check
    CHECK (source IN ('manual', 'import', 'trace', 'synthetic', 'promotion'));
