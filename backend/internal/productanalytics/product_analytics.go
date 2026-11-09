// Package productanalytics owns the canonical acquisition-to-first-run
// product event contract. It deliberately accepts only pseudonymous IDs and
// wraps an optional PostHog client, so recording never gates product state.
package productanalytics

import (
	"context"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/posthog"
	"github.com/google/uuid"
)

const SchemaVersion = 1

const (
	AccountSignupCompleted = "product.account.signup_completed"
	OrganizationCreated    = "product.organization.created"
	WorkspaceCreated       = "product.workspace.created"
	ProviderAccountCreated = "product.provider_account.created"
	AgentDeploymentCreated = "product.agent_deployment.created"
	ChallengePackPublished = "product.challenge_pack.published"
	RunCreated             = "product.run.created"
	RunStarted             = "product.run.started"
	RunCompleted           = "product.run.completed"
	RunFailed              = "product.run.failed"
	RunCancelled           = "product.run.cancelled"
	TryoutClaimed          = "product.tryout.claimed"
	SDKReportImported      = "product.sdk_report.imported"
)

type Surface string

const (
	SurfaceWeb Surface = "web"
	SurfaceCLI Surface = "cli"
	SurfaceAPI Surface = "api"
)

type Event struct {
	Name           string
	DistinctID     uuid.UUID
	EntityID       uuid.UUID
	OrganizationID uuid.UUID
	WorkspaceID    uuid.UUID
	Surface        Surface
	// DedupeKey distinguishes multiple canonical milestones for the same entity
	// when the event name alone is insufficient (for example a status value).
	DedupeKey  string
	Properties map[string]any
}

type ProductAnalytics interface {
	Record(context.Context, Event)
}

// Record safely invokes an optional recorder. It keeps manager zero values and
// existing tests no-op capable without analytics-specific construction.
func Record(recorder ProductAnalytics, ctx context.Context, event Event) {
	if recorder != nil {
		recorder.Record(ctx, event)
	}
}

type Recorder struct {
	client posthog.Client
}

func New(client posthog.Client) *Recorder {
	if client == nil {
		client = posthog.Noop{}
	}
	return &Recorder{client: client}
}

func (r *Recorder) Record(ctx context.Context, event Event) {
	if r == nil || event.Name == "" || event.DistinctID == uuid.Nil || event.EntityID == uuid.Nil {
		return
	}

	properties := make(map[string]any, len(event.Properties)+5)
	properties["schema_version"] = SchemaVersion
	properties["observed_host"] = ObservedHostFromContext(ctx)
	properties["workflow_kind"] = workflowKind(event.Name)
	properties[entityProperty(event.Name)] = event.EntityID.String()
	if event.OrganizationID != uuid.Nil {
		properties["org_id"] = event.OrganizationID.String()
	}
	if event.WorkspaceID != uuid.Nil {
		properties["workspace_id"] = event.WorkspaceID.String()
	}
	surface := event.Surface
	if surface == "" {
		surface = SurfaceFromContext(ctx)
	}
	if surface.Valid() {
		properties["surface"] = string(surface)
	}
	for key, value := range event.Properties {
		if safePropertyKey(key) && !reservedProperty(key) {
			properties[key] = value
		}
	}

	r.client.Capture(posthog.Event{
		DistinctID: event.DistinctID.String(),
		EventName:  event.Name,
		Properties: properties,
		UUID:       DeterministicUUID(event.Name, event.EntityID, event.DedupeKey).String(),
	})
}

func DeterministicUUID(eventName string, entityID uuid.UUID, dedupeKey string) uuid.UUID {
	return uuid.NewSHA1(eventNamespace, []byte(eventName+":"+entityID.String()+":"+dedupeKey))
}

func entityProperty(eventName string) string {
	switch eventName {
	case AccountSignupCompleted:
		return "user_id"
	case OrganizationCreated:
		return "organization_id"
	case WorkspaceCreated:
		return "workspace_id"
	case ProviderAccountCreated:
		return "provider_account_id"
	case AgentDeploymentCreated:
		return "agent_deployment_id"
	case ChallengePackPublished:
		return "challenge_pack_id"
	case RunCreated, RunStarted, RunCompleted, RunFailed, RunCancelled:
		return "run_id"
	case TryoutClaimed:
		return "tryout_id"
	case SDKReportImported:
		return "sdk_report_id"
	default:
		return "entity_id"
	}
}

func safePropertyKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	switch normalized {
	case "email", "display_name", "name", "account_name", "run_name", "credentials", "form_contents":
		return false
	}
	return !strings.Contains(normalized, "password") &&
		!strings.Contains(normalized, "secret") &&
		!strings.Contains(normalized, "credential") &&
		!strings.HasSuffix(normalized, "_token")
}

func (s Surface) Valid() bool {
	switch s {
	case SurfaceWeb, SurfaceCLI, SurfaceAPI:
		return true
	default:
		return false
	}
}

type surfaceContextKey struct{}
type actorContextKey struct{}
type pathContextKey struct{}

func WithSurface(ctx context.Context, surface Surface) context.Context {
	if !surface.Valid() {
		return ctx
	}
	return context.WithValue(ctx, surfaceContextKey{}, surface)
}

func SurfaceFromContext(ctx context.Context) Surface {
	surface, _ := ctx.Value(surfaceContextKey{}).(Surface)
	return surface
}

func WithActor(ctx context.Context, userID uuid.UUID) context.Context {
	if userID == uuid.Nil {
		return ctx
	}
	return context.WithValue(ctx, actorContextKey{}, userID)
}

func ActorFromContext(ctx context.Context) uuid.UUID {
	userID, _ := ctx.Value(actorContextKey{}).(uuid.UUID)
	return userID
}

func WithPath(ctx context.Context, path string) context.Context {
	path = strings.TrimSpace(path)
	if path == "" {
		return ctx
	}
	return context.WithValue(ctx, pathContextKey{}, path)
}

func PathFromContext(ctx context.Context) string {
	path, _ := ctx.Value(pathContextKey{}).(string)
	return path
}

type Noop struct{}

func (Noop) Record(context.Context, Event) {}

var eventNamespace = uuid.MustParse("c28b3020-a03c-5f58-92a3-3928d704ef3f")

var _ ProductAnalytics = (*Recorder)(nil)
var _ ProductAnalytics = Noop{}

// Host markers are explicit, self-reported observations, never proof of install.
func NormalizeObservedHost(host string) string {
	switch host {
	case "claude", "codex":
		return host
	default:
		return "unknown"
	}
}

type observedHostContextKey struct{}

func WithObservedHost(ctx context.Context, host string) context.Context {
	return context.WithValue(ctx, observedHostContextKey{}, NormalizeObservedHost(host))
}
func ObservedHostFromContext(ctx context.Context) string {
	host, _ := ctx.Value(observedHostContextKey{}).(string)
	return NormalizeObservedHost(host)
}
func workflowKind(name string) string {
	switch name {
	case RunCreated, RunStarted, RunCompleted, RunFailed, RunCancelled:
		return "hosted_run"
	case TryoutClaimed:
		return "platform_demo"
	case SDKReportImported:
		return "imported_local_evidence"
	default:
		return "unknown"
	}
}
func reservedProperty(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "schema_version", "observed_host", "workflow_kind", "surface", "org_id", "workspace_id", "user_id", "organization_id", "provider_account_id", "agent_deployment_id", "challenge_pack_id", "run_id", "tryout_id", "sdk_report_id", "entity_id":
		return true
	default:
		return false
	}
}
