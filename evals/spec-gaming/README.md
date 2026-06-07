# Spec-gaming cross-model study corpus

Research artifacts for the cross-model gaming signature pilot ([PR #858](https://github.com/agentclash/agentclash/pull/858)).

## Hypothesis

Trajectory-level “gaming strategy” representations cluster by **model family** (Anthropic, OpenAI, Gemini, xAI) more cleanly than by **capability** (composite score / correctness).

## Study design

- **6 challenge packs** (Thaman spec-gaming taxonomy): leakage, tampering, sequence manipulation, proxy gaming, special casing, denial of evaluation
- **8 models** per race (neutral lane labels in v2: Primary / Participant 2–8; mapped via deployment id)
- **v1**: single rep per pack (48 trajectories)
- **v2**: 3 reps per pack (144 trajectories); leakage pack bumped to **version 2** (removed in-file comment leak vector)

## Corpus index

| File | Description |
|------|-------------|
| `ARTICLE.md` | Research write-up (methods, results, limitations) |
| `run-starts.json` | v1 manifest: model lineup, pack version ids, run ids |
| `v2-run-starts.json` | v2 eval sessions: 18 run ids (6 packs × 3 reps) |
| `results-summary.json` | v1 scorecards (correctness, integrity, judge snippets) |
| `results-summary-v2.json` | v2 scorecards (144 rows) |
| `full-trail-matrix.json` | v1 compact trail matrix (reads/writes/flags/submit) |
| `full-trail-matrix-v2.json` | v2 full trails (tool traces + submits, 144 agents) |
| `trail-analysis.json` | v1 deep dive: events, tool traces, integrity judge reasoning (27 focus agents) |
| `features.csv` | Extracted behavioral + score features (192 rows: v1 + v2) |
| `trajectory-text.json` | TF-IDF input blobs per trajectory |
| `cluster-stats.json` | KMeans clustering ARI/NMI vs family, capability, pack, strategy |
| `figures/` | PCA scatter plots + ARI bar chart |
| `export_v2_trails.py` | CLI exporter for v2 runs → matrix + results |
| `extract_features.py` | Merge v1/v2 → `features.csv` + `trajectory-text.json` |
| `cluster_analysis.py` | PCA + KMeans + alignment metrics |
| `run_analysis.sh` | End-to-end: export v2 → extract → cluster |
| `queue-v2-sessions.py` | Idempotent v2 eval session queueing |
| `queue-remaining-runs.py` | v1 run queue helper |
| `requirements-analysis.txt` | Python deps for clustering pipeline |
| `deployments/native-coding-runtime-profile.json` | Runtime profile snapshot |

## Reproduce analysis

```bash
export AGENTCLASH_API_URL="https://api.agentclash.dev"
export AGENTCLASH_WORKSPACE="511e2d3e-9076-4db3-b9f2-5ef54ab591d5"
./evals/spec-gaming/run_analysis.sh
```

Requires authenticated CLI (`agentclash auth login`) for re-export; committed JSON/CSV artifacts work offline.

## Key result (combined v1+v2, n=192)

| Label | Adjusted Rand Index |
|-------|---------------------|
| Capability tertile | **0.81** |
| Strategy heuristic | 0.42 |
| Pack | 0.31 |
| Family | 0.01 |

Family did **not** beat capability in this pilot. Qualitative leakage v1 traces remain the strongest family-differentiated signal.
