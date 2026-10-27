# Local Evaltest smoke checks

`agentclash evaltest run` invokes the installed Python runner with `--mode smoke`.
It checks that local evaluation scaffolding can run and produce reports. Passing
this command does not establish that your application, deployment or model passed
its real test cases. No AgentClash account is required for this smoke path.

For actual application evaluation, use the supported [canonical connected runtime](../vibe-evals-ci-roundtrip.md).
A separately installed SDK can provide its own pytest/application integration;
use that package's versioned documentation. This repository does not vendor or
verify its Python application API.

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
- name: Run AgentClash local smoke checks
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
| 0 | Smoke checks passed |
| 1 | Smoke assertions failed |
| 2 | Config/test authoring error |
| 3 | Provider/runtime error |
| 4 | Internal SDK/runner error |

See `schemas/evaltest/README.md` for the full contract.
