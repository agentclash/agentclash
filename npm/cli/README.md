# agentclash

Command-line interface for the [AgentClash](https://www.agentclash.dev) race
engine — evaluate, compare, and deploy AI agents.

## Install

```bash
npm i -g agentclash
# or
npx agentclash --help
```

The install pulls exactly one prebuilt binary for your OS/architecture from
the matching optional dependency (`@agentclash/cli-<platform>-<arch>`). No
postinstall scripts; no downloads at install time.

Supported platforms:

- `darwin-arm64`, `darwin-x64`
- `linux-arm64`, `linux-x64`
- `win32-arm64`, `win32-x64`

## Get started

```bash
agentclash auth login
agentclash workspace use <workspace-id>
agentclash run create --help
```

## Other install channels

Source, Homebrew, Winget, and direct downloads are documented at
<https://github.com/agentclash/agentclash#cli>.

## License

[FSL-1.1-MIT](https://fsl.software) — see `LICENSE`.

Short version: use and modify it for anything except running a competing
commercial eval-engine service; each version auto-converts to MIT two years
after release.
