# codex/roadmap-693-race-series-create - Test Contract

## Files touched
- `backend/internal/api/eval_sessions.go` - decode and return per-child run matrix metadata.
- `backend/internal/api/eval_session_service.go` - expand matrix entries into queued child runs.
- `backend/internal/api/run_service_test.go` - service tests for matrix expansion and metadata.
- `backend/internal/api/runs_test.go` - handler/decode tests for the API contract.
- `cli/cmd/run.go` - CLI flag for multi-lineup series creation.
- `cli/cmd/run_create_helpers.go` - build matrix eval-session request bodies.
- `cli/cmd/eval_session_helpers.go` - render series child metadata.
- `cli/cmd/eval_session_test.go` and `cli/cmd/run_create_interactive_test.go` - CLI request/response tests.
- `docs/api-server/openapi.yaml` - document the additive request/response fields.

## External APIs used
- Cobra flag parsing - `StringSlice` flags and `GetStringSlice`, verified-as-of: 2026-05-10, source URL: https://pkg.go.dev/github.com/spf13/pflag#FlagSet.StringSlice
- Go `encoding/json` - `Decoder.DisallowUnknownFields` and `RawMessage`, verified-as-of: 2026-05-10, source URL: https://pkg.go.dev/encoding/json

## Rollback strategy
Revert this PR. It is additive to the eval-session request/response schema and CLI flags, with no migration. Existing single-run and single-lineup seeded run creation paths remain intact.

## Functional Behavior
- API accepts optional `eval_session.run_matrix` entries; each entry supplies a key, optional seed, optional deployment lineup name, and participants for that child run.
- When `run_matrix` is present, `eval_session.repetitions` must equal the matrix length.
- Service creates one queued child run per matrix entry, preserving child order and setting each child execution plan seed and participant list from the entry.
- Existing uniform `participants` plus `seed_fanout` behavior remains unchanged.
- Response continues to include `run_ids` and additionally surfaces series child metadata with every child `run_id`, `matrix_key`, `deployment_lineup`, and `seed` when available.
- CLI supports multiple pack-declared lineups crossed with `--seeds N`, resolving each lineup to deployment IDs and posting a matrix eval-session request.
- CLI rejects multi-lineup series without `--seeds`, with `--deployments`, or with `--follow`.

## Unit Tests
- `TestDecodeEvalSessionConfigAcceptsRunMatrix` - accepts valid matrix entries and rejects length mismatches/invalid participants.
- `TestRunCreationManagerCreateEvalSessionExpandsRunMatrix` - verifies generated child run count, per-child participants, names, execution plans, and returned metadata.
- `TestBuildSeriesEvalSessionBodyCrossesLineupsAndSeeds` - verifies CLI request body matrix shape.
- `TestRunCreateDeploymentLineupsRoutesToEvalSession` - verifies CLI posts to `/v1/eval-sessions` and surfaces response metadata.

## Integration / Functional Tests
- `go test ./internal/api -run 'TestCreateEvalSession|TestRunCreationManagerCreateEvalSession'`
- `go test ./cmd -run 'TestRunCreate|TestEvalSession'`

## Smoke Tests
- `cd backend && go test ./internal/api`
- `cd cli && go test ./cmd`
- `cd cli && go build ./...`

## E2E Tests
- After merge and deploy, run hosted CLI smoke against `https://api.agentclash.dev` with a request path that exercises the new multi-lineup flag. If the hosted pack does not declare enough lineups, verify the deployed CLI/API returns the expected validation error without regressing existing seeded creation.

## Manual / cURL Tests
```bash
curl -sS -X POST "$AGENTCLASH_API_URL/v1/eval-sessions" \
  -H "Authorization: Bearer $AGENTCLASH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "workspace_id":"<workspace-id>",
    "challenge_pack_version_id":"<pack-version-id>",
    "eval_session":{
      "repetitions":2,
      "aggregation":{"method":"mean","report_variance":true,"confidence_interval":0.95},
      "routing_task_snapshot":{"routing":{"mode":"series"},"task":{"pack_version":"v1"}},
      "schema_version":1,
      "run_matrix":[
        {"key":"default:seed-1","deployment_lineup":"default","seed":1,"participants":[{"agent_deployment_id":"<deployment-id>","label":"Primary"}]},
        {"key":"default:seed-2","deployment_lineup":"default","seed":2,"participants":[{"agent_deployment_id":"<deployment-id>","label":"Primary"}]}
      ]
    }
  }'
# Expected: 201, body includes eval_session, run_ids length 2, and series_runs with matching run IDs.
```
