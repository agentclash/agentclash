# TypeScript Eval SDK Alpha Spike

**Recommendation: GO** for a full TypeScript alpha after Python SDK #1107–#1110 land.

## Mapping

| Python | TypeScript |
|--------|------------|
| `assert_agent` | `assertAgent` |
| `evaluate` | `evaluate` |
| `Contains` | `Contains` |
| `AgentEvalResult` | `AgentEvalResult` |
| `EvalReport` | `EvalReport` (same JSON schema) |

## Package location

`sdk/typescript/evals` as `@agentclash/evals` (private monorepo package until publish).

## Prototype status

- Plain async function evals: implemented
- Deterministic metrics: `Contains` implemented; `OutputSchema`, `ToolCalled`, `LatencyLimit` specced for alpha
- Vitest example: `tests/evals.test.ts`
- No network/auth/telemetry by default

## Next steps for full alpha

1. Port deterministic metrics from Python
2. Add Vitest/Jest helper entrypoints
3. Add Vercel AI SDK adapter
4. Wire CI publish rehearsal (no npm publish in spike)
