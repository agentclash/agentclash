# Multi-turn challenge packs

Hybrid `multi_turn` execution runs a conversation loop outside the native multi-step tool loop. Each case declares a `user_simulator` manifest with scripted, LLM, and human phases.

## Execution flow

1. Worker routes `execution_mode: multi_turn` to `MultiTurnExecutor`.
2. Phases emit `turn.*` events (see [multi-turn-events.md](./multi-turn-events.md)).
3. Human phases block on operator input via API/CLI until timeout.
4. Scoring builds a transcript from events and evaluates `recovery_behavior` plus optional `human_preference` from arena votes.

## Operator APIs

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/v1/workspaces/{ws}/runs/{runId}/run-agents/{runAgentId}/turns` | Submit human user message |
| `GET` | `/v1/workspaces/{ws}/runs/{runId}/run-agents/{runAgentId}/turns/status` | Poll awaiting-human state |
| `POST` | `/v1/workspaces/{ws}/calibration-reviews` | Record H2 calibration score (1–5) |
| `GET` | `/v1/workspaces/{ws}/calibration-reviews` | List recent calibration reviews |
| `GET` | `/v1/workspaces/{ws}/arena/tasks` | List pending pairwise arena tasks |
| `POST` | `/v1/workspaces/{ws}/arena/votes` | Submit arena preference vote |

## CLI

```bash
export AGENTCLASH_WORKSPACE="<workspace-id>"

# While a run agent is executing and awaiting human input:
agentclash run turn status <runAgentId> --run <runId>
agentclash run turn submit <runAgentId> --run <runId> --message "Fine, email me when it posts."
```

### Calibration reviews and arena votes

Record a human score for a run agent turn. Use a turn index of 0 or more and a score from 1 to 5. The server keeps two decimal places of the score.

```bash
agentclash calibration create --run-agent <run-agent-id> --turn 0 --score 4.5 --rubric-key recovery --notes "The agent resumed the task."
agentclash calibration list
agentclash calibration list --json --query '.items[] | select(.run_agent_id == "<run-agent-id>")'
```

The server returns up to 100 reviews, newest first. The query selects reviews from those results. If `calibration create` exits 75, run `agentclash calibration list` before a retry, because a retry can record a second review.

List the pending arena tasks and select the left or right run agent. You can give one `--rubric-score key=value` flag for each rubric key.

```bash
agentclash arena tasks
agentclash arena vote <task-id> --winner left --rubric-score recovery=4.5
```

The server returns up to 20 pending tasks, oldest first. If the task is not in the list, give the run agent UUID of the winner with `--winner-run-agent`.

```bash
agentclash arena vote <task-id> --winner-run-agent <run-agent-id>
```

Use `--json` or `--output yaml` to read the server response. The table shows a warning if the list reaches the server limit.

## Web UI

The run-agent replay page groups steps by `turn_index`, shows mismatch badges, and surfaces an awaiting-human banner with a submit form while the agent is executing.

## Reference pack

Publish and run [`examples/challenge-packs/multi-turn-refund-recovery.yaml`](../../examples/challenge-packs/multi-turn-refund-recovery.yaml) for an end-to-end smoke test.

## Related

- [user-simulator.md](./user-simulator.md) — schema and triggers
- [multi-turn-events.md](./multi-turn-events.md) — event types and transcript
- [case-templating.md](./case-templating.md) — `{{placeholder}}` in scripted messages
- Epic [#839](https://github.com/agentclash/agentclash/issues/839)
