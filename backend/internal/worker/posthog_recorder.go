package worker

import (
	"context"
	"log/slog"

	"github.com/agentclash/agentclash/backend/internal/posthog"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

// RunMetadataLookup resolves a run's workspace/org/creator so lifecycle
// analytics events can be attributed to a user and a workspace. Implemented by
// *repository.Repository (GetRunAnalyticsMetadata).
type RunMetadataLookup interface {
	GetRunAnalyticsMetadata(ctx context.Context, runID uuid.UUID) (repository.RunAnalyticsMetadata, error)
}

// PostHogRecorder decorates a RunEventRecorder and emits a PostHog analytics
// event for run-lifecycle transitions (run started / completed / failed).
// Every other event type passes straight through. Emission is best-effort: it
// never blocks the request path beyond the enqueue and never affects whether
// the underlying event was recorded.
//
// distinct_id is the run creator's user UUID (when known) so a run's outcome
// stitches onto the same PostHog person that created it — completing the
// "user created run -> run completed" funnel. When the creator is unknown the
// event is emitted without a person profile ($process_person_profile=false)
// and remains analyzable by its workspace_id / model / status properties.
type PostHogRecorder struct {
	inner  RunEventRecorder
	hog    posthog.Client
	lookup RunMetadataLookup
	logger *slog.Logger
}

var _ RunEventRecorder = (*PostHogRecorder)(nil)

// NewPostHogRecorder wraps inner. lookup may be nil (events then carry no
// workspace/user attribution). A nil hog degrades to the posthog noop.
func NewPostHogRecorder(inner RunEventRecorder, hog posthog.Client, lookup RunMetadataLookup, logger *slog.Logger) *PostHogRecorder {
	if hog == nil {
		hog = posthog.Noop{}
	}
	return &PostHogRecorder{inner: inner, hog: hog, lookup: lookup, logger: logger}
}

func (r *PostHogRecorder) RecordRunEvent(ctx context.Context, params repository.RecordRunEventParams) (repository.RunEvent, error) {
	event, err := r.inner.RecordRunEvent(ctx, params)
	if err != nil {
		return event, err
	}
	if name, ok := runLifecycleEventName(params.Event.EventType); ok {
		r.emit(ctx, name, params.Event)
	}
	return event, nil
}

func runLifecycleEventName(t runevents.Type) (string, bool) {
	switch t {
	case runevents.EventTypeSystemRunStarted:
		return "run.started", true
	case runevents.EventTypeSystemRunCompleted:
		return "run.completed", true
	case runevents.EventTypeSystemRunFailed:
		return "run.failed", true
	default:
		return "", false
	}
}

func (r *PostHogRecorder) emit(ctx context.Context, eventName string, env runevents.Envelope) {
	props := map[string]any{
		"run_id":       env.RunID.String(),
		"run_agent_id": env.RunAgentID.String(),
		"source":       string(env.Source),
	}
	if env.Summary.Status != "" {
		props["status"] = env.Summary.Status
	}
	if env.Summary.ProviderModelID != "" {
		props["model"] = env.Summary.ProviderModelID
	}
	if env.Summary.ProviderKey != "" {
		props["provider"] = env.Summary.ProviderKey
	}

	distinctID := posthog.AnonymousDistinctID()
	attributed := false
	if r.lookup != nil {
		if meta, err := r.lookup.GetRunAnalyticsMetadata(ctx, env.RunID); err != nil {
			if r.logger != nil {
				r.logger.WarnContext(ctx, "posthog recorder: run metadata lookup failed",
					"run_id", env.RunID, "event", eventName, "error", err)
			}
		} else {
			if meta.WorkspaceID != uuid.Nil {
				props["workspace_id"] = meta.WorkspaceID.String()
			}
			if meta.OrganizationID != uuid.Nil {
				props["org_id"] = meta.OrganizationID.String()
			}
			if meta.CreatedByUserID != nil && *meta.CreatedByUserID != uuid.Nil {
				distinctID = meta.CreatedByUserID.String()
				attributed = true
			}
		}
	}
	if !attributed {
		// Don't create a PostHog person profile for unattributed run events.
		props["$process_person_profile"] = false
	}

	r.hog.Capture(posthog.Event{
		DistinctID: distinctID,
		EventName:  eventName,
		Properties: props,
	})
}
