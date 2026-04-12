# feat/115-tenant-crud — Test Contract

## Functional Behavior

### Phase 1: Caller Expansion
- Caller struct includes `OrganizationMemberships map[uuid.UUID]OrganizationMembership`
- OrganizationMembership has `OrganizationID uuid.UUID` and `Role string`
- WorkOS authenticator loads org memberships alongside workspace memberships
- Dev authenticator parses `X-Agentclash-Org-Memberships` header (format: `orgUUID:role,orgUUID:role`)
- CORS allows `X-Agentclash-Org-Memberships` in dev mode
- Session response includes org memberships
- Repository method `GetActiveOrganizationMembershipsByUserID` queries organization_memberships where membership_status = 'active'

### Phase 2: Auto-Create User
- WorkOS authenticator: when `GetUserByWorkOSID` returns `ErrUserNotFound`, creates user from JWT claims (workos_user_id, email)
- Returns Caller with empty memberships (org and workspace) on first login
- Repository method `CreateUser` inserts into users table, returns created user
- Handles unique constraint race condition (concurrent first requests)

### Phase 3: GET /v1/users/me
- Returns nested response: user profile + organizations[] with workspaces[]
- For org_admin users, includes ALL workspaces in their org (implicit access), with role = "org_admin"
- For org_member users, includes only explicitly-membered workspaces
- Deduplicates when user is both org_admin and explicit workspace member (explicit wins)
- Empty organizations array for users with no memberships

### Phase 4: Organization CRUD
- `POST /v1/organizations` — any authenticated user, hard limit of 1 org per user
- Slug: optional on create, auto-generated from name if omitted, validated format, 409 on conflict
- Creates org + org_admin membership in single transaction
- `GET /v1/organizations` — list caller's orgs with pagination (limit/offset)
- `GET /v1/organizations/{organizationID}` — any org member
- `PATCH /v1/organizations/{organizationID}` — org_admin only, accepts name and/or status
- Status transitions: active <-> archived. Archive cascades to all workspaces + all memberships in one tx
- Unarchive restores org only, not workspaces

### Phase 5: Workspace CRUD
- `POST /v1/organizations/{organizationID}/workspaces` — org_admin only
- Slug uniqueness scoped to organization
- Creates workspace + workspace_admin membership in single transaction
- `GET /v1/organizations/{organizationID}/workspaces` — org_admin sees all, org_member sees only their workspaces
- `GET /v1/workspaces/{workspaceID}` — workspace member OR org_admin of parent org
- `PATCH /v1/workspaces/{workspaceID}` — workspace_admin OR org_admin, accepts name and/or status
- Archive cascades to workspace memberships
- Workspace authorizer updated: org_admin of parent org gets implicit access

### Phase 6: Organization Membership CRUD
- `GET /v1/organizations/{organizationID}/memberships` — any org member, returns active+invited (not archived)
- `POST /v1/organizations/{organizationID}/memberships` — org_admin only, email-based invite
- Creates stub user if email not found in users table
- Membership created with status = 'invited'
- 409 if already active member
- Re-invite allowed if previously archived
- `PATCH /v1/organization-memberships/{membershipID}` — org_admin or invited user (for accepting)
- Status transitions: invited->active (accept, by invited user), invited->archived (revoke, admin), active->suspended, active->archived, suspended->active, suspended->archived
- Invite expires after 7 days (checked on accept)
- Last org_admin protection: cannot demote/archive/suspend if last active org_admin
- Cannot change own role

### Phase 7: Workspace Membership CRUD
- `GET /v1/workspaces/{workspaceID}/memberships` — workspace member or org_admin
- `POST /v1/workspaces/{workspaceID}/memberships` — workspace_admin or org_admin
- Pre-check: user must have active org membership for parent org, else 400 "org_membership_required"
- Same invite flow as org memberships
- `PATCH /v1/workspace-memberships/{membershipID}` — workspace_admin, org_admin, or invited user
- Same status transitions, last admin protection, 7-day expiry
- Role values: workspace_admin, workspace_member, workspace_viewer

### Phase 8: POST /v1/onboarding
- Any authenticated user with zero active org_admin memberships
- 409 "already_onboarded" if user already has an org
- Request: organization_name, organization_slug (optional), workspace_name, workspace_slug (optional)
- Atomic transaction: org + workspace + org_admin membership + workspace_admin membership
- Response: 201 with nested org + workspace

## Unit Tests

### Phase 1
- `TestCallerOrgMemberships` — Caller struct holds org memberships correctly
- `TestWorkOSAuthenticator_LoadsOrgMemberships` — authenticator populates OrganizationMemberships
- `TestDevAuthenticator_ParsesOrgMemberships` — dev authenticator parses X-Agentclash-Org-Memberships header
- `TestDevAuthenticator_EmptyOrgMemberships` — returns empty map when header missing

