-- +goose Up
-- A claimed turn holds a lease until lease_until. If the session that claimed
-- it crashes, the turn stays 'processing'; a later claim takes it again after
-- the lease runs out. Rows already 'processing' have no lease and count as
-- expired.
ALTER TABLE agent_tryout_turns ADD COLUMN lease_until timestamptz;

-- +goose Down
ALTER TABLE agent_tryout_turns DROP COLUMN lease_until;
