# Local Evaltest CLI

Run pre-deploy agent evals locally without an AgentClash account.

## Quick start

```bash
python -m pip install agentclash-evals
agentclash evaltest init
agentclash evaltest run --format both --out agentclash-results
```

The SDK package lives in the dedicated repo:
https://github.com/agentclash/agentclash-evals

For local SDK development, point the CLI at a source checkout:

```bash
export AGENTCLASH_EVAL_SDK_SRC=/path/to/agentclash-evals/python/agentclash_eval/src
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
