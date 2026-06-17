-- +goose Up
-- +goose StatementBegin

-- Backfill the $3.00 signup eval credit for orgs that predate the wallet (idempotent on the stable
-- grant key — re-runs and the live SeedOrgEvalCreditTx never double-grant). Wallet + ledger stay
-- consistent: only orgs missing the signup grant are credited, and the ledger delta matches.
INSERT INTO org_eval_credit_wallets (organization_id)
SELECT id FROM organizations
ON CONFLICT (organization_id) DO NOTHING;

WITH missing AS (
    SELECT o.id AS org_id
    FROM organizations o
    WHERE NOT EXISTS (
        SELECT 1 FROM org_eval_credit_ledger l
        WHERE l.organization_id = o.id
          AND l.entry_type = 'grant'
          AND l.idempotency_key = 'signup-eval-credit:v1'
    )
),
credited AS (
    UPDATE org_eval_credit_wallets w
    SET available_micros = available_micros + 3000000, updated_at = now()
    FROM missing m
    WHERE w.organization_id = m.org_id
    RETURNING w.organization_id
)
INSERT INTO org_eval_credit_ledger (
    id, organization_id, entry_type, amount_micros,
    available_delta_micros, reserved_delta_micros, spent_delta_micros,
    idempotency_key, reason
)
SELECT gen_random_uuid(), organization_id, 'grant', 3000000, 3000000, 0, 0,
       'signup-eval-credit:v1', 'backfill signup eval credit'
FROM credited;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Reverse the backfill while keeping wallet+ledger reconciled: decrement available by the granted
-- amount AND delete the matching grant row ONLY for orgs where the decrement succeeded. An org that
-- has since reserved/spent the credit (available < 3000000) is left fully intact — both its wallet
-- and its ledger row — so the ledger always reconciles to the wallet.
WITH backfilled AS (
    SELECT organization_id FROM org_eval_credit_ledger
    WHERE entry_type = 'grant' AND idempotency_key = 'signup-eval-credit:v1'
      AND reason = 'backfill signup eval credit'
),
decremented AS (
    UPDATE org_eval_credit_wallets w
    SET available_micros = available_micros - 3000000, updated_at = now()
    FROM backfilled b
    WHERE w.organization_id = b.organization_id AND w.available_micros >= 3000000
    RETURNING w.organization_id
)
DELETE FROM org_eval_credit_ledger l
USING decremented d
WHERE l.organization_id = d.organization_id
  AND l.entry_type = 'grant' AND l.idempotency_key = 'signup-eval-credit:v1'
  AND l.reason = 'backfill signup eval credit';
-- +goose StatementEnd
