package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	survey "github.com/AlecAivazis/survey/v2"
)

type errPicker struct {
	err error
}

func (p *errPicker) Select(_ string, _ []pickerOption) (pickerOption, error) {
	return pickerOption{}, p.err
}

func (p *errPicker) MultiSelect(_ string, _ []pickerOption, _ int) ([]pickerOption, error) {
	return nil, p.err
}

type fakePicker struct {
	selectIndices      []int
	multiSelectIndices [][]int
	selectCalls        int
	multiSelectCalls   int
}

func (p *fakePicker) Select(_ string, options []pickerOption) (pickerOption, error) {
	if p.selectCalls >= len(p.selectIndices) {
		return pickerOption{}, fmt.Errorf("unexpected select call %d", p.selectCalls+1)
	}
	index := p.selectIndices[p.selectCalls]
	p.selectCalls++
	if index < 0 || index >= len(options) {
		return pickerOption{}, fmt.Errorf("select index %d out of range", index)
	}
	return options[index], nil
}

func (p *fakePicker) MultiSelect(_ string, options []pickerOption, _ int) ([]pickerOption, error) {
	if p.multiSelectCalls >= len(p.multiSelectIndices) {
		return nil, fmt.Errorf("unexpected multiselect call %d", p.multiSelectCalls+1)
	}
	indices := p.multiSelectIndices[p.multiSelectCalls]
	p.multiSelectCalls++

	selected := make([]pickerOption, 0, len(indices))
	for _, index := range indices {
		if index < 0 || index >= len(options) {
			return nil, fmt.Errorf("multiselect index %d out of range", index)
		}
		selected = append(selected, options[index])
	}
	return selected, nil
}

func TestSelectOneOrAutoSkipsPromptForSingleOption(t *testing.T) {
	picker := &fakePicker{}
	option, err := selectOneOrAuto(picker, "Choose one", []pickerOption{{Label: "only", Value: "1"}})
	if err != nil {
		t.Fatalf("selectOneOrAuto error: %v", err)
	}
	if option.Value != "1" {
		t.Fatalf("selected value = %q, want 1", option.Value)
	}
	if picker.selectCalls != 0 {
		t.Fatalf("picker select calls = %d, want 0", picker.selectCalls)
	}
}

func TestSelectOneOrAutoErrorsOnEmptyOptions(t *testing.T) {
	_, err := selectOneOrAuto(&fakePicker{}, "Choose one", nil)
	if err == nil || err.Error() != "no options available for Choose one" {
		t.Fatalf("error = %v, want empty-options error", err)
	}
}

