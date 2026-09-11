# Self-host AgentClash on Kubernetes

Fleet 12 ships a cloud-agnostic Helm chart so any Kubernetes cluster can run the
control plane (api-server) and execution plane (Temporal workers) without
AWS/GCP-specific services.

## Prerequisites

- Kubernetes 1.27+
- [Helm 3](https://helm.sh/)
- A Temporal server (self-hosted or Temporal Cloud)
- Postgres 16+ (migrations applied via `backend/scripts/migrate.sh`)
- Redis (optional but recommended for pub/sub, sandbox budget, provider throttle)
- [KEDA](https://keda.sh/) 2.14+ if you want queue-depth autoscaling (Temporal scaler)
- Container images from GHCR (`ghcr.io/agentclash/agentclash-api-server`, `…-worker`)

Optional for the full self-host sandbox story: in-cluster Kubernetes sandbox
provider (Fleet 4) with a default sandbox image.

## Quick install

```bash
helm upgrade --install agentclash ./deploy/helm/agentclash \
  --namespace agentclash --create-namespace \
  --set secrets.existingSecret=agentclash-secrets \
  --set external.temporalAddress=temporal-frontend.temporal.svc:7233 \
  --set sandbox.provider=kubernetes \
  --set keda.enabled=true
```

Create the secret first:

```bash
kubectl -n agentclash create secret generic agentclash-secrets \
  --from-literal=DATABASE_URL='postgres://…' \
  --from-literal=REDIS_URL='redis://…' \
  --from-literal=AUTH_MODE=dev   # local only
```

## Values reference (high level)

| Key | Meaning |
|-----|---------|
| `workers.mode` | `split` (execution/scoring/background Deployments) or `combined` |
| `workers.classes.*.minReplicas` / `maxReplicas` | KEDA bounds per queue class |
| `workers.classes.*.targetQueueSize` | Temporal backlog target per replica |
| `keda.enabled` | Install Temporal `ScaledObject`s |
| `sandbox.provider` | `kubernetes` \| `e2b` \| `unconfigured` |
| `sandbox.maxConcurrent` | Fleet 3 semaphore (caps sandboxes even when workers scale out) |
| `apiServer.metrics.enabled` / `workers.metrics.enabled` | Fleet 14 `/metrics` scrape |
| `serviceMonitor.enabled` | Prometheus Operator ServiceMonitor |

Full defaults live in [`deploy/helm/agentclash/values.yaml`](../../deploy/helm/agentclash/values.yaml).

## Sizing guidance

- **Execution workers × sandbox budget:** KEDA may scale execution replicas to N,
  but `SANDBOX_MAX_CONCURRENT` still caps live sandboxes cluster-wide (Redis
  budget) or per-process (local budget). Size the budget for your provider
  quotas (E2B concurrency or node allocatable for k8s sandboxes).
- **Scoring / background:** Keep lower `maxReplicas`; they share Temporal but
  should not starve execution slots.
- **Scale-to-zero:** Set `minReplicas: 0` on execution/scoring/background. Cold
  start latency is Temporal task backlog drain time + image pull.

## Kind rehearsal

```bash
./deploy/kind/up.sh
```

Loads a kind cluster, `helm lint`s, and installs the chart. You still need
Temporal/Postgres/Redis reachable and images loaded into kind for a full eval
set. KEDA scale 0→N is optional (`KEDA_ENABLED=true` after installing KEDA).

## Image pipeline

On `v*` tags, [`.github/workflows/release-backend-images.yml`](../../.github/workflows/release-backend-images.yml)
builds multi-arch api-server and worker images to GHCR with provenance + SBOM.
Pin chart `image.tag` (or digests) to the release tag.

## Upgrades

1. Run the [application migrator](application-migrations.md) as a one-off job
   and check its exit status before rolling api-server/worker. The existing
   `backend/scripts/migrate.sh` hook delegates to that runner. Do not use numeric
   Goose versions: this application preserves a full-filename migration ledger.
2. `helm upgrade` with the new chart/`appVersion`.
3. Watch Temporal worker slot metrics (Fleet 14) and KEDA HPA events.

## Out of scope

Node autoscaling (Karpenter / cluster-autoscaler) remains a cluster-operator
concern. This chart scales **pods**, not nodes.
