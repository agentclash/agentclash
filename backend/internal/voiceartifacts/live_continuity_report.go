package voiceartifacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
)

const (
	LiveContinuityReportType       = "agentclash.voice.live_continuity_eval.v1"
	VoiceyLiveContinuityReportType = "voicey.live_continuity_eval"
)

type LiveContinuityReport struct {
	SchemaVersion string                `json:"schema_version"`
	Type          string                `json:"type"`
	Status        string                `json:"status"`
	Passed        bool                  `json:"passed"`
	Input         string                `json:"input,omitempty"`
	Metrics       LiveContinuityMetrics `json:"metrics"`
	Caveats       []string              `json:"caveats,omitempty"`
	AgentNotes    []string              `json:"agentclash_notes,omitempty"`
	Raw           json.RawMessage       `json:"-"`
}

type LiveContinuityMetrics struct {
	SpeechStartCount             *float64 `json:"speech_start_count"`
	SpeechStopCount              *float64 `json:"speech_stop_count"`
	OutputEventCount             *float64 `json:"output_event_count"`
	EvidenceStatus               string   `json:"evidence_status"`
	MedianFirstAudioMS           *float64 `json:"median_first_audio_ms"`
	P90FirstAudioMS              *float64 `json:"p90_first_audio_ms"`
	MaxOutputGapMS               *float64 `json:"max_output_gap_ms"`
	MedianOutputGapMS            *float64 `json:"median_output_gap_ms"`
	SpeechNoOutputCount          *float64 `json:"speech_no_output_count"`
	SpeechNoOutputRatio          *float64 `json:"speech_no_output_ratio"`
	SpeechStartDuringOutputCount *float64 `json:"speech_start_during_output_count"`
}

type LiveContinuityEvidence struct {
	Status                       string   `json:"status"`
	Passed                       bool     `json:"passed"`
	EvidenceStatus               string   `json:"evidence_status"`
	MedianFirstAudioMS           *float64 `json:"median_first_audio_ms,omitempty"`
	P90FirstAudioMS              *float64 `json:"p90_first_audio_ms,omitempty"`
	MaxOutputGapMS               *float64 `json:"max_output_gap_ms,omitempty"`
	MedianOutputGapMS            *float64 `json:"median_output_gap_ms,omitempty"`
	SpeechNoOutputRatio          *float64 `json:"speech_no_output_ratio,omitempty"`
	SpeechStartDuringOutputCount *float64 `json:"speech_start_during_output_count,omitempty"`
	Caveats                      []string `json:"caveats,omitempty"`
	AgentNotes                   []string `json:"agentclash_notes,omitempty"`
}

func LoadLiveContinuityReport(path string) (LiveContinuityReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LiveContinuityReport{}, err
	}
	return IngestLiveContinuityReport(data)
}

func IngestLiveContinuityReport(data []byte) (LiveContinuityReport, error) {
	var report LiveContinuityReport
	if err := json.Unmarshal(data, &report); err != nil {
		return LiveContinuityReport{}, fmt.Errorf("decode live continuity report: %w", err)
	}
	report.Raw = append(json.RawMessage(nil), data...)
	if err := report.Validate(); err != nil {
		return LiveContinuityReport{}, err
	}
	return report, nil
}

func (r *LiveContinuityReport) Validate() error {
	if strings.TrimSpace(r.SchemaVersion) == "" {
		return errors.New("schema_version is required")
	}
	r.Type = normalizeReportType(r.Type)
	if !isAcceptedReportType(r.Type, LiveContinuityReportType, VoiceyLiveContinuityReportType) {
		return fmt.Errorf("type must be %q or %q", LiveContinuityReportType, VoiceyLiveContinuityReportType)
	}
	switch r.Status {
	case "passed", "warn", "failed", "degraded":
	default:
		return errors.New("status must be one of passed, warn, failed, degraded")
	}
	if r.Passed != (r.Status == "passed") {
		return errors.New("passed must be true only when status is passed")
	}
	if strings.TrimSpace(r.Metrics.EvidenceStatus) == "" {
		return errors.New("metrics.evidence_status is required")
	}
	switch r.Metrics.EvidenceStatus {
	case "available", "degraded":
	default:
		return errors.New("metrics.evidence_status must be available or degraded")
	}
	if r.Status == "passed" && r.Metrics.EvidenceStatus == "degraded" {
		return errors.New("status cannot be passed when metrics.evidence_status is degraded")
	}
	for _, metric := range []struct {
		name  string
		value *float64
		min   float64
		max   float64
		count bool
	}{
		{name: "metrics.speech_no_output_ratio", value: r.Metrics.SpeechNoOutputRatio, min: 0, max: 1},
		{name: "metrics.speech_start_count", value: r.Metrics.SpeechStartCount, min: 0, max: math.Inf(1), count: true},
		{name: "metrics.speech_stop_count", value: r.Metrics.SpeechStopCount, min: 0, max: math.Inf(1), count: true},
		{name: "metrics.output_event_count", value: r.Metrics.OutputEventCount, min: 0, max: math.Inf(1), count: true},
		{name: "metrics.max_output_gap_ms", value: r.Metrics.MaxOutputGapMS, min: 0, max: math.Inf(1)},
		{name: "metrics.median_output_gap_ms", value: r.Metrics.MedianOutputGapMS, min: 0, max: math.Inf(1)},
		{name: "metrics.median_first_audio_ms", value: r.Metrics.MedianFirstAudioMS, min: 0, max: math.Inf(1)},
		{name: "metrics.p90_first_audio_ms", value: r.Metrics.P90FirstAudioMS, min: 0, max: math.Inf(1)},
		{name: "metrics.speech_no_output_count", value: r.Metrics.SpeechNoOutputCount, min: 0, max: math.Inf(1), count: true},
		{name: "metrics.speech_start_during_output_count", value: r.Metrics.SpeechStartDuringOutputCount, min: 0, max: math.Inf(1), count: true},
	} {
		if metric.value != nil && (*metric.value < metric.min || *metric.value > metric.max) {
			return fmt.Errorf("%s must be between %v and %v", metric.name, metric.min, metric.max)
		}
		if metric.value != nil && metric.count && math.Trunc(*metric.value) != *metric.value {
			return fmt.Errorf("%s must be a whole number", metric.name)
		}
	}
	return nil
}

func (r LiveContinuityReport) TimingEvidence() LiveContinuityEvidence {
	return LiveContinuityEvidence{
		Status:                       r.Status,
		Passed:                       r.Passed,
		EvidenceStatus:               r.Metrics.EvidenceStatus,
		MedianFirstAudioMS:           cloneFloat64(r.Metrics.MedianFirstAudioMS),
		P90FirstAudioMS:              cloneFloat64(r.Metrics.P90FirstAudioMS),
		MaxOutputGapMS:               cloneFloat64(r.Metrics.MaxOutputGapMS),
		MedianOutputGapMS:            cloneFloat64(r.Metrics.MedianOutputGapMS),
		SpeechNoOutputRatio:          cloneFloat64(r.Metrics.SpeechNoOutputRatio),
		SpeechStartDuringOutputCount: cloneFloat64(r.Metrics.SpeechStartDuringOutputCount),
		Caveats:                      append([]string(nil), r.Caveats...),
		AgentNotes:                   append([]string(nil), r.AgentNotes...),
	}
}
