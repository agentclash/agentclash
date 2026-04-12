# AgentClash

B2B AI agent evaluation platform. Go backend (Chi router, PostgreSQL, Temporal) + Next.js frontend.

## OpenAPI Specification

The backend API spec lives at `docs/api-server/openapi.yaml` (OpenAPI 3.1).

When you add, modify, or remove a backend API route in `backend/internal/api/`:

1. Update the corresponding path in `docs/api-server/openapi.yaml`
2. If the request or response shape changed, update the matching schema under `components/schemas/`
3. If a new status enum value was added in `backend/internal/domain/`, update the corresponding enum in the spec
4. Validate: `npx @redocly/cli lint docs/api-server/openapi.yaml`

### Route-to-spec mapping

- Routes are registered in `backend/internal/api/routes.go`
- Go struct `createRunResponse` maps to OpenAPI schema `CreateRunResponse`
- Go struct `getRunResponse` maps to OpenAPI schema `RunDetail`
- Naming convention: Go camelCase struct name -> OpenAPI PascalCase schema name

### What is NOT in the spec

- Next.js waitlist routes (`web/src/app/api/waitlist/`) — separate frontend service
- Internal Temporal workflow contracts — not HTTP APIs
