# Vibe browser stack regression

This suite drives the actual Vibe page through HTTP, anonymous cookies, CORS,
SSE, admission, PostgreSQL, the outbox dispatcher, a **real Temporal server and
worker**, the compiler, and the persisted attempt/result journal. No Playwright
request interception is used. A scripted `provider.Client` replaces model
responses, and miniredis supplies Redis protocol behavior. It makes no provider
network requests and does not measure semantic model accuracy or real billing.
The fixture defaults to the current `suite-review-v3` validator, supplying a
source-grounded fact ledger for these fixed scenarios. V3 derives expected ask
obligations on the server; the fake provider omits that field entirely. Set
`VIBE_BROWSER_REVIEW_VERSION=suite-review-v1` or `suite-review-v2` to check an older
review contract. V2 retains its original model-extracted obligation fields.

The journey checks:

- Byte-preserved multiline preparation and exactly three reviewed, persisted tests.
- Reloading the browser without losing the prepared suite.
- A defective agent scoring 2/3, a focused instruction change, and a 3/3 rerun
  with identical canonical **full blueprint and grading hashes**.
- A direct editor change rejected by policy review while the previous suite and
  typed editor draft survive.
- Failure of both bounded author attempts, persistence through later chat and
  reload, and explicit retry of the failed operation without another user bubble
  or loss of the unrelated composer draft.
- Completion receipt counts, zero-cost settled attempts, and dispatched outbox entries.

From the repository root, provide a local PostgreSQL test database URL. The role
must have `CREATEDB`; the selected database must be `vibe_test` or begin with
`vibe_test_`. The fixture creates, migrates, and removes its own uniquely named
`vibe_test_browser_<uuid>` database. It does not migrate or change the supplied
base database. Every execution also gets a unique Temporal namespace, preventing
the fixed `vibe-evals` queue from consuming another worker's work.

```sh
cd web
export VIBE_TEST_DATABASE_URL='postgres://vibe_test:local_test_only@127.0.0.1:5432/vibe_test?sslmode=disable'
npx playwright test --config playwright.vibe-stack.config.ts
```

Install web dependencies and Playwright Chromium using the repository's normal
CI setup first. The Go version comes from `backend/go.mod`. If Temporal is not
configured, the Temporal Go SDK downloads and launches the real Temporal CLI
development server on a free loopback port. Set `VIBE_BROWSER_TEMPORAL_CLI` to a
preinstalled CLI executable for offline CI, or set
`VIBE_BROWSER_TEMPORAL_ADDRESS=127.0.0.1:7233` to use an existing local server.
Existing servers retain the uniquely named test namespace; its workflow history
has a one-day retention period.

The default ports are frontend **53518** and fixture API **55441**. Override them
using `VIBE_BROWSER_WEB_PORT` and `VIBE_BROWSER_API_PORT`. Both bind to loopback;
Playwright refuses to reuse occupied ports. `start-web.mjs` copies the real app
source/config into a temporary directory, links installed dependencies, and
removes the copy on exit. It does not copy `.env` files or write the developer's
`.next`, `next-env.d.ts`, or `tsconfig.json`. No change to production configuration
or service restart is needed.

On a headless Linux development server, use the existing resource launcher and
browser installation, for example:

```sh
agent-run env \
  VIBE_BROWSER_TEMPORAL_ADDRESS=127.0.0.1:7233 \
  VIBE_TEST_CHROMIUM=/path/to/existing/chrome \
  npx playwright test --config playwright.vibe-stack.config.ts
```

CI should add PostgreSQL (the same `vibe_test` service configuration used by
`.github/workflows/vibe-evals.yml`), `actions/setup-go`, and the test database URL
to a browser job, then run the Playwright command above. Add this config and
`web/e2e/vibe-stack/**` to the workflow's path filter. Upload
`web/test-results/vibe-stack` on failure; it contains the Playwright trace,
screenshots, and successful journey's synthetic persisted evidence. The Go
fixture test skips during ordinary `go test` unless `VIBE_BROWSER_STACK=1`.
