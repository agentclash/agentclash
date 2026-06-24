# Local Evaltest CLI

Run pre-deploy agent evals locally without an AgentClash account.

## Quick start

```bash
agentclash evaltest init
agentclash evaltest run --format both --out agentclash-results
```

## GitHub Actions

```yaml
- name: Run AgentClash pre-deploy evals
  run: agentclash evaltest run --format junit --out agentclash-results

- name: Upload eval report
  uses: actions/upload-artifact@v4
  with:
    name: agentclash-eval-report
    path: agentclash-results
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All evals passed |
| 1 | Eval assertions failed |
| 2 | Config/test authoring error |
| 3 | Provider/runtime error |
| 4 | Internal SDK/runner error |

See `schemas/evaltest/README.md` for the full contract.
