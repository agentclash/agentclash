# AgentClash Prompt Eval CI

Prompt eval CI uses shared workspace playground resources during V1. Until the backend exposes an idempotent playground upsert API, serialize jobs that target the same workspace and config.

```yaml
concurrency:
  group: agentclash-prompt-eval-${{ github.repository }}-${{ github.workflow }}-${{ vars.AGENTCLASH_WORKSPACE }}-.agentclash-prompt-eval-yaml
  cancel-in-progress: false
```

Do not include `github.ref` in the group. Two pull requests can target the same AgentClash workspace and prompt-eval config, so including the ref would allow both jobs to race the same playground.
