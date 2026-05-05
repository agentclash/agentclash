# AgentClash Prompt Eval CI

Prompt eval CI uses shared workspace playground resources during V1. Until the backend exposes an idempotent playground upsert API, serialize jobs that target the same workspace and config.

```yaml
concurrency:
  group: agentclash-prompt-eval-${{ github.repository }}-${{ github.workflow }}-${{ vars.AGENTCLASH_WORKSPACE }}-.agentclash-prompt-eval-yaml
  cancel-in-progress: false
```

Do not include `github.ref` in the group. Two pull requests can target the same AgentClash workspace and prompt-eval config, so including the ref would allow both jobs to race the same playground.

If you use a config path other than `.agentclash/prompt-eval.yaml`, replace the final suffix with a stable slug for that path. Multiple prompt-eval configs can use separate concurrency groups, but every PR targeting the same workspace and config should share one group.
