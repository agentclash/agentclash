-- +goose Up
ALTER TABLE workspaces
ADD COLUMN public_packs boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspaces
DROP COLUMN IF EXISTS public_packs;
