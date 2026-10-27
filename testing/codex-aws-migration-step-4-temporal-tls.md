# Temporal client TLS/mTLS — test contract

This change prepares the API and worker for a self-hosted Temporal service. It
does not provision or deploy that service or change the source environment.

## Functional behavior

- Both API and worker load the same validated Temporal connection configuration
  at startup and pass it to the shared client constructor. Metrics options remain
  attached. A client is never dialed with invalid address or namespace settings.
- An existing `TEMPORAL_API_KEY` continues to enable TLS with system trust and
  bearer authentication when no explicit TLS switch is supplied.
- `TEMPORAL_TLS_ENABLED=true` enables TLS independently of the API key.
  `TEMPORAL_TLS_CA_FILE` supplies a PEM CA bundle instead of system roots, and
  `TEMPORAL_TLS_SERVER_NAME` optionally selects the verified certificate name.
  Certificate verification is always enabled; TLS 1.2 is the minimum version.
- `TEMPORAL_TLS_CERT_FILE` and `TEMPORAL_TLS_KEY_FILE` supply a matching PEM client
  certificate/key pair for mTLS. API-key and mTLS authentication are separate
  modes; configuring both fails clearly.
- Only `development`, `dev`, `local` and `test` environments allow unauthenticated
  or plaintext connections. Other environments require TLS and either API-key
  authentication or the complete client certificate/key pair.
- Plaintext local development remains available when TLS and credentials are
  unset. Explicitly disabling TLS with credentials or any TLS material is an
  error. TLS file/name settings without enabling TLS are not silently ignored.
- Malformed booleans, missing/unreadable files, malformed CA material, incomplete
  or mismatched client pairs, and expired/not-yet-valid client certificates fail
  before network access. Configuration errors identify variable names, not
  private paths, key values or certificate contents.
- Wrong CA, wrong server name, untrusted client certificates and a required but
  absent client certificate fail real TLS handshakes. There is no insecure
  certificate-verification option or plaintext fallback after a TLS failure.
- Real secrets, certificates, account identifiers and operational evidence stay
  outside Git. Test certificates/keys are generated at runtime in temporary files.

## Unit tests

- `TestLoadConnectionConfigFromEnv` covers development defaults, existing Cloud
  behavior, independent TLS, production authentication requirements, conflicts,
  boolean validation and all development environment aliases.
- `TestLoadConnectionConfigFromEnvCertificateFiles` covers CA bundles, key pairs,
  certificate validity, missing/unreadable files and sanitized errors.
- `TestClientOptions` covers address/namespace validation, configured transport,
  metrics preservation and isolation of TLS settings between client instances.
- API and worker configuration tests prove both startup paths enforce the shared
  production connection policy and accept the existing Cloud configuration.

## Integration / functional tests

Use an in-process gRPC Temporal service stub and runtime-generated certificates,
with the project's pinned Temporal Go SDK. Tests exercise the real SDK dial path
and TLS handshakes rather than replacing the SDK dialer.

- Local plaintext succeeds in development.
- TLS plus an API key sends the expected bearer metadata to a trusted server.
- Self-hosted mTLS succeeds without an API key and sends the expected client
  certificate. Both explicit server names and address-derived names are covered.
- Wrong server name/CA, an untrusted client and missing required client
  authentication fail within bounded test deadlines.

## Smoke tests

- `go build ./...` and `go vet ./...` from `backend/` pass.
- `go test -short -race -count=1 ./...` from `backend/` passes, including the
  connection tests and existing API/worker tests.
- Public examples explain local, Cloud and self-hosted configuration, certificate
  mounts and restart-based rotation without containing real deployment material.
- Review the exact staged diff and run the configured secret scanner before each
  commit. Do not include unrelated working-tree changes.

## E2E tests

N/A for this preparation step: no deployed Temporal server, cloud resources,
production workflow or browser journey changes. A real self-hosted Temporal
deployment/recovery rehearsal remains a later migration step. The local gRPC
tests establish client transport/authentication behavior only.

## Manual tests

From `backend/`, reviewers can run:

```sh
go test -race -count=1 ./internal/temporalutil
go test -short -race -count=1 ./internal/api ./internal/worker
```

No production credentials, live API keys, or operator login is required.
