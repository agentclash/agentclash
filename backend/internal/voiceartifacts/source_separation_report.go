package voiceartifacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const SourceSeparationReportType = "voicey.source_separation_eval"

type SourceSeparationReport struct {
	SchemaVersion string            `json:"schema_version"`
	Type          string            `json:"type"`
	Status        string            `json:"status"`
	Passed        bool              `json:"passed"`
	Metrics       SourceMixMetrics  `json:"metrics"`
	Caveats       []string          `json:"caveats,omitempty"`
	AgentNotes    []string          `json:"agentclash_notes,omitempty"`
	Artifacts     map[string]string `json:"artifacts,omitempty"`
	Raw           json.RawMessage   `json:"-"`
}

type SourceMixMetrics struct {
	DialogueRetentionRatio           *float64 `json:"dialogue_retention_ratio"`
	BackgroundPreservationRatio      *float64 `json:"background_preservation_ratio"`
	SpeechDropRisk                   *float64 `json:"speech_drop_risk"`
	BackgroundLeakageInDialogueRatio *float64 `json:"background_leakage_in_dialogue_ratio"`
	DialogueLeakageInBackgroundRatio *float64 `json:"dialogue_leakage_in_background_ratio"`
}

type MediaPolicyEvidence struct {
	Status                           string   `json:"status"`
	Passed                           bool     `json:"passed"`
	DialogueRetentionRatio           *float64 `json:"dialogue_retention_ratio,omitempty"`
	BackgroundPreservationRatio      *float64 `json:"background_preservation_ratio,omitempty"`
	SpeechDropRisk                   *float64 `json:"speech_drop_risk,omitempty"`
	BackgroundLeakageInDialogueRatio *float64 `json:"background_leakage_in_dialogue_ratio,omitempty"`
	DialogueLeakageInBackgroundRatio *float64 `json:"dialogue_leakage_in_background_ratio,omitempty"`
	Caveats                          []string `json:"caveats,omitempty"`
	AgentNotes                       []string `json:"agentclash_notes,omitempty"`
}

func LoadSourceSeparationReport(path string) (SourceSeparationReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SourceSeparationReport{}, err
	}
	var report SourceSeparationReport
	if err := json.Unmarshal(data, &report); err != nil {
		return SourceSeparationReport{}, fmt.Errorf("decode source separation report: %w", err)
	}
	report.Raw = append(json.RawMessage(nil), data...)
	if err := report.Validate(); err != nil {
		return SourceSeparationReport{}, err
	}
	return report, nil
}

func (r SourceSeparationReport) Validate() error {
	if strings.TrimSpace(r.SchemaVersion) == "" {
		return errors.New("schema_version is required")
	}
	if r.Type != SourceSeparationReportType {
		return fmt.Errorf("type must be %q", SourceSeparationReportType)
	}
	switch r.Status {
	case "passed", "failed", "degraded":
	default:
		return errors.New("status must be one of passed, failed, degraded")
	}
	if r.Passed != (r.Status == "passed") {
		return errors.New("passed must be true only when status is passed")
	}
	if r.Metrics.DialogueRetentionRatio == nil {
		return errors.New("metrics.dialogue_retention_ratio is required")
	}
	if r.Metrics.BackgroundPreservationRatio == nil {
		return errors.New("metrics.background_preservation_ratio is required")
	}
	if r.Metrics.SpeechDropRisk == nil {
		return errors.New("metrics.speech_drop_risk is required")
	}
	for name, value := range map[string]*float64{
		"metrics.dialogue_retention_ratio":             r.Metrics.DialogueRetentionRatio,
		"metrics.background_preservation_ratio":        r.Metrics.BackgroundPreservationRatio,
		"metrics.speech_drop_risk":                     r.Metrics.SpeechDropRisk,
		"metrics.background_leakage_in_dialogue_ratio": r.Metrics.BackgroundLeakageInDialogueRatio,
		"metrics.dialogue_leakage_in_background_ratio": r.Metrics.DialogueLeakageInBackgroundRatio,
	} {
		if value != nil && (*value < 0 || *value > 1) {
			return fmt.Errorf("%s must be between 0 and 1", name)
		}
	}
	return nil
}

func (r SourceSeparationReport) MediaPolicyEvidence() MediaPolicyEvidence {
	return MediaPolicyEvidence{
		Status:                           r.Status,
		Passed:                           r.Passed,
		DialogueRetentionRatio:           r.Metrics.DialogueRetentionRatio,
		BackgroundPreservationRatio:      r.Metrics.BackgroundPreservationRatio,
		SpeechDropRisk:                   r.Metrics.SpeechDropRisk,
		BackgroundLeakageInDialogueRatio: r.Metrics.BackgroundLeakageInDialogueRatio,
		DialogueLeakageInBackgroundRatio: r.Metrics.DialogueLeakageInBackgroundRatio,
		Caveats:                          append([]string(nil), r.Caveats...),
		AgentNotes:                       append([]string(nil), r.AgentNotes...),
	}
}
