# Page Inventory

Every page the frontend needs, mapped to the backend APIs that power it.

## Auth Pages

| Page | Route | Backend API | Status |
|---|---|---|---|
| Login | `/auth/login` | WorkOS AuthKit redirect | Built |
| Auth callback | `/auth/callback` | WorkOS code exchange | Built |
| CLI device verification | `/auth/device` | `POST /v1/cli-auth/device/approve`, `POST /v1/cli-auth/device/deny` | Built |
| Logout | `/auth/logout` | WorkOS session destroy | Not built |

## Dashboard

| Page | Route | Backend API | Status |
|---|---|---|---|
| Home / Run list | `/` | `GET /v1/runs` (not yet built) | Not built |
| Create run | `/runs/new` | `POST /v1/runs` | Not built |

## Run Detail

| Page | Route | Backend API | Status |
|---|---|---|---|
| Run overview | `/runs/[id]` | `GET /v1/runs/{id}` | Not built |
| Run agents | `/runs/[id]/agents` | `GET /v1/runs/{id}/agents` | Not built |

## Replay

| Page | Route | Backend API | Status |
|---|---|---|---|
| Replay viewer | `/replays/[id]` | `GET /v1/replays/{runAgentId}?limit=&cursor=` | Not built |

Backend API returns: `state` (ready/pending/errored), `replay` object with summary, paginated `steps` array, `pagination` with cursor/limit/total.

The Go backend already serves a minimal HTML viewer at `GET /v1/replays/{runAgentId}/viewer` — the React page should replace this with a proper component.

## Scorecards

| Page | Route | Backend API | Status |
|---|---|---|---|
| Scorecard view | `/scorecards/[id]` | `GET /v1/scorecards/{runAgentId}` | Not built |

Backend API returns: `state` (ready/pending/errored), `run_agent_status`, scorecard with `correctness_score`, `reliability_score`, `latency_score`, `cost_score`, `overall_score`, and full `scorecard` JSONB.

## Comparison

| Page | Route | Backend API | Status |
|---|---|---|---|
| Compare view | `/compare?baseline={runId}&candidate={runId}` | `GET /v1/compare?baseline_run_id=&candidate_run_id=` | Not built |

Backend API returns: comparison status (comparable/not_comparable), dimension deltas, failure divergence, replay summary divergence, evidence quality warnings.

The Go backend already serves a minimal HTML viewer at `GET /v1/compare/viewer?baseline_run_id=&candidate_run_id=` — the React page should replace this.

## Settings / Management

| Page | Route | Backend API | Status |
|---|---|---|---|
| Agent deployments | `/deployments` | Not yet built | Not built |
| Challenge packs | `/packs` | Not yet built | Not built |
| Workspace settings | `/settings` | Not yet built | Not built |

---

## API Availability Summary

These backend APIs exist today and are ready for frontend consumption:

| API | Method | Exists |
|---|---|---|
| `POST /v1/runs` | Create a run | Yes |
| `GET /v1/runs/{id}` | Run detail | Yes |
| `GET /v1/runs/{id}/agents` | List run agents | Yes |
| `GET /v1/replays/{runAgentId}` | Replay with pagination | Yes |
| `GET /v1/scorecards/{runAgentId}` | Scorecard with states | Yes |
| `GET /v1/compare` | Cross-run comparison | Yes |
| `GET /healthz` | Health check | Yes |
| `POST /v1/integrations/hosted-runs/{runID}/events` | Hosted callback | Yes |

These are needed but don't exist yet:

| API | Method | Needed for |
|---|---|---|
| `GET /v1/runs` | List runs | Dashboard home |
| `GET /v1/deployments` | List deployments | Create run form |
| `GET /v1/challenge-packs` | List packs | Create run form |
