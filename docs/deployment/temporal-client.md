# Temporal client connections

The API and worker use the same connection loader. Set `APP_ENV=production` in
deployed services. Environments other than `development`, `dev`, `local` and `test`
require verified TLS and either an API key or a client certificate/key pair.
`APP_ENV` defaults to development for the existing local setup.

This guide configures application clients. Server deployment, server-side trust,
certificate issuance, namespace creation and authorization must be configured
separately before a production migration.

## Settings

| Variable | Behavior |
| --- | --- |
| `TEMPORAL_HOST_PORT` | DNS name or IP address and a port, such as `temporal:7233` or `[::1]:7233`; no URL scheme. |
| `TEMPORAL_NAMESPACE` | Nonempty namespace without whitespace; it must already exist on the server. |
| `TEMPORAL_API_KEY` | Optional bearer credential, supplied by the secret manager. When set, enables TLS by default. |
| `TEMPORAL_TLS_ENABLED` | Explicit boolean. Leave it **unset**, rather than empty, for the API-key default. Set `true` for self-hosted TLS/mTLS. |
| `TEMPORAL_TLS_CA_FILE` | Optional readable PEM CA bundle. When set, its CA certificates replace system trust for this connection. Without it, the client uses system roots. |
| `TEMPORAL_TLS_SERVER_NAME` | Optional DNS name or IP address to verify against the server certificate, without a port. Otherwise the TLS stack verifies the host from `TEMPORAL_HOST_PORT`. |
| `TEMPORAL_TLS_CERT_FILE` | PEM client certificate, with any intermediate chain certificates. Must be supplied with the key file. |
| `TEMPORAL_TLS_KEY_FILE` | Matching unencrypted PEM private key, mounted from protected runtime secret storage. Never store the real key in Git or an image. |

TLS requires version 1.2 or newer and always verifies the server certificate.
There is no option to skip verification. A CA bundle must contain only valid PEM
CA certificates; an empty, malformed or partially malformed bundle is rejected.
Client certificates must be currently valid, permit client authentication, and
be end-entity certificates rather than CA certificates.
File/configuration errors name the relevant variable without printing private
paths, credentials or certificate material.

An API key and a client certificate are separate authentication modes; setting
both is an error. Disabling TLS while credentials or TLS file/name settings remain
configured is also an error. A failed TLS connection never retries over plaintext.

## Local development

The existing development configuration works without TLS or credentials:

```dotenv
APP_ENV=development
TEMPORAL_HOST_PORT=localhost:7233
TEMPORAL_NAMESPACE=default
```

Leave `TEMPORAL_API_KEY` and all `TEMPORAL_TLS_*` variables unset. Explicit
`TEMPORAL_TLS_ENABLED=false` is also allowed when no credentials or TLS settings
are provided. Development can use TLS without client authentication for local
server tests; production requires authentication.

## Temporal Cloud with an API key

Keep the existing Cloud host and namespace in private runtime configuration and
supply `TEMPORAL_API_KEY` through the deployment secret manager. No new TLS
variables are required. The application continues to use TLS, system certificate
trust and the SDK's bearer authentication. Explicit `TEMPORAL_TLS_ENABLED=true`
is also valid.

The local tests use a generated CA to simulate this connection mode. They do not
contact Temporal Cloud or verify a live key or namespace.

## Self-hosted mTLS

Example configuration, using a private service name and placeholder mount paths:

```dotenv
APP_ENV=production
TEMPORAL_HOST_PORT=temporal:7233
TEMPORAL_NAMESPACE=agentclash-prod
TEMPORAL_TLS_ENABLED=true
TEMPORAL_TLS_CA_FILE=/run/secrets/temporal/ca.pem
TEMPORAL_TLS_SERVER_NAME=temporal
TEMPORAL_TLS_CERT_FILE=/run/secrets/temporal/client.crt
TEMPORAL_TLS_KEY_FILE=/run/secrets/temporal/client.key
```

Remove or empty `TEMPORAL_API_KEY` for this mode. The server certificate must
cover the configured server name in its subject alternative names. The server
must trust the issuing CA of the client certificate and require client
authentication. Use separate issued identities for the API, worker and operator
where the deployment's certificate policy calls for them; the mount paths can be
the same inside each isolated service.

Deliver files through protected, read-only runtime mounts, readable only by the
intended service. Real certificates, keys, endpoint inventories and populated
deployment parameters stay outside the public repository. Never copy them into
build contexts, command arguments, CI output or release artifacts. mTLS identifies
trusted clients; server-side authorization still controls their allowed actions.

## Rotation and verification

The loader reads and validates the certificate files once at process startup.
Changing mounted files does not reload a running client. Stage compatible server
and client trust, replace the protected files, and restart clients through the
deployment's drain/restart procedure. A CA bundle can temporarily contain both
issuers during a planned rotation. Expiry monitoring and server-side rotation are
part of the deployment's operational setup.

Run the local transport tests from `backend/`:

```sh
go test -race -count=1 ./internal/temporalutil
```

They use the pinned Temporal SDK and an in-process gRPC service to verify
plaintext development, bearer authentication over TLS, mTLS, server-name checks,
CA trust, rejected client certificates and no plaintext fallback. All certificates
and private keys are generated at test runtime and removed with temporary files.
A real Temporal deployment, namespace and workflow/recovery rehearsal remain
necessary before production cutover.

References: [Temporal Go client connection options](https://pkg.go.dev/go.temporal.io/sdk@v1.41.0/client#ConnectionOptions),
[Temporal SDK API-key credentials](https://pkg.go.dev/go.temporal.io/sdk@v1.41.0/client#NewAPIKeyStaticCredentials).