### Phase 2
- `TestWorkOSAuthenticator_FirstLoginCreatesUser` — user not found -> creates user, returns empty memberships
- `TestWorkOSAuthenticator_RaceConditionOnCreate` — handles unique constraint violation gracefully

### Phase 3
- `TestGetUserMe_FullNested` — returns correct nested structure
- `TestGetUserMe_OrgAdminImplicitWorkspaces` — org_admin sees all workspaces
- `TestGetUserMe_EmptyOrgs` — new user sees empty organizations array

### Phase 4
- `TestCreateOrganization_Success` — creates org with auto-slug
- `TestCreateOrganization_CustomSlug` — creates org with provided slug
- `TestCreateOrganization_LimitReached` — rejects when user already has an org
- `TestCreateOrganization_SlugConflict` — returns 409 slug_taken
- `TestListOrganizations_OnlyMemberOrgs` — returns only caller's orgs
- `TestGetOrganization_NotMember` — returns 403
- `TestUpdateOrganization_ArchiveCascade` — archives org, workspaces, and all memberships
- `TestUpdateOrganization_NotAdmin` — returns 403 for non-admin

### Phase 5
- `TestCreateWorkspace_Success` — creates workspace with membership
- `TestCreateWorkspace_NotOrgAdmin` — returns 403
- `TestListWorkspaces_OrgAdminSeesAll` — org_admin sees all workspaces
- `TestListWorkspaces_MemberSeesOwn` — org_member sees only their workspaces
- `TestGetWorkspace_OrgAdminImplicit` — org_admin can access without explicit membership
- `TestUpdateWorkspace_ArchiveCascade` — archives workspace and its memberships

### Phase 6
- `TestInviteOrgMember_Success` — creates invited membership
- `TestInviteOrgMember_StubUser` — creates stub user when email not found
- `TestInviteOrgMember_AlreadyMember` — returns 409
- `TestAcceptOrgInvite_Success` — invited user accepts, status -> active
- `TestAcceptOrgInvite_Expired` — rejects expired invite (>7 days)
- `TestAcceptOrgInvite_NotInvitedUser` — rejects accept from wrong user
- `TestDemoteLastOrgAdmin` — rejects demotion of last admin

### Phase 7
- `TestInviteWorkspaceMember_Success` — creates invited membership
- `TestInviteWorkspaceMember_NoOrgMembership` — returns 400 org_membership_required
- `TestAcceptWorkspaceInvite_Success` — status -> active
- `TestDemoteLastWorkspaceAdmin` — rejects demotion of last admin

### Phase 8
- `TestOnboarding_Success` — creates org + workspace + memberships atomically
- `TestOnboarding_AlreadyOnboarded` — returns 409
- `TestOnboarding_SlugConflict` — returns 409 slug_taken

## Integration / Functional Tests
- N/A for this PR — we use unit tests with stub repositories following existing codebase patterns

## Smoke Tests
- After all phases, `go build ./...` compiles without errors
- `go test ./backend/internal/api/...` passes all tests
- `go vet ./backend/...` reports no issues

## E2E Tests
- N/A — no running database in CI for this PR. Verified through unit tests with stubs.

## Manual / cURL Tests

```bash
# Dev mode headers for all requests:
# X-Agentclash-User-Id: <uuid>
# X-Agentclash-WorkOS-User-Id: user_test
# X-Agentclash-User-Email: test@example.com
# X-Agentclash-User-Display-Name: Test User
# X-Agentclash-Org-Memberships: <org-uuid>:org_admin

# GET /v1/users/me — should return nested response
curl -s http://localhost:8080/v1/users/me \
  -H "X-Agentclash-User-Id: 00000000-0000-0000-0000-000000000001" \
  -H "X-Agentclash-Org-Memberships: "
# Expected: 200, {"user_id":"...","organizations":[]}

# POST /v1/onboarding — create first org+workspace
curl -s -X POST http://localhost:8080/v1/onboarding \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: 00000000-0000-0000-0000-000000000001" \
  -d '{"organization_name":"My Org","workspace_name":"Default"}'
# Expected: 201, {"organization":{"id":"...","name":"My Org","slug":"my-org",...},"workspace":{...}}

# POST /v1/organizations — should fail (limit 1)
curl -s -X POST http://localhost:8080/v1/organizations \
  -H "Content-Type: application/json" \
  -H "X-Agentclash-User-Id: 00000000-0000-0000-0000-000000000001" \
  -H "X-Agentclash-Org-Memberships: <org-uuid>:org_admin" \
  -d '{"name":"Second Org"}'
# Expected: 409, {"error":{"code":"organization_limit_reached",...}}

# GET /v1/organizations — list my orgs
curl -s http://localhost:8080/v1/organizations \
  -H "X-Agentclash-User-Id: 00000000-0000-0000-0000-000000000001" \
  -H "X-Agentclash-Org-Memberships: <org-uuid>:org_admin"
# Expected: 200, {"items":[...],"total":1,"limit":50,"offset":0}
```
