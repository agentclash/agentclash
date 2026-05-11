# codex/docs-md-noindex - Test Contract

## Functional Behavior

- `/docs-md/...` Markdown exports continue to serve existing rendered Markdown content.
- Successful `/docs-md/...` responses include `X-Robots-Tag: noindex, follow`.
- XML sitemap excludes `/docs-md/...` URLs so canonical `/docs/...` pages remain the indexed docs surface.
- `/docs/...`, `/llms.txt`, and `/llms-full.txt` behavior remains unchanged.

## Unit Tests

- N/A - this change is route header and sitemap behavior.

## Integration / Functional Tests

- `pnpm -C web lint`
- `pnpm -C web build`

## Smoke Tests

- Built sitemap output should not include `/docs-md`.
- `/docs-md` route source should set `X-Robots-Tag: noindex, follow` on successful Markdown responses.

## E2E Tests

- N/A - no browser workflow changes.

## Manual / cURL Tests

After deploy:

```bash
curl -I https://www.agentclash.dev/docs-md/getting-started/quickstart
curl -s https://www.agentclash.dev/sitemap.xml | rg docs-md
```

Expected: `X-Robots-Tag: noindex, follow` is present and the sitemap command returns no matches.
