-- +goose Up
ALTER TABLE vibe_enquiries ALTER COLUMN session_id DROP NOT NULL;
ALTER TABLE vibe_enquiries ADD COLUMN actor text;
ALTER TABLE vibe_enquiries ADD CONSTRAINT vibe_enquiries_owner CHECK (session_id IS NOT NULL OR actor IS NOT NULL);
CREATE UNIQUE INDEX vibe_contact_enquiries_client ON vibe_enquiries(client_id) WHERE session_id IS NULL;
CREATE INDEX vibe_contact_enquiries_quota ON vibe_enquiries(actor, created_at) WHERE session_id IS NULL;

-- +goose Down
DELETE FROM vibe_enquiries WHERE session_id IS NULL;
DROP INDEX vibe_contact_enquiries_client;
DROP INDEX vibe_contact_enquiries_quota;
ALTER TABLE vibe_enquiries DROP CONSTRAINT vibe_enquiries_owner;
ALTER TABLE vibe_enquiries DROP COLUMN actor;
ALTER TABLE vibe_enquiries ALTER COLUMN session_id SET NOT NULL;
