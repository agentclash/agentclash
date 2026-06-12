# codex/agent-opportunity-report — Test Contract

## Functional Behavior
- A public marketing page lets a prospect enter a company URL and request an AI Agent Opportunity Report for AgentClash.
- The client sends the URL to a public API route without requiring authentication.
- The API accepts only absolute http/https URLs with public hostnames, blocks localhost/private/link-local hosts, normalizes the submitted URL, and limits input length.
- The API fetches the target company homepage with a timeout, extracts bounded readable text and page signals, and does not forward raw secrets or internal headers.
- The API calls OpenAI using a server-side API key and asks for structured JSON covering agent fit, honest verdict, use cases, savings ranges, risks, and suggested AgentClash evaluation pack.
- The API returns a typed JSON report on success and clear JSON errors for invalid URLs, unreachable pages, missing OpenAI configuration, and model failures.
- The UI renders loading, error, and completed report states. It must not promise every company needs an AI agent.

## Unit Tests
- Public analyzer URL validation blocks private and local targets and accepts normal https URLs.
- HTML extraction removes scripts/styles, truncates content, and keeps useful title/meta/body signals.
- OpenAI response parsing accepts valid structured JSON and rejects malformed report payloads.

## Integration / Functional Tests
- The route returns 400 for invalid URLs before fetching.
- The route returns 503 when OpenAI is not configured.
- With mocked page fetch and mocked OpenAI response, the route returns a complete report payload.

## Smoke Tests
- `cd web && pnpm exec vitest run src/lib/agent-opportunity.test.ts src/app/api/agent-opportunity/route.test.ts`
- `cd web && pnpm lint`

## E2E Tests
- N/A — this change adds a public page and API route; browser E2E can be added after the lead magnet copy is finalized.

## Manual / cURL Tests
- `curl -s -X POST http://localhost:3000/api/agent-opportunity -H "content-type: application/json" -d "{\"url\":\"https://example.com\"}"`
- Visit `/agent-opportunity` locally, submit a public company URL, and confirm the report renders or a useful configuration error appears.
