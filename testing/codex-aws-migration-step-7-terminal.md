# Step 7 — terminal and proxy test contract

## Functional Behavior

- Production startup requires E2B, durable Redis with verified TLS, an explicit
  origin allowlist, a strong proxy secret and a valid HTTPS gateway origin.
  Invalid/missing settings fail without revealing values. Mock sandboxes and
  in-memory limits remain development-only. Readiness checks actual configured
  dependencies and lifecycle; cheap liveness remains separate.
- Trust browser client IP/identity only from the authenticated frontend proxy or
  explicitly configured immediate reverse proxies. Ignore arbitrary forwarded
  headers on direct requests. Preserve anonymous and authenticated BYO tiers.
  The frontend proxy strips incoming identity/secret headers and supplies its
  own server-derived values. Document the friend-managed deployment handoff.
- Close session/reset admission during drain while existing terminal/gateway
  work can finish. SIGTERM closes sockets, aborts outstanding gateway requests,
  awaits metering and sandbox cleanup, closes durable clients and exits within
  a documented deadline. Failed or timed-out cleanup is reported as failure.
- Track session startup/destroy races, late sandbox creation, reset/expiry,
  capacity and PTY ownership. One session cannot leak duplicate terminals or
  regain access after destruction. Anonymous reset must not extend its deadline.
- Anonymous trial admission and durable spending fail closed when Redis cannot
  answer. Test restart persistence, atomic trial admission, expiry and concurrent
  gateway reservations. Keep uncertainties in provider-cost estimation explicit;
  do not claim exact billing from token estimates.
- Provider credentials never enter anonymous sandboxes or browser responses.
  Remove the direct operator-key injection into the opencode anonymous demo;
  keep it BYO-only unless a separately reviewed metered gateway is added.
- Keep one terminal process with drained, non-overlapping deployments. Session
  maps are not moved into a new distributed ownership system in this step.

## Unit Tests

- Configuration: missing production settings, malformed URLs/origins, invalid
  numeric limits and TLS configuration; legitimate development fallbacks.
- Proxy trust: spoofed X-Forwarded-For/identity headers, trusted socket addresses,
  signed-in/anonymous frontend forwarding and invalid proxy credentials.
- Session lifecycle: startup failure, destroy during boot, late allocation,
  capacity release only after confirmed cleanup, repeated destruction, reset
  expiry, drain rejection, PTY reconnect/ownership and cleanup failures.
- Gateway: valid/expired tokens, both auth tiers, provider-secret isolation,
  request/output limits, reservations/metering, Redis failure and shutdown abort.

## Integration / Functional Tests

- Run real local Redis over TLS with a generated test CA: valid CA works;
  untrusted certificates fail; trial/spend limits survive client restart and
  preserve expiry semantics; unavailable Redis rejects admission/readiness.
- Start the service with injected synthetic E2B/provider dependencies and run
  real HTTP/WebSocket traffic through Caddy. Check both tiers, browser Origin,
  trusted client IP handling, idle WebSocket survival and gateway callbacks.
- Exercise session creation, PTY input/reconnect, expiry/reset, drain and process
  shutdown with controlled local dependencies. No real provider/E2B calls.
- Test the actual frontend proxy helper/route contract locally. Live Vercel
  authentication and routing remain the friend's Steps 13–14 handoff.

## Smoke Tests

- Frozen service dependency install; Bun tests and TypeScript validation.
- Caddy configuration validation plus live isolated proxy checks.
- Production misconfiguration exits nonzero, liveness/readiness reflect state,
  and all disposable resources are cleaned up.
- Review exact diffs/artifacts and run the configured secret scanner before
  each local commit. Keep operational evidence, generated private keys and
  test runtime credentials outside the public Git checkout.

## E2E Tests

Local simulated journeys cover browser HTTP/WS and sandbox gateway contracts.
Real Vercel, public DNS/certificates, Cloudflare behavior and E2B reachability
require the later owner-operated rehearsal/cutover. Do not label local fakes as
production E2E, and do not publish or deploy this step.

## Manual / cURL Tests

- The reusable terminal runbook must show local liveness/readiness and drain
  checks, signal ordering and the required process/proxy stop budgets.
- Preserve actual URLs/secrets privately. Document HTTP versus WebSocket versus
  gateway paths, forwarded header trust and one-instance deployment ordering.
- Record results and stop at the Step 8 handoff. No AWS provisioning, source
  shutdown, credential rotation, frontend deployment, push or PR is authorized.
