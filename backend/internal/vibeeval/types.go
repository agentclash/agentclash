// Package vibeeval is the control-plane guide agent for Vibe Eval: a bounded, SSE-streamed
// LLM tool-calling loop that drives an eval-authoring conversation via narrow semantic
// tools. Per the #875 design it must NOT import internal/api — identity and authorization
// arrive through the small Actor/WorkspaceAuthorizer contracts below, which the api layer
// bridges from api.Caller. See docs/vibe-eval-backend-design.md §11.1.
package vibeeval

import (
	"context"

	"github.com/google/uuid"
)

// Actor is audit/authorship identity only (never api.Caller). Authorization is performed
// through WorkspaceAuthorizer, not by inspecting Actor — Actor carries no roles/memberships
// so it cannot be used to make a policy decision by mistake (§11.1).
type Actor struct {
	UserID uuid.UUID
}

// WorkspaceAuthorizer authorizes an action (by NAME, a plain string) in a workspace. The
// api layer supplies a per-turn implementation that closes over the authenticated
// api.Caller and bridges to api.AuthorizeWorkspaceAction. vibeeval holds action names as
// strings and never decides role floors.
type WorkspaceAuthorizer interface {
	Authorize(ctx context.Context, workspaceID uuid.UUID, action string) error
}

// Conversation is the workspace-scoped guide conversation a turn runs against. Mirrors the
// vibe_eval_conversations row (migration 00049) without depending on the repository types.
type Conversation struct {
	ID             uuid.UUID
	WorkspaceID    uuid.UUID
	OrganizationID uuid.UUID
	Phase          string // plan|author|validate|publish|run|analyze|regress|admin
}

// RiskTier classifies a tool for confirmation/credit handling (Phase 0 matrix). Step 2 only
// exercises ReadTier; the higher tiers gate confirmation (Step 3) and credit (Step 4).
type RiskTier string

const (
	ReadTier           RiskTier = "read"
	DraftTier          RiskTier = "draft"
	WorkspaceWriteTier RiskTier = "workspace_write"
	CostIncurringTier  RiskTier = "cost_incurring"
	AdminSensitiveTier RiskTier = "admin_sensitive"
	DestructiveTier    RiskTier = "destructive_external"
)

// Conversation phases.
const (
	PhasePlan     = "plan"
	PhaseAuthor   = "author"
	PhaseValidate = "validate"
	PhasePublish  = "publish"
	PhaseRun      = "run"
	PhaseAnalyze  = "analyze"
	PhaseRegress  = "regress"
	PhaseAdmin    = "admin"
)
