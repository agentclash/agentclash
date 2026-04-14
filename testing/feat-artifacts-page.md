# feat-artifacts-page — Test Contract

## Functional Behavior

Add a workspace-level Artifacts page accessible from the sidebar under Infrastructure.
Users can browse all artifacts uploaded to their workspace, upload new artifacts, and
download existing ones.

### Backend: List Artifacts Endpoint
- `GET /v1/workspaces/{workspaceID}/artifacts` returns `{ items: Artifact[] }`
- Scoped to workspace — only returns artifacts belonging to that workspace
- Requires authentication (workspace member or above)
- Returns fields: id, artifact_type, content_type, size_bytes, visibility, metadata, run_id, run_agent_id, created_at
- Returns empty array (not null) when no artifacts exist

### Frontend: Artifacts Page
- New route: `/workspaces/[workspaceId]/artifacts/page.tsx`
- Server component using `withAuth` + `createApiClient` (same pattern as Knowledge Sources page)
- Table columns: Name (from metadata.original_filename or artifact ID), Type, Content Type, Size, Visibility, Created
- Empty state with icon when no artifacts exist
- "Upload Artifact" button using existing `UploadArtifactDialog` component
- "Download" action per row using existing `DownloadArtifactButton` component
- Size formatted as human-readable (B/KB/MB)

### Sidebar Navigation
- "Artifacts" entry under Infrastructure section in nav-items.ts
- Uses `FileArchive` icon from lucide-react
- Positioned after Knowledge Sources, before Secrets

### OpenAPI Spec
- `GET /v1/workspaces/{workspaceID}/artifacts` path added to openapi.yaml
- Response schema: `ListArtifactsResponse` with `items` array of `ArtifactUploadResponse`

## Unit Tests
- N/A — page is a server component with no testable logic beyond the API call
- Backend: `ListArtifactsByWorkspaceID` repository method returns correct rows

## Integration / Functional Tests
- N/A for this UI-focused change

## Smoke Tests
- Backend compiles: `cd backend && go build ./...`
- Backend vet passes: `cd backend && go vet ./...`
- Frontend compiles: `cd web && npx tsc --noEmit`
- Frontend lint passes: `cd web && pnpm lint`
- OpenAPI spec validates: `npx @redocly/cli lint docs/api-server/openapi.yaml`

## E2E Tests
- N/A — not applicable for this change

## Manual / cURL Tests
```bash
# List artifacts (should return empty items or existing artifacts)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/workspaces/$WORKSPACE_ID/artifacts

# Expected: { "items": [] } or { "items": [ { "id": "...", ... } ] }
```

Navigate to `/workspaces/{id}/artifacts` in browser:
1. Sidebar shows "Artifacts" under Infrastructure
2. Page loads with table or empty state
3. Upload button opens dialog
4. After upload, artifact appears in table
5. Download button fetches signed URL and opens in new tab
