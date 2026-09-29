package vibe

import "github.com/agentclash/agentclash/backend/internal/vibe/inputs"

// Keep the legacy catalogue used by frozen prompts unchanged. Runtime readiness
// belongs to config responses; the worker still rechecks before reading a PDF.
func AvailableCapabilities(pdf bool) []Capability {
	return append(Capabilities(), Capability{ID: "pdf_text_extraction", Label: "PDF text", Available: pdf,
		Description: "Reads one text-based PDF, up to 30 pages and 10 MB. No OCR, image interpretation or linked-resource fetching."})
}

// These are formats this executor really supports, not a catalogue of jobs.
type InputContract struct {
	Version int      `json:"version"`
	Formats []string `json:"formats"`
	Label   string   `json:"label"`
}

// Written by completion after a target response, never supplied by the model.
// Materials carry exact immutable hashes; they are not entailment claims.
type ExecutionReceipt struct {
	Capabilities    []string `json:"capabilities"`
	TargetCompleted bool     `json:"target_completed"`
}

func materialContract() *InputContract {
	return &InputContract{Version: 1, Formats: []string{"text", "pdf_text"}, Label: "Add the material you want it to work on"}
}
func referenceInputs(p Plan) []inputs.Binding {
	if p.Artifact != nil && len(p.Artifact.ReferenceInputs) > 0 {
		return append([]inputs.Binding(nil), p.Artifact.ReferenceInputs...)
	}
	out := []inputs.Binding{}
	bindings := p.Submission.Inputs
	if p.Cycle != nil {
		bindings = p.Cycle.Materials
	}
	for _, b := range bindings {
		if b.Usage == "reference" {
			out = append(out, b)
		}
	}
	return out
}
func executionInputs(p Plan) []inputs.Binding {
	out := append([]inputs.Binding(nil), p.Submission.Inputs...)
	if p.Artifact != nil {
		for _, ref := range p.Artifact.ReferenceInputs {
			found := false
			for _, b := range out {
				found = found || b.ID == ref.ID
			}
			if !found {
				out = append(out, ref)
			}
		}
	}
	return out
}

func hasTaskInput(bindings []inputs.Binding) bool {
	for _, b := range bindings {
		if b.Usage == "task_input" {
			return true
		}
	}
	return false
}
