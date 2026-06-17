-- +goose Up
-- +goose StatementBegin

-- Prepaid, reserve-then-settle eval-execution credit (AgentClash-managed). Integer micros only
-- (1 USD = 1_000_000 micros) — never floats/numeric for balances. Distinct from the budget package /
-- workspace_spend_ledger (which is spend-policy reporting, not prepaid reserved credit).
CREATE TABLE org_eval_credit_wallets (
    organization_id  uuid PRIMARY KEY REFERENCES organizations (id) ON DELETE CASCADE,
    currency_code    text NOT NULL DEFAULT 'USD',
    available_micros bigint NOT NULL DEFAULT 0,
    reserved_micros  bigint NOT NULL DEFAULT 0,
    spent_micros     bigint NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    -- Invariant 1 at the DB level: a balance can never go negative.
    CONSTRAINT org_eval_credit_wallets_available_nonneg CHECK (available_micros >= 0),
    CONSTRAINT org_eval_credit_wallets_reserved_nonneg CHECK (reserved_micros >= 0),
    CONSTRAINT org_eval_credit_wallets_spent_nonneg CHECK (spent_micros >= 0)
);

-- Reservation lifecycle (open -> settled | released). One reservation per run (key `run:<run_id>`).
-- The (organization_id, reservation_key) unique index makes Reserve idempotent.
CREATE TABLE org_eval_credit_reservations (
    id              uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES org_eval_credit_wallets (organization_id) ON DELETE CASCADE,
    reservation_key text NOT NULL,
    amount_micros   bigint NOT NULL CHECK (amount_micros > 0),
    status          text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'settled', 'released')),
    run_id          uuid,
    eval_session_id uuid,
    created_at      timestamptz NOT NULL DEFAULT now(),
    resolved_at     timestamptz,
    UNIQUE (organization_id, reservation_key)
);

-- Immutable, append-only audit of every wallet mutation. Delta columns let the ledger reconstruct the
-- wallet totals exactly (Invariant 6: sum of deltas == wallet columns).
CREATE TABLE org_eval_credit_ledger (
    id                     uuid PRIMARY KEY,
    organization_id        uuid NOT NULL REFERENCES org_eval_credit_wallets (organization_id) ON DELETE CASCADE,
    entry_type             text NOT NULL CHECK (entry_type IN ('grant', 'reserve', 'settle', 'release', 'adjust')),
    amount_micros          bigint NOT NULL,
    available_delta_micros bigint NOT NULL,
    reserved_delta_micros  bigint NOT NULL,
    spent_delta_micros     bigint NOT NULL,
    idempotency_key        text,
    reservation_id         uuid REFERENCES org_eval_credit_reservations (id) ON DELETE SET NULL,
    run_id                 uuid,
    eval_session_id        uuid,
    tool_invocation_id     uuid,
    confirmation_id        uuid,
    actor_user_id          uuid,
    reason                 text,
    metadata               jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at             timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_org_eval_credit_ledger_org_created ON org_eval_credit_ledger (organization_id, created_at);

-- Grant idempotency: at most one grant per (organization, grant key).
CREATE UNIQUE INDEX uq_org_eval_credit_ledger_grant_key
    ON org_eval_credit_ledger (organization_id, idempotency_key)
    WHERE entry_type = 'grant';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS org_eval_credit_ledger;
DROP TABLE IF EXISTS org_eval_credit_reservations;
DROP TABLE IF EXISTS org_eval_credit_wallets;
-- +goose StatementEnd
