# fix/security-hardening-224 — Test Contract

Addresses: https://github.com/agentclash/agentclash/issues/224

## Functional Behavior

### 1. CORS origin is configurable by environment
- `CORS_ALLOWED_ORIGINS` env var controls which origins are accepted.
- When `AUTH_MODE=dev` and env var is unset, default to `*` (preserve dev UX).
- When `AUTH_MODE=workos` (production), `CORS_ALLOWED_ORIGINS` must be set or the origin header is not sent (deny-by-default).
- Multiple origins can be comma-separated; the middleware matches the request `Origin` header against the list and reflects it back (or omits the header).
- Preflight (`OPTIONS`) requests still return 204 with proper headers when origin matches.

### 2. Silent config parse failures are logged
- `allowedToolKinds()` logs a structured warning on JSON unmarshal failure.
- `applyChallengeSandboxPolicy()` logs a structured warning on JSON unmarshal failure.
- `applyRuntimeSandboxPolicy()` logs a structured warning on JSON unmarshal failure.
- `applySandboxConfig()` logs a structured warning on JSON unmarshal failure.
- All four use `slog.Warn` with the function name and the error message.

### 3. File tool paths are validated against workspace root
- `read_file`, `write_file`, `list_files` all reject paths that escape the sandbox working directory.
- A path like `../../etc/passwd` returns a tool error, never reaches the sandbox API.
- Paths are cleaned with `path.Clean()` and checked with a prefix match against the workspace root (`/workspace`).
- Valid paths (absolute under `/workspace` or relative from it) still work.

### 4. Callback token includes workspace ID (scoped signing)
- **DEFERRED** — Per-workspace secrets require a DB migration and wiring changes across API + worker + Temporal workflow signals. This is too invasive for a single security-hardening PR. This PR documents the finding and defers it.

## Unit Tests

### CORS
- `TestCORSMiddleware_DevDefaultsToWildcard` — with `authMode=dev` and no env override, `Access-Control-Allow-Origin: *` is returned.
- `TestCORSMiddleware_ProductionRequiresExplicitOrigins` — with `authMode=workos` and no env var, no `Access-Control-Allow-Origin` header is returned.
- `TestCORSMiddleware_MatchesRequestOrigin` — with `CORS_ALLOWED_ORIGINS=https://app.example.com`, a request with `Origin: https://app.example.com` gets that origin reflected. A request with `Origin: https://evil.com` gets no origin header.
- `TestCORSMiddleware_MultipleOrigins` — comma-separated origins all match correctly.
- `TestCORSMiddleware_PreflightReturns204` — OPTIONS requests return 204 when origin matches.

### Silent Error Logging
- `TestAllowedToolKinds_MalformedManifest` — malformed JSON returns nil (existing behavior preserved) but a warning is emitted.
- `TestApplyChallengeSandboxPolicy_MalformedManifest` — malformed JSON causes no policy changes but a warning is emitted.
- `TestApplyRuntimeSandboxPolicy_MalformedJSON` — same pattern.
- `TestApplySandboxConfig_MalformedJSON` — same pattern.

### Path Validation
- `TestReadFile_RejectsPathTraversal` — `../../etc/passwd` returns tool error with `IsError: true`.
- `TestWriteFile_RejectsPathTraversal` — same for write.
- `TestListFiles_RejectsPathTraversal` — same for list.
- `TestReadFile_AllowsValidAbsolutePath` — `/workspace/main.go` passes validation.
- `TestReadFile_AllowsRelativePath` — `main.go` is resolved to `/workspace/main.go` and passes.

## Integration / Functional Tests

N/A — these are in-process unit-testable changes. No cross-service integration needed.

## Smoke Tests

N/A — the changes are internal middleware and engine behavior, not new endpoints.

## E2E Tests

N/A — no new user-facing flows.

## Manual / cURL Tests

### CORS
```bash
# Dev mode (default) — should see Access-Control-Allow-Origin: *
curl -s -D - -o /dev/null -H "Origin: https://evil.com" http://localhost:8080/healthz

# Production mode with explicit origins — should see origin reflected
CORS_ALLOWED_ORIGINS=https://app.agentclash.com AUTH_MODE=workos ... 
curl -s -D - -o /dev/null -H "Origin: https://app.agentclash.com" http://localhost:8080/healthz

# Production mode, wrong origin — should NOT see Access-Control-Allow-Origin
curl -s -D - -o /dev/null -H "Origin: https://evil.com" http://localhost:8080/healthz
```
