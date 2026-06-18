package vibeeval

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/agentclash/agentclash/backend/internal/provider"
)

// Tool is one narrow semantic capability. Concrete tools are thin adapters living in
// package api over existing managers; vibeeval only sees this interface (§11.1). The action
// name is a plain string — api validates it against api.Action at registry construction.
type Tool interface {
	Name() string
	Phases() []string // which conversation phases load this tool
	RiskTier() RiskTier
	RequiredAction() string // action NAME; api validates against api.Action at registration
	Definition() provider.ToolDefinition

	// Execute runs the semantic action. args are already schema-validated and the loop has
	// already authorized via WorkspaceAuthorizer. actor is audit identity only.
	Execute(ctx context.Context, actor Actor, conv Conversation, args json.RawMessage) (ToolOutput, error)
}

// CostEstimator is implemented by cost-incurring tools. The engine calls EstimateCost at PROPOSE time
// for CostIncurringTier tools — after authorization, before persisting the confirmation — to produce
// the server-computed, immutable cost envelope shown on the confirmation card and bound at confirmed
// execution. The returned JSON is opaque to the engine; the tool owns its shape. An error means the
// action is unpriceable and MUST NOT be proposed.
type CostEstimator interface {
	EstimateCost(ctx context.Context, actor Actor, conv Conversation, args json.RawMessage) (json.RawMessage, error)
}

// ConfirmedTool is implemented by tools whose confirmed execution must read the approved confirmation
// (e.g. to reserve exactly the approved cost envelope). On resume, the engine calls ExecuteConfirmed
// for the matching call instead of Execute, passing the resolved PendingConfirmation. A ConfirmedTool's
// plain Execute should refuse (the action requires a confirmation's approved estimate).
type ConfirmedTool interface {
	ExecuteConfirmed(ctx context.Context, actor Actor, conv Conversation, args json.RawMessage, pc PendingConfirmation) (ToolOutput, error)
}

// ToolOutput is a tool's result. Result re-enters model context only after redaction
// (EvidenceRedactor). AuditResult is a metadata-only summary (no secrets/contents); the
// generalized audit log consumes it in Step 3.
type ToolOutput struct {
	Result      any
	AuditResult map[string]any
}

// ToolRegistry holds the registered tools and exposes the subset loaded for a phase.
type ToolRegistry interface {
	Register(t Tool)
	ForPhase(phase string) []Tool
	Resolve(name string) (Tool, bool)
	// Definitions returns provider.ToolDefinition for ForPhase(phase), to send to the model.
	Definitions(phase string) []provider.ToolDefinition
}

// registry is the default in-memory ToolRegistry. Construct via NewRegistry.
type registry struct {
	byName  map[string]Tool
	byPhase map[string][]Tool
}

// NewRegistry returns an empty registry.
func NewRegistry() *registry {
	return &registry{
		byName:  make(map[string]Tool),
		byPhase: make(map[string][]Tool),
	}
}

func (r *registry) Register(t Tool) {
	name := t.Name()
	r.byName[name] = t
	for _, phase := range t.Phases() {
		r.byPhase[phase] = append(r.byPhase[phase], t)
	}
}

func (r *registry) ForPhase(phase string) []Tool {
	tools := append([]Tool(nil), r.byPhase[phase]...)
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name() < tools[j].Name() })
	return tools
}

func (r *registry) Resolve(name string) (Tool, bool) {
	t, ok := r.byName[name]
	return t, ok
}

func (r *registry) Definitions(phase string) []provider.ToolDefinition {
	tools := r.ForPhase(phase)
	defs := make([]provider.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		defs = append(defs, t.Definition())
	}
	return defs
}
