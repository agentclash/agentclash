# Challenge Pack Library

The library is a curated set of ready-to-run challenge packs ("templates"). Add
one to your workspace, run it against your agents, and customize it in the
builder — no authoring from scratch required.

## Use a pack

**Web** — open **Challenge Packs → Pack Library** (`/workspaces/{id}/challenge-packs/library`),
pick a pack, and click **Use template**. It's cloned into your workspace as a
runnable pack and you land on its detail page. Adding the same template twice
just reopens your existing copy.

**CLI**

```bash
agentclash challenge-pack catalog list
agentclash challenge-pack catalog use text-to-sql
agentclash run create --challenge-pack-version-id <id> --follow
```

`catalog use` is idempotent — re-running it returns the existing pack.

## Tweak a pack

A cloned pack is a normal workspace pack. On its detail page click **Edit in
builder** to open it in the visual builder (the published manifest is decompiled
into an editable composition), change what you need, and re-publish. Advanced
fields the builder doesn't have editors for yet (security policy, custom tools,
metrics, behavioral signals, runtime limits, voice modality) are preserved
losslessly through the round-trip — edit them via the YAML escape hatch.

Each pack ships with small inline fixtures and clearly-marked **swap points** in
its YAML header (the corpus, schema, SOP, tool catalog, or hidden tests) so you
can point it at your own data.

## What's in the library

Every pack also reports **cost / latency / reliability** dimensions alongside its
task-specific scoring.

### Enterprise use cases
| Slug | Mode | Measures |
|------|------|----------|
| `text-to-sql` | native | NL→SQL execution accuracy (sandboxed result-set match) + safe-SQL guard |
| `document-extraction` | prompt_eval | messy doc → strict JSON, field-level correctness |
| `customer-support-policy` | multi_turn | SOP adherence, PII safety, de-escalation, recovery (tau-style) |
| `swe-bug-fix` | native | fix a real bug, graded by hidden tests (SWE-bench-style) |
| `rag-faithfulness` | prompt_eval | grounded QA with citations; faithfulness + relevancy (RAGAS-style) |
| `summarization-faithfulness` | prompt_eval | no-fabrication summary; ROUGE floor + faithfulness/conciseness |
| `it-helpdesk-triage` | native | runbook tool-use with a destructive-action guard |

### Agent capabilities
| Slug | Mode | Measures |
|------|------|----------|
| `tool-calling-accuracy` | native | tool selection/args/order + abstention (BFCL-style) |
| `json-output-conformance` | prompt_eval | strict-schema structured output + grounded values |
| `knowledge-reasoning-regression` | prompt_eval | hard MCQ accuracy — a cheap model-migration anchor |

### Safety & security
| Slug | Mode | Measures |
|------|------|----------|
| `prompt-injection-defense` | native | indirect-injection resistance: canary leak + exfil detection (AgentDojo-style) |
| `jailbreak-refusal` | native | multi-vector harmful-request refusal (HarmBench/AgentHarm-style) |

## Add a pack to the library (contributors)

The catalog is embedded in the backend. To add a pack:

1. Drop a complete, runnable bundle at
   `backend/internal/challengepack/catalog/<slug>.yaml`. Keep it **asset-free**
   (inline fixtures only) — artifact-backed assets can't be instantiated into a
   workspace.
2. Add an editorial entry (category, tags, est. cost/runtime) to
   `catalogMetadata` in `backend/internal/challengepack/catalog.go`.
3. `go test ./internal/challengepack/` enforces the invariants: every pack
   parses, validates, composes a manifest, is asset-free, has matching metadata,
   and round-trips through the builder (decompile → recompose).

The pack is then served by `GET /v1/challenge-pack-catalog`, appears in the web
gallery, and is instantiable via the API / CLI — no database changes.

See also: [multi-turn](multi-turn.md), [user-simulator](user-simulator.md),
[case-templating](case-templating.md).