func TestSelectOneOrAutoPropagatesPickerErrors(t *testing.T) {
	wantErr := fmt.Errorf("picker cancelled")
	_, err := selectOneOrAuto(&errPicker{err: wantErr}, "Choose one", []pickerOption{
		{Label: "first", Value: "1"},
		{Label: "second", Value: "2"},
	})
	if err != wantErr {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestSelectManyOrAutoSkipsPromptWhenOnlyMinimumChoicesExist(t *testing.T) {
	picker := &fakePicker{}
	selected, err := selectManyOrAuto(picker, "Choose deployments", []pickerOption{{Label: "only", Value: "dep-1"}}, 1)
	if err != nil {
		t.Fatalf("selectManyOrAuto error: %v", err)
	}
	if len(selected) != 1 || selected[0].Value != "dep-1" {
		t.Fatalf("selected = %#v, want dep-1", selected)
	}
	if picker.multiSelectCalls != 0 {
		t.Fatalf("picker multiselect calls = %d, want 0", picker.multiSelectCalls)
	}
}

func TestSelectManyOrAutoErrorsOnEmptyOptions(t *testing.T) {
	_, err := selectManyOrAuto(&fakePicker{}, "Choose deployments", nil, 1)
	if err == nil || err.Error() != "no options available for Choose deployments" {
		t.Fatalf("error = %v, want empty-options error", err)
	}
}

func TestNormalizedPickerOptionsAvoidsPostNormalizationCollisions(t *testing.T) {
	options := normalizedPickerOptions([]pickerOption{
		{Label: "Fraud Ops", Value: "pack-1"},
		{Label: "Fraud Ops", Value: "pack-2"},
		{Label: "Fraud Ops [pack-2]", Value: "pack-3"},
	})

	got := []string{options[0].Label, options[1].Label, options[2].Label}
	want := []string{
		"Fraud Ops [pack-1]",
		"Fraud Ops [pack-2]",
		"Fraud Ops [pack-2] (2)",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("normalized labels = %#v, want %#v", got, want)
		}
	}
}

func TestSurveyPickerMultiSelectPreservesSelectionsOnSubmit(t *testing.T) {
	oldSurveyAskOne := surveyAskOne
	surveyAskOne = func(prompt survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		var askOptions survey.AskOptions
		for _, opt := range opts {
			if err := opt(&askOptions); err != nil {
				return err
			}
		}

		selected := []survey.OptionAnswer{
			{Value: "baseline", Index: 0},
			{Value: "candidate", Index: 1},
		}
		for _, validator := range askOptions.Validators {
			if err := validator(selected); err != nil {
				return err
			}
		}

		resolved, ok := response.(*[]string)
		if !ok {
			t.Fatalf("response type = %T, want *[]string", response)
		}
		*resolved = []string{"baseline", "candidate"}
		return nil
	}
	t.Cleanup(func() { surveyAskOne = oldSurveyAskOne })

	picker := &surveyPicker{}
	selected, err := picker.MultiSelect("Choose deployments", []pickerOption{
		{Label: "baseline", Value: "dep-a"},
		{Label: "candidate", Value: "dep-b"},
	}, 1)
	if err != nil {
		t.Fatalf("MultiSelect error: %v", err)
	}

	if len(selected) != 2 {
		t.Fatalf("selected length = %d, want 2", len(selected))
	}
	if selected[0].Value != "dep-a" || selected[1].Value != "dep-b" {
		t.Fatalf("selected values = %#v, want dep-a and dep-b", selected)
	}
}

func TestRunCreateGuidedSelectionPostsResolvedIDs(t *testing.T) {
	picker := &fakePicker{
		selectIndices:      []int{1, 0, 1},
		multiSelectIndices: [][]int{{2, 0}},
	}
	oldInteractive := isInteractiveTerminal
	oldPickerFactory := newInteractivePicker
	isInteractiveTerminal = func(*RunContext) bool { return true }
	newInteractivePicker = func() interactivePicker { return picker }
	t.Cleanup(func() {
		isInteractiveTerminal = oldInteractive
		newInteractivePicker = oldPickerFactory
	})

	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"GET /v1/workspaces/ws-1/challenge-packs": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{
					"id":   "pack-1",
					"name": "Customer Support",
					"versions": []map[string]any{
						{"id": "cpv-1", "version_number": 1, "lifecycle_status": "active"},
					},
				},
				{
					"id":   "pack-2",
					"name": "Fraud Ops",
					"versions": []map[string]any{
						{"id": "cpv-3", "version_number": 1, "lifecycle_status": "active"},
						{"id": "cpv-2", "version_number": 2, "lifecycle_status": "active"},
					},
				},
			},
		}),
		"GET /v1/workspaces/ws-1/challenge-pack-versions/cpv-2/input-sets": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "input-1", "name": "Small", "input_key": "small"},
				{"id": "input-2", "name": "Large", "input_key": "large"},
			},
		}),
		"GET /v1/workspaces/ws-1/agent-deployments": jsonHandler(200, map[string]any{
			"items": []map[string]any{
				{"id": "dep-a", "name": "baseline", "status": "active"},
				{"id": "dep-b", "name": "candidate", "status": "active"},
				{"id": "dep-c", "name": "shadow", "status": "active"},
			},
		}),
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "run-1", "status": "queued"})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{"run", "create", "-w", "ws-1", "--name", "nightly"}, srv.URL); err != nil {
		t.Fatalf("run create error: %v", err)
	}

	if gotBody["challenge_pack_version_id"] != "cpv-2" {
		t.Fatalf("challenge_pack_version_id = %v, want cpv-2", gotBody["challenge_pack_version_id"])
	}
	if gotBody["challenge_input_set_id"] != "input-2" {
		t.Fatalf("challenge_input_set_id = %v, want input-2", gotBody["challenge_input_set_id"])
	}
	deploymentIDs, ok := gotBody["agent_deployment_ids"].([]any)
	if !ok {
		t.Fatalf("agent_deployment_ids type = %T, want []any", gotBody["agent_deployment_ids"])
	}
	if len(deploymentIDs) != 2 || deploymentIDs[0] != "dep-c" || deploymentIDs[1] != "dep-a" {
		t.Fatalf("agent_deployment_ids = %#v, want [dep-c dep-a]", deploymentIDs)
	}
	if picker.selectCalls != 3 {
		t.Fatalf("picker select calls = %d, want 3", picker.selectCalls)
	}
	if picker.multiSelectCalls != 1 {
		t.Fatalf("picker multiselect calls = %d, want 1", picker.multiSelectCalls)
	}
}

func TestRunCreateNonInteractiveRequiresExplicitFlags(t *testing.T) {
	oldInteractive := isInteractiveTerminal
	isInteractiveTerminal = func(*RunContext) bool { return false }
	t.Cleanup(func() { isInteractiveTerminal = oldInteractive })

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{"run", "create", "-w", "ws-1"}, "http://unused")
	if err == nil {
		t.Fatal("expected non-interactive validation error")
	}
	if got := err.Error(); got != "challenge pack version and deployment selection required in non-interactive mode; pass --challenge-pack-version and --deployments or rerun `agentclash run create` in a TTY for guided selection" {
		t.Fatalf("error = %q", got)
	}
}

