# feat/200-fe-api-client-shared-infra — Test Contract

## Functional Behavior

### API Client (`lib/api/client.ts`)
- Typed `apiClient` with methods: `get<T>()`, `post<T>()`, `patch<T>()`, `del<T>()`
- Reads `API_URL` / `NEXT_PUBLIC_API_URL` env var as base URL
- Accepts an access token (string) and attaches `Authorization: Bearer {token}` header
- On 4xx/5xx responses, parses `{"error":{"code":"...","message":"..."}}` and throws a typed `ApiError`
- `ApiError` exposes `.code`, `.message`, `.status` for consumer handling
- Pagination helper: `fetchPaginated<T>(path, params)` returns `{ items: T[], total, limit, offset }`
- All methods set `Content-Type: application/json` for request bodies

### Shared UI Components
- **ErrorBoundary** — class component wrapping children; on render error shows recovery UI with "Try again" button that calls `reset()`
- **LoadingSpinner** — SVG spinner with optional `size` prop ("sm" | "md" | "lg"), renders centered by default
- **Skeleton** — animated pulse placeholder with configurable `width`, `height`, `rounded` props
- **EmptyState** — centered column with icon slot, title, description, and optional CTA button
- **Toast / notification system** — `ToastProvider` context + `useToast()` hook returning `toast.success(msg)` and `toast.error(msg)`; renders fixed-position toast stack; auto-dismisses after 5s
- **ConfirmDialog** — modal overlay with title, description, confirm/cancel buttons; confirm button accepts `variant` ("danger" | "default"); uses `useConfirm()` hook returning `confirm(options) => Promise<boolean>`
- **Badge** — inline pill with `variant` prop mapping to status colors (success/warning/error/info/neutral); text is passed as children
- **DataTable** — accepts `columns` (with `header`, `accessor`, `sortable`) and `data` array; renders `<table>` with sortable column headers, empty state, and pagination controls (prev/next with page info)
- **PageHeader** — renders page title, optional breadcrumb trail, and optional action slot (right-aligned)

### Hooks
- **`useSession()`** — fetches `GET /v1/auth/session` via the API client on mount; returns `{ session, loading, error, refresh }` where `session` is typed `SessionResponse | null`
- **`useWorkspace()`** — reads `workspaceSlug` from URL params; cross-references with session memberships; returns `{ workspaceId, workspaceSlug, role } | null`
- **`useOrganization()`** — reads `orgSlug` from URL params; cross-references with `/v1/users/me` organization list; returns `{ organizationId, orgSlug, orgName, role } | null`

## Unit Tests
N/A — no test runner configured in the frontend yet. Verified via TypeScript compilation and manual testing.

## Integration / Functional Tests
N/A — no integration test harness. Verified via TypeScript compilation (`npm run build` must succeed).

## Smoke Tests
- `npm run build` in `web/` completes without errors
- `npm run lint` passes
- All new TypeScript files compile without type errors

## E2E Tests
N/A — no E2E framework configured.

## Manual / cURL Tests

### Verify API client works (requires running backend at localhost:8080)
```bash
# Start the dev server
cd web && npm run dev

# In browser: navigate to /dashboard (must be authenticated)
# Open DevTools Network tab
# Verify: GET request to {API_URL}/v1/auth/session with Authorization header
# Verify: Response is parsed and session data is displayed
```

### Verify toast notifications
```
# In browser console on any authenticated page:
# Trigger an API error (e.g., navigate to invalid workspace)
# Verify: Error toast appears in bottom-right corner
# Verify: Toast auto-dismisses after ~5 seconds
```

### Verify shared components render
```
# Import and render each component in a test page or Storybook
# Verify: LoadingSpinner renders animated SVG
# Verify: Skeleton renders pulsing placeholder
# Verify: Badge renders colored pill for each variant
# Verify: EmptyState renders centered content with CTA
# Verify: DataTable renders sortable headers and pagination
# Verify: PageHeader renders title and breadcrumbs
# Verify: ConfirmDialog opens as modal overlay
# Verify: ErrorBoundary catches thrown errors and shows recovery UI
```
