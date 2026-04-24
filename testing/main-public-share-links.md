# Main Public Share Links — Test Contract

## Functional Behavior
- Authenticated workspace users can create public share links for challenge pack versions, run scorecards, and run-agent scorecards.
- Sharing must verify workspace authorization through the source object before creating a link.
- Public reads must require an active share token and must not expose workspace secrets, provider credentials, private member data, or unrelated workspace objects.
- Share tokens must be unguessable, URL-safe, and scoped to one resource.
- Users can disable an existing share link without deleting the underlying resource.
- Public web routes on agentclash.dev render the shared object without requiring login.

## Unit Tests
- Repository tests cover creating, reading, and revoking public share links.
- API tests cover authz for share creation and unauthenticated public reads.
- Web/API helper tests cover public share fetch error handling.

## Integration / Functional Tests
- Challenge pack version share returns pack metadata, version metadata, manifest, and input set summaries.
- Run scorecard share returns run metadata and scorecard JSON only after the run belongs to an authorized workspace.
- Run-agent scorecard share returns run-agent metadata, scorecard JSON, and replay availability metadata without exposing private artifacts.

## Smoke Tests
- `go test ./internal/api ./internal/repository -run 'PublicShare|Share' -count=1`
- `pnpm test -- public-share`
- `pnpm lint`

## E2E Tests
- N/A for this increment; public pages are covered by server component/API helper smoke tests and manual browser checks.

## Manual / cURL Tests
```bash
curl -X POST "$API/v1/share-links" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"resource_type":"challenge_pack_version","resource_id":"<version-id>"}'
# Expected: 201 with {"url":"https://agentclash.dev/share/<token>", ...}

curl "$API/v1/public/shares/<token>"
# Expected: 200 with public shared resource payload.

curl -X DELETE "$API/v1/share-links/<share-id>" -H "Authorization: Bearer $TOKEN"
# Expected: 204. Subsequent public GET returns 404.
```