func TestRunCreateExplicitFlagsBypassGuidedPrompts(t *testing.T) {
	oldInteractive := isInteractiveTerminal
	oldPickerFactory := newInteractivePicker
	isInteractiveTerminal = func(*RunContext) bool { return true }
	newInteractivePicker = func() interactivePicker {
		return &fakePicker{
			selectIndices:      nil,
			multiSelectIndices: nil,
		}
	}
	t.Cleanup(func() {
		isInteractiveTerminal = oldInteractive
		newInteractivePicker = oldPickerFactory
	})

	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "run-1", "status": "queued"})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "create",
		"-w", "ws-1",
		"--challenge-pack-version", "cpv-explicit",
		"--input-set", "input-explicit",
		"--deployments", "dep-a,dep-b",
	}, srv.URL); err != nil {
		t.Fatalf("run create error: %v", err)
	}

	if gotBody["challenge_pack_version_id"] != "cpv-explicit" {
		t.Fatalf("challenge_pack_version_id = %v, want cpv-explicit", gotBody["challenge_pack_version_id"])
	}
	if gotBody["challenge_input_set_id"] != "input-explicit" {
		t.Fatalf("challenge_input_set_id = %v, want input-explicit", gotBody["challenge_input_set_id"])
	}
}

func TestRunCreateRaceContextFlagsPropagate(t *testing.T) {
	oldInteractive := isInteractiveTerminal
	oldPickerFactory := newInteractivePicker
	isInteractiveTerminal = func(*RunContext) bool { return true }
	newInteractivePicker = func() interactivePicker {
		return &fakePicker{selectIndices: nil, multiSelectIndices: nil}
	}
	t.Cleanup(func() {
		isInteractiveTerminal = oldInteractive
		newInteractivePicker = oldPickerFactory
	})

	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "run-race", "status": "queued"})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "create",
		"-w", "ws-1",
		"--challenge-pack-version", "cpv-explicit",
		"--deployments", "dep-a,dep-b",
		"--race-context",
		"--race-context-cadence", "4",
	}, srv.URL); err != nil {
		t.Fatalf("run create error: %v", err)
	}

	if gotBody["race_context"] != true {
		t.Fatalf("race_context = %v, want true", gotBody["race_context"])
	}
	// JSON decodes numeric fields as float64 unless explicitly typed.
	if got, ok := gotBody["race_context_min_step_gap"].(float64); !ok || got != 4 {
		t.Fatalf("race_context_min_step_gap = %v (%T), want 4", gotBody["race_context_min_step_gap"], gotBody["race_context_min_step_gap"])
	}
}

func TestRunCreateRaceContextCadenceOutOfRangeFailsLocally(t *testing.T) {
	oldInteractive := isInteractiveTerminal
	oldPickerFactory := newInteractivePicker
	isInteractiveTerminal = func(*RunContext) bool { return true }
	newInteractivePicker = func() interactivePicker {
		return &fakePicker{selectIndices: nil, multiSelectIndices: nil}
	}
	t.Cleanup(func() {
		isInteractiveTerminal = oldInteractive
		newInteractivePicker = oldPickerFactory
	})

	// The server also rejects values outside [1, 10] with a 400, but the
	// CLI should refuse before issuing the POST so users get a clear
	// message without a round-trip. Handler should never be hit.
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("server was reached despite out-of-range cadence; CLI should have rejected locally")
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	err := executeCommand(t, []string{
		"run", "create",
		"-w", "ws-1",
		"--challenge-pack-version", "cpv-explicit",
		"--deployments", "dep-a,dep-b",
		"--race-context",
		"--race-context-cadence", "15",
	}, srv.URL)
	if err == nil {
		t.Fatalf("expected error for out-of-range cadence")
	}
	if !strings.Contains(err.Error(), "between 1 and 10") {
		t.Errorf("error message = %q, want to mention the range", err.Error())
	}
}

func TestRunCreateWithoutRaceContextFlagsOmitsFields(t *testing.T) {
	oldInteractive := isInteractiveTerminal
	oldPickerFactory := newInteractivePicker
	isInteractiveTerminal = func(*RunContext) bool { return true }
	newInteractivePicker = func() interactivePicker {
		return &fakePicker{selectIndices: nil, multiSelectIndices: nil}
	}
	t.Cleanup(func() {
		isInteractiveTerminal = oldInteractive
		newInteractivePicker = oldPickerFactory
	})

	var gotBody map[string]any
	srv := fakeAPI(t, map[string]http.HandlerFunc{
		"POST /v1/runs": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "run-norace", "status": "queued"})
		},
	})
	defer srv.Close()

	t.Setenv("AGENTCLASH_TOKEN", "test-tok")
	if err := executeCommand(t, []string{
		"run", "create",
		"-w", "ws-1",
		"--challenge-pack-version", "cpv-explicit",
		"--deployments", "dep-a",
	}, srv.URL); err != nil {
		t.Fatalf("run create error: %v", err)
	}

	if _, present := gotBody["race_context"]; present {
		t.Fatalf("race_context present when flag not set, want absent; body = %#v", gotBody)
	}
	if _, present := gotBody["race_context_min_step_gap"]; present {
		t.Fatalf("race_context_min_step_gap present when flag not set, want absent")
	}
}
