-- +goose Up
-- Canonical agent_build_versions.model_spec and session documents are editable,
-- not trustworthy historical receipts. Leave every existing save unknown; do
-- not backfill this nullable column from either source.
ALTER TABLE vibe_saved_artifacts ADD COLUMN saved_models jsonb;
ALTER TABLE vibe_saved_artifacts ADD CONSTRAINT vibe_saved_artifacts_saved_models_check CHECK (
 saved_models IS NULL OR (
  jsonb_typeof(saved_models) = 'object'
  AND jsonb_typeof(saved_models->'assistant') = 'string' AND saved_models->>'assistant' <> ''
  AND jsonb_typeof(saved_models->'target') = 'string' AND saved_models->>'target' <> ''
  AND jsonb_typeof(saved_models->'evaluator') = 'string' AND saved_models->>'evaluator' <> ''
 ) IS TRUE
);

-- +goose StatementBegin
CREATE FUNCTION vibe_saved_models_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'Vibe saved model receipt is immutable' USING ERRCODE = '23514';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER vibe_saved_models_immutable
 BEFORE UPDATE OF saved_models ON vibe_saved_artifacts
 FOR EACH ROW WHEN (OLD.saved_models IS DISTINCT FROM NEW.saved_models)
 EXECUTE FUNCTION vibe_saved_models_immutable();

-- +goose Down
DROP TRIGGER IF EXISTS vibe_saved_models_immutable ON vibe_saved_artifacts;
DROP FUNCTION IF EXISTS vibe_saved_models_immutable();
ALTER TABLE vibe_saved_artifacts DROP COLUMN IF EXISTS saved_models;
