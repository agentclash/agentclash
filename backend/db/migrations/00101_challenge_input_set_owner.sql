-- +goose Up
-- Keep the owner after workspace deletion. Clearing it would make the set visible
-- to other workspaces. Runs can prevent cascading deletion of input sets.
ALTER TABLE challenge_input_sets ADD COLUMN owner_workspace_id uuid;

-- API instances that run older code write dataset input sets with no owner.
-- This trigger sets the owner when the dataset link is written. It never changes an owner that is set.
-- +goose StatementBegin
CREATE FUNCTION set_dataset_input_set_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE challenge_input_sets cis
    SET owner_workspace_id = d.workspace_id
    FROM datasets d
    WHERE d.id = NEW.dataset_id
      AND cis.id = NEW.challenge_input_set_id
      AND cis.owner_workspace_id IS NULL;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER dataset_version_input_sets_set_owner
AFTER INSERT OR UPDATE OF challenge_input_set_id, dataset_id ON dataset_version_input_sets
FOR EACH ROW EXECUTE FUNCTION set_dataset_input_set_owner();

UPDATE challenge_input_sets cis
SET owner_workspace_id = d.workspace_id
FROM dataset_version_input_sets dvis
JOIN datasets d ON d.id = dvis.dataset_id
WHERE dvis.challenge_input_set_id = cis.id
  AND cis.owner_workspace_id IS NULL;

UPDATE challenge_input_sets cis
SET owner_workspace_id = d.workspace_id
FROM dataset_versions dv
JOIN datasets d ON d.id = dv.dataset_id
WHERE cis.owner_workspace_id IS NULL
  AND cis.generated_at IS NOT NULL
  AND cis.description LIKE 'Materialized from dataset version %'
  AND dv.id = CASE
      WHEN cis.input_key ~ '^dataset-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}-'
      THEN substring(cis.input_key from 9 for 36)::uuid
  END;

-- Archive dataset input sets whose dataset no longer exists, so that no workspace can select them.
-- Existing runs still load these sets by ID, because those reads do not filter on archived_at.
UPDATE challenge_input_sets
SET archived_at = now()
WHERE owner_workspace_id IS NULL
  AND archived_at IS NULL
  AND generated_at IS NOT NULL
  AND description LIKE 'Materialized from dataset version %'
  AND input_key ~ '^dataset-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}-';

-- +goose Down
-- Down does not unarchive the dataset input sets that Up archived.
DROP TRIGGER dataset_version_input_sets_set_owner ON dataset_version_input_sets;
DROP FUNCTION set_dataset_input_set_owner();
ALTER TABLE challenge_input_sets DROP COLUMN owner_workspace_id;
