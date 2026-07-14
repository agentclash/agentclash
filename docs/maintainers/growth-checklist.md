# Growth checklist (PR F5)

A living checklist for driving discovery and adoption. Tick items as they ship.
Items marked **(admin)** need repo owner/maintain access — see
`docs/maintainers/governance-setup.md` §g.

## Discoverability (high leverage, low effort)
- [ ] **(admin)** Set repo **topics**: `ai-agents`, `llm`, `agent-evaluation`, `evals`,
      `llmops`, `ci`, `golang`, `nextjs`, `temporal`, `open-source`
      (`gh repo edit agentclash/agentclash --add-topic ai-agents …`).
- [ ] **(admin)** Tighten the repo **description** and set **homepage** =
      `https://www.agentclash.dev`.
- [ ] **(admin)** Upload a branded **social-preview image** (Settings → Social preview)
      so shared links render a card instead of a bare URL.

## README assets
- [x] Star-history chart embedded.
- [x] "Open in GitHub Codespaces" badge.
- [x] "Try in 60 seconds" block.
- [ ] **Demo cast:** record an [asciinema](https://asciinema.org) cast of
      `agentclash eval start --follow`, commit under `docs/assets/`, and embed it
      (replace the `TODO(demo)` marker in `README.md`).
- [ ] Cut a 60–90s screen capture for socials.

## Outreach
- [ ] Submit to **awesome-* lists**: awesome-llmops, awesome-ai-agents.
- [ ] Launch posts: Hacker News (Show HN), Product Hunt, r/LocalLLaMA.
- [ ] Short launch blog post (link from README + docs).

## Trust signals
- [ ] Add an **OpenSSF Scorecard** workflow + badge.
- [ ] Consider an OpenSSF Best Practices badge.

## Sustaining the contributor loop
- [ ] Keep **8–12** `good first issue`s open — refresh from
      `docs/maintainers/good-first-issues.md` when the pool runs low.
- [ ] Ensure the welcome bot, merge-share comment, and all-contributors bot are live
      (PR C + the all-contributors App install, see governance-setup.md).
