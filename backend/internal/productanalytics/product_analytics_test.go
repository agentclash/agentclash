package productanalytics

import (
	"context"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/posthog"
	"github.com/google/uuid"
)

type captureClient struct {
	events []posthog.Event
}

func (c *captureClient) Capture(event posthog.Event) {
	c.events = append(c.events, event)
}
func (*captureClient) Identify(string, map[string]any) {}
func (*captureClient) Close() error                    { return nil }

func TestRecorderAddsContractPropertiesAndDeterministicUUID(t *testing.T) {
	client := &captureClient{}
	recorder := New(client)
	userID := uuid.New()
	runID := uuid.New()
	orgID := uuid.New()
	workspaceID := uuid.New()
	ctx := WithSurface(context.Background(), SurfaceCLI)

	event := Event{
		Name:           RunCreated,
		DistinctID:     userID,
		EntityID:       runID,
		OrganizationID: orgID,
		WorkspaceID:    workspaceID,
		Properties: map[string]any{
			"agent_count":  2,
			"email":        "must-not-land@example.com",
			"display_name": "Must Not Land",
		},
	}
	recorder.Record(ctx, event)
	recorder.Record(ctx, event)

	if len(client.events) != 2 {
		t.Fatalf("captured events = %d, want 2", len(client.events))
	}
	got := client.events[0]
	if got.DistinctID != userID.String() || got.EventName != RunCreated {
		t.Fatalf("capture identity/name = %q/%q", got.DistinctID, got.EventName)
	}
	if got.UUID == "" || got.UUID != client.events[1].UUID {
		t.Fatalf("deterministic UUIDs = %q and %q", got.UUID, client.events[1].UUID)
	}
	if got.Properties["schema_version"] != SchemaVersion || got.Properties["surface"] != "cli" {
		t.Fatalf("contract properties = %#v", got.Properties)
	}
	if got.Properties["run_id"] != runID.String() || got.Properties["org_id"] != orgID.String() || got.Properties["workspace_id"] != workspaceID.String() {
		t.Fatalf("entity properties = %#v", got.Properties)
	}
	if _, ok := got.Properties["email"]; ok {
		t.Fatal("raw email reached PostHog properties")
	}
	if _, ok := got.Properties["display_name"]; ok {
		t.Fatal("display name reached PostHog properties")
	}
}

func TestDeterministicUUIDSeparatesMilestones(t *testing.T) {
	entityID := uuid.New()
	first := DeterministicUUID(RunCompleted, entityID, "completed")
	if first != DeterministicUUID(RunCompleted, entityID, "completed") {
		t.Fatal("same milestone did not produce the same UUID")
	}
	if first == DeterministicUUID(RunFailed, entityID, "failed") {
		t.Fatal("different milestone produced the same UUID")
	}
}

func TestObservedHostAndReservedProperties(t *testing.T) {
	for _, host := range []string{"claude", "codex", "ClaudeBot", "claude; private", ""} {
		client := &captureClient{}
		id := uuid.New()
		New(client).Record(WithObservedHost(context.Background(), host), Event{Name: RunCreated, EntityID: id, DistinctID: id, Properties: map[string]any{"observed_host": "forged", "run_id": "forged", "workflow_kind": "customer_app"}})
		want := "unknown"
		if host == "claude" || host == "codex" {
			want = host
		}
		p := client.events[0].Properties
		if p["observed_host"] != want || p["workflow_kind"] != "hosted_run" || p["run_id"] != id.String() {
			t.Fatal(p)
		}
	}
}
