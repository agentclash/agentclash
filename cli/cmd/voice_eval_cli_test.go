package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestEvalPackListShowsVoiceMetadata(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/eval-packs": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":               "pack-voice",
				"name":             "Voice Support",
				"slug":             "voice-support",
				"lifecycle_status": "active",
				"versions": []map[string]any{{
					"id":                   "cpv-voice",
					"version_number":       2,
					"lifecycle_status":     "runnable",
					"modality":             "voice",
					"interface_transports": []string{"text_sim"},
				}},
			}},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	if err := executeCommand(t, []string{"eval-pack", "list", "-w", "ws-1"}, srv.URL); err != nil {
		t.Fatalf("eval-pack list error: %v", err)
	}

	out := stdout.finish()
	for _, want := range []string{"MODALITY", "Voice Support", "voice / text_sim"} {
		if !strings.Contains(out, want) {
			t.Fatalf("eval-pack list missing %q\n---\n%s", want, out)
		}
	}
}

func TestRunListShowsVoiceMetadata(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/runs": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":             "run-voice",
				"name":           "Voice run",
				"status":         "completed",
				"execution_mode": "single_agent",
				"agent_count":    1,
				"created_at":     "2026-05-13T18:00:00Z",
				"voice": map[string]any{
					"mode":      "text-sim",
					"modality":  "voice",
					"transport": "text_sim",
				},
			}},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	if err := executeCommand(t, []string{"run", "list", "-w", "ws-1"}, srv.URL); err != nil {
		t.Fatalf("run list error: %v", err)
	}

	out := stdout.finish()
	for _, want := range []string{"MODE", "single_agent; voice / Text simulation / text_sim"} {
		if !strings.Contains(out, want) {
			t.Fatalf("run list missing %q\n---\n%s", want, out)
		}
	}
}

func TestRunGetShowsNestedVoiceMetadata(t *testing.T) {
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/runs/run-voice": jsonHandler(200, map[string]any{
			"id":             "run-voice",
			"name":           "Voice run",
			"status":         "completed",
			"workspace_id":   "ws-1",
			"execution_mode": "single_agent",
			"created_at":     "2026-05-13T18:00:00Z",
			"voice": map[string]any{
				"mode":      "text-sim",
				"modality":  "voice",
				"transport": "text_sim",
			},
		}),
	})
	defer srv.Close()

	stdout := captureStdout(t)
	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	if err := executeCommand(t, []string{"run", "get", "run-voice"}, srv.URL); err != nil {
		t.Fatalf("run get error: %v", err)
	}

	out := stdout.finish()
	for _, want := range []string{"Execution Mode", "single_agent", "Mode", "Text simulation", "Voice", "voice / Text simulation / text_sim"} {
		if !strings.Contains(out, want) {
			t.Fatalf("run get missing %q\n---\n%s", want, out)
		}
	}
}

func TestEvalPackPickerLabelsVoiceVersions(t *testing.T) {
	pack := evalPackSummary{
		ID:   "pack-voice",
		Name: "Voice Support",
		Versions: []evalPackVersionBrief{{
			ID:                  "cpv-voice",
			VersionNumber:       3,
			LifecycleStatus:     "runnable",
			Modality:            "voice",
			InterfaceTransports: []string{"text_sim"},
		}},
	}

	if got := evalPackPickerLabel(pack); got != "Voice Support (voice)" {
		t.Fatalf("evalPackPickerLabel() = %q", got)
	}
	if got := evalPackPickerDescription(pack); !strings.Contains(got, "text_sim") {
		t.Fatalf("evalPackPickerDescription() = %q, want transport", got)
	}
	if got := evalPackVersionPickerLabel(pack.Versions[0]); got != "v3 (voice)" {
		t.Fatalf("evalPackVersionPickerLabel() = %q", got)
	}
	if got := suggestedRunModeForVersion(pack.Versions[0]); got != "text-sim" {
		t.Fatalf("suggestedRunModeForVersion() = %q, want text-sim", got)
	}
}

func TestRunCreateGuidedVoiceSelectionAutoPostsTextSimMode(t *testing.T) {
	oldInteractive := isInteractiveTerminal
	oldPickerFactory := newInteractivePicker
	isInteractiveTerminal = func(*RunContext) bool { return true }
	newInteractivePicker = func() interactivePicker { return &fakePicker{} }
	t.Cleanup(func() {
		isInteractiveTerminal = oldInteractive
		newInteractivePicker = oldPickerFactory
	})

	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/eval-packs": jsonHandler(200, map[string]any{
			"items": []map[string]any{{
				"id":   "pack-voice",
				"name": "Voice Support",
				"versions": []map[string]any{{
					"id":                   "cpv-voice",
					"version_number":       1,
					"lifecycle_status":     "runnable",
					"modality":             "voice",
					"interface_transports": []string{"text_sim"},
				}},
			}},
		}),
		"GET /v1/workspaces/ws-1/eval-pack-versions/cpv-voice/input-sets": jsonHandler(200, map[string]any{
			"items": []map[string]any{},
		}),
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			jsonHandler(http.StatusCreated, map[string]any{
				"id":     "run-voice",
				"status": "queued",
			})(w, r)
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-token")
	if err := executeCommand(t, []string{"run", "create", "-w", "ws-1", "--deployments", "dep-1"}, srv.URL); err != nil {
		t.Fatalf("run create error: %v", err)
	}
	if gotBody["mode"] != "text-sim" {
		t.Fatalf("mode = %v, want text-sim in guided voice run", gotBody["mode"])
	}
}
