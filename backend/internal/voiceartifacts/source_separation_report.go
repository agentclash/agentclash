package voiceartifacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	SourceSeparationReportType       = "agentclash.voice.source_separation_eval.v1"
	VoiceySourceSeparationReportType = "voicey.source_separation_eval"
)

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
	return IngestSourceSeparationReport(data)
}

func IngestSourceSeparationReport(data []byte) (SourceSeparationReport, error) {
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
	if !isAcceptedReportType(r.Type, SourceSeparationReportType, VoiceySourceSeparationReportType) {
		return fmt.Errorf("type must be %q or %q", SourceSeparationReportType, VoiceySourceSeparationReportType)
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
	for _, metric := range []struct {
		name  string
		value *float64
	}{
		{name: "metrics.dialogue_retention_ratio", value: r.Metrics.DialogueRetentionRatio},
		{name: "metrics.background_preservation_ratio", value: r.Metrics.BackgroundPreservationRatio},
		{name: "metrics.speech_drop_risk", value: r.Metrics.SpeechDropRisk},
		{name: "metrics.background_leakage_in_dialogue_ratio", value: r.Metrics.BackgroundLeakageInDialogueRatio},
		{name: "metrics.dialogue_leakage_in_background_ratio", value: r.Metrics.DialogueLeakageInBackgroundRatio},
	} {
		name := metric.name
		value := metric.value
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
		DialogueRetentionRatio:           cloneFloat64(r.Metrics.DialogueRetentionRatio),
		BackgroundPreservationRatio:      cloneFloat64(r.Metrics.BackgroundPreservationRatio),
		SpeechDropRisk:                   cloneFloat64(r.Metrics.SpeechDropRisk),
		BackgroundLeakageInDialogueRatio: cloneFloat64(r.Metrics.BackgroundLeakageInDialogueRatio),
		DialogueLeakageInBackgroundRatio: cloneFloat64(r.Metrics.DialogueLeakageInBackgroundRatio),
		Caveats:                          append([]string(nil), r.Caveats...),
		AgentNotes:                       append([]string(nil), r.AgentNotes...),
	}
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func isAcceptedReportType(value string, accepted ...string) bool {
	value = strings.TrimSpace(value)
	for _, candidate := range accepted {
		if value == candidate {
			return true
		}
	}
	return false
}
