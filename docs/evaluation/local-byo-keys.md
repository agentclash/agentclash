# Local BYO provider keys

Part of Local M1 (`agentclash local run`). Credentials for local evals stay on
the user's machine: process environment, optional user config, optional OS
keychain. They are never uploaded to the AgentClash hosted control plane.

## Resolution order

`runtime/provider/local.ChainResolver` resolves a credential reference in this
order:

1. **Process environment** — `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`,
   `GEMINI_API_KEY`, `XAI_API_KEY`, `OPENROUTER_API_KEY`, `MISTRAL_API_KEY`
   (and `env://` / `secret://` candidates).
2. **User config file** — `$XDG_CONFIG_HOME/agentclash/provider_keys.yaml`
   (default `~/.config/agentclash/provider_keys.yaml`).
3. **OS keychain** (optional) — service `agentclash.local.providers`,
   account = provider key (`openai`, `anthropic`, …).

Missing keys fail closed with an actionable error that names the env var,
config path, and keychain service.

`workspace-secret://` references are rejected in local mode
(`ErrHostedSecretRejected`). Hosted BYOK / workspace secrets are unchanged.

## Supported providers

| Provider key | Env var | Default credential reference |
|---|---|---|
| `openai` | `OPENAI_API_KEY` | `env://OPENAI_API_KEY` |
| `anthropic` | `ANTHROPIC_API_KEY` | `env://ANTHROPIC_API_KEY` |
| `gemini` | `GEMINI_API_KEY` | `env://GEMINI_API_KEY` |
| `xai` | `XAI_API_KEY` | `env://XAI_API_KEY` |
| `openrouter` | `OPENROUTER_API_KEY` | `env://OPENROUTER_API_KEY` |
| `mistral` | `MISTRAL_API_KEY` | `env://MISTRAL_API_KEY` |

## Config file shape

```yaml
providers:
  openai:
    api_key: sk-...
  anthropic:
    api_key: sk-ant-...
```

The file is created with mode `0600` when written via
`local.SaveProviderKeys` / `local.SetProviderKey`.

## OS keychain

Service name: `agentclash.local.providers`. Account: provider key.

```go
_ = local.OSKeychain{}.Set("openai", os.Getenv("OPENAI_API_KEY"))
```

Headless environments without a keyring treat missing/unavailable keychain
entries as misses and continue to the fail-closed error.

## Constructing a local router

```go
import "github.com/agentclash/agentclash/runtime/provider/local"

router := local.NewDefaultLocalRouter(nil)
// or with options:
router := local.NewLocalRouter(nil, local.ChainOptions{
    ConfigPath: "/path/to/provider_keys.yaml", // optional
    Keychain:   local.OSKeychain{},            // nil skips keychain
})
```

`local.NewLocalRouter` wraps `provider.NewDefaultRouter` with
`local.NewChainResolver`. No AgentClash API calls are made for secrets.

## Credential references

Local runs typically use `env://PROVIDER_API_KEY` (same as
`scoring.JudgeDefaultCredentialReference`) or a bare provider key /
`provider://openai`. `secret://name` still resolves via
`AGENTCLASH_SECRET_*` env candidates for compatibility with existing env-based
fixtures.
