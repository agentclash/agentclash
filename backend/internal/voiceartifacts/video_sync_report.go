package voiceartifacts

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
)

type VideoSyncReport struct {
	Inputs             map[string]any     `json:"inputs,omitempty"`
	Summary            VideoSyncSummary   `json:"summary"`
	SourceSegments     []VideoSyncSegment `json:"source_segments,omitempty"`
	TranslatedSegments []VideoSyncSegment `json:"translated_segments,omitempty"`
	Pairs              []VideoSyncPair    `json:"pairs,omitempty"`
	VideoSyncNote      string             `json:"video_sync_note,omitempty"`
	VoiceyLogMetrics   map[string]any     `json:"voicey_log_metrics,omitempty"`
	MouthMotionMetrics map[string]any     `json:"mouth_motion_metrics,omitempty"`
	Raw                json.RawMessage    `json:"-"`
}

type VideoSyncSummary struct {
	Status                     string    `json:"status"`
	StatusReasons              []string  `json:"status_reasons,omitempty"`
	StatusBasisMS              *float64  `json:"status_basis_ms"`
	FirstOnsetLagMS            *float64  `json:"first_onset_lag_ms"`
	MedianOnsetLagMS           *float64  `json:"median_onset_lag_ms"`
	P90OnsetLagMS              *float64  `json:"p90_onset_lag_ms"`
	TailLagMS                  *float64  `json:"tail_lag_ms"`
	PairedSegments             *float64  `json:"paired_segments"`
	MissingTranslationSegments *float64  `json:"missing_translation_segments"`
	ExtraTranslationSegments   *float64  `json:"extra_translation_segments"`
	SegmentCoverageRatio       *float64  `json:"segment_coverage_ratio"`
	DurationFitScore           *float64  `json:"duration_fit_score"`
	MedianDurationRatio        *float64  `json:"median_duration_ratio"`
	MinDurationRatio           *float64  `json:"min_duration_ratio"`
	MaxDurationRatio           *float64  `json:"max_duration_ratio"`
	SourceSpanMS               *float64  `json:"source_span_ms"`
	TranslatedSpanMS           *float64  `json:"translated_span_ms"`
	SpanDurationRatio          *float64  `json:"span_duration_ratio"`
	WarnThresholdMS            *float64  `json:"warn_threshold_ms"`
	FailThresholdMS            *float64  `json:"fail_threshold_ms"`
	WarnDurationRatioRange     []float64 `json:"warn_duration_ratio_range,omitempty"`
	FailDurationRatioRange     []float64 `json:"fail_duration_ratio_range,omitempty"`
	Interpretation             string    `json:"interpretation,omitempty"`
}

type VideoSyncSegment struct {
	StartMS float64 `json:"start_ms"`
	EndMS   float64 `json:"end_ms"`
}

type VideoSyncPair struct {
	SourceIndex       *float64 `json:"source_index"`
	TranslatedIndex   *float64 `json:"translated_index"`
	SourceStartMS     *float64 `json:"source_start_ms"`
	SourceEndMS       *float64 `json:"source_end_ms"`
	TranslatedStartMS *float64 `json:"translated_start_ms"`
	TranslatedEndMS   *float64 `json:"translated_end_ms"`
	OnsetLagMS        *float64 `json:"onset_lag_ms"`
	DurationRatio     *float64 `json:"duration_ratio"`
	Status            string   `json:"status"`
}

type VideoSyncEvidence struct {
	Status                     string   `json:"status"`
	StatusReasons              []string `json:"status_reasons,omitempty"`
	MedianOnsetLagMS           *float64 `json:"median_onset_lag_ms,omitempty"`
	P90OnsetLagMS              *float64 `json:"p90_onset_lag_ms,omitempty"`
	TailLagMS                  *float64 `json:"tail_lag_ms,omitempty"`
	SegmentCoverageRatio       *float64 `json:"segment_coverage_ratio,omitempty"`
	DurationFitScore           *float64 `json:"duration_fit_score,omitempty"`
	MissingTranslationSegments *float64 `json:"missing_translation_segments,omitempty"`
	ExtraTranslationSegments   *float64 `json:"extra_translation_segments,omitempty"`
	MedianDurationRatio        *float64 `json:"median_duration_ratio,omitempty"`
	MaxDurationRatio           *float64 `json:"max_duration_ratio,omitempty"`
	Interpretation             string   `json:"interpretation,omitempty"`
}

func LoadVideoSyncReport(path string) (VideoSyncReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return VideoSyncReport{}, err
	}
	return IngestVideoSyncReport(data)
}

func IngestVideoSyncReport(data []byte) (VideoSyncReport, error) {
	var report VideoSyncReport
	if err := json.Unmarshal(data, &report); err != nil {
		return VideoSyncReport{}, fmt.Errorf("decode video sync report: %w", err)
	}
	report.Raw = append(json.RawMessage(nil), data...)
	if err := report.Validate(); err != nil {
		return VideoSyncReport{}, err
	}
	return report, nil
}

func (r VideoSyncReport) Validate() error {
	switch r.Summary.Status {
	case "pass", "warn", "fail":
	default:
		return errors.New("summary.status must be one of pass, warn, fail")
	}
	if strings.TrimSpace(r.Summary.Interpretation) == "" {
		return errors.New("summary.interpretation is required")
	}
	for _, metric := range []struct {
		name  string
		value *float64
		min   float64
		max   float64
		count bool
	}{
		{name: "summary.status_basis_ms", value: r.Summary.StatusBasisMS, min: 0, max: math.Inf(1)},
		{name: "summary.first_onset_lag_ms", value: r.Summary.FirstOnsetLagMS, min: math.Inf(-1), max: math.Inf(1)},
		{name: "summary.median_onset_lag_ms", value: r.Summary.MedianOnsetLagMS, min: math.Inf(-1), max: math.Inf(1)},
		{name: "summary.p90_onset_lag_ms", value: r.Summary.P90OnsetLagMS, min: math.Inf(-1), max: math.Inf(1)},
		{name: "summary.tail_lag_ms", value: r.Summary.TailLagMS, min: math.Inf(-1), max: math.Inf(1)},
		{name: "summary.paired_segments", value: r.Summary.PairedSegments, min: 0, max: math.Inf(1), count: true},
		{name: "summary.missing_translation_segments", value: r.Summary.MissingTranslationSegments, min: 0, max: math.Inf(1), count: true},
		{name: "summary.extra_translation_segments", value: r.Summary.ExtraTranslationSegments, min: 0, max: math.Inf(1), count: true},
		{name: "summary.segment_coverage_ratio", value: r.Summary.SegmentCoverageRatio, min: 0, max: 1},
		{name: "summary.duration_fit_score", value: r.Summary.DurationFitScore, min: 0, max: 1},
		{name: "summary.median_duration_ratio", value: r.Summary.MedianDurationRatio, min: 0, max: math.Inf(1)},
		{name: "summary.min_duration_ratio", value: r.Summary.MinDurationRatio, min: 0, max: math.Inf(1)},
		{name: "summary.max_duration_ratio", value: r.Summary.MaxDurationRatio, min: 0, max: math.Inf(1)},
		{name: "summary.source_span_ms", value: r.Summary.SourceSpanMS, min: 0, max: math.Inf(1)},
		{name: "summary.translated_span_ms", value: r.Summary.TranslatedSpanMS, min: 0, max: math.Inf(1)},
		{name: "summary.span_duration_ratio", value: r.Summary.SpanDurationRatio, min: 0, max: math.Inf(1)},
		{name: "summary.warn_threshold_ms", value: r.Summary.WarnThresholdMS, min: 0, max: math.Inf(1)},
		{name: "summary.fail_threshold_ms", value: r.Summary.FailThresholdMS, min: 0, max: math.Inf(1)},
	} {
		if err := validateVideoSyncMetric(metric.name, metric.value, metric.min, metric.max, metric.count); err != nil {
			return err
		}
	}
	if err := validateRange("summary.warn_duration_ratio_range", r.Summary.WarnDurationRatioRange); err != nil {
		return err
	}
	if err := validateRange("summary.fail_duration_ratio_range", r.Summary.FailDurationRatioRange); err != nil {
		return err
	}
	for index, segment := range r.SourceSegments {
		if err := validateSegment(fmt.Sprintf("source_segments[%d]", index), segment); err != nil {
			return err
		}
	}
	for index, segment := range r.TranslatedSegments {
		if err := validateSegment(fmt.Sprintf("translated_segments[%d]", index), segment); err != nil {
			return err
		}
	}
	for index, pair := range r.Pairs {
		if err := validatePair(fmt.Sprintf("pairs[%d]", index), pair, len(r.SourceSegments), len(r.TranslatedSegments)); err != nil {
			return err
		}
	}
	if err := r.validateSummaryAgainstPairs(); err != nil {
		return err
	}
	return nil
}

func (r VideoSyncReport) validateSummaryAgainstPairs() error {
	if len(r.Pairs) == 0 {
		return nil
	}
	paired := 0
	missing := 0
	pairedTranslated := make(map[int]struct{})
	for _, pair := range r.Pairs {
		switch pair.Status {
		case "paired":
			paired++
			if pair.TranslatedIndex != nil {
				pairedTranslated[int(*pair.TranslatedIndex)] = struct{}{}
			}
		case "missing_translation":
			missing++
		}
	}
	if err := requireCountMatch("summary.paired_segments", r.Summary.PairedSegments, paired); err != nil {
		return err
	}
	if err := requireCountMatch("summary.missing_translation_segments", r.Summary.MissingTranslationSegments, missing); err != nil {
		return err
	}
	extra := len(r.TranslatedSegments) - len(pairedTranslated)
	if err := requireCountMatch("summary.extra_translation_segments", r.Summary.ExtraTranslationSegments, extra); err != nil {
		return err
	}
	if len(r.SourceSegments) > 0 && r.Summary.SegmentCoverageRatio != nil {
		expected := float64(paired) / float64(len(r.SourceSegments))
		if math.Abs(*r.Summary.SegmentCoverageRatio-expected) > 0.001 {
			return fmt.Errorf("summary.segment_coverage_ratio must match paired/source segment counts")
		}
	}
	return nil
}

func (r VideoSyncReport) TimingEvidence() VideoSyncEvidence {
	return VideoSyncEvidence{
		Status:                     r.Summary.Status,
		StatusReasons:              append([]string(nil), r.Summary.StatusReasons...),
		MedianOnsetLagMS:           cloneFloat64(r.Summary.MedianOnsetLagMS),
		P90OnsetLagMS:              cloneFloat64(r.Summary.P90OnsetLagMS),
		TailLagMS:                  cloneFloat64(r.Summary.TailLagMS),
		SegmentCoverageRatio:       cloneFloat64(r.Summary.SegmentCoverageRatio),
		DurationFitScore:           cloneFloat64(r.Summary.DurationFitScore),
		MissingTranslationSegments: cloneFloat64(r.Summary.MissingTranslationSegments),
		ExtraTranslationSegments:   cloneFloat64(r.Summary.ExtraTranslationSegments),
		MedianDurationRatio:        cloneFloat64(r.Summary.MedianDurationRatio),
		MaxDurationRatio:           cloneFloat64(r.Summary.MaxDurationRatio),
		Interpretation:             r.Summary.Interpretation,
	}
}

func validateVideoSyncMetric(name string, value *float64, min float64, max float64, count bool) error {
	if value == nil {
		return nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) {
		return fmt.Errorf("%s must be finite", name)
	}
	if *value < min || *value > max {
		return fmt.Errorf("%s must be between %v and %v", name, min, max)
	}
	if count && math.Trunc(*value) != *value {
		return fmt.Errorf("%s must be a whole number", name)
	}
	return nil
}

func validateRange(name string, values []float64) error {
	if len(values) == 0 {
		return nil
	}
	if len(values) != 2 {
		return fmt.Errorf("%s must contain exactly two values", name)
	}
	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return fmt.Errorf("%s[%d] must be a finite non-negative number", name, index)
		}
	}
	if values[0] > values[1] {
		return fmt.Errorf("%s lower bound must be <= upper bound", name)
	}
	return nil
}

func validateSegment(name string, segment VideoSyncSegment) error {
	if math.IsNaN(segment.StartMS) || math.IsInf(segment.StartMS, 0) || segment.StartMS < 0 {
		return fmt.Errorf("%s.start_ms must be a finite non-negative number", name)
	}
	if math.IsNaN(segment.EndMS) || math.IsInf(segment.EndMS, 0) || segment.EndMS < 0 {
		return fmt.Errorf("%s.end_ms must be a finite non-negative number", name)
	}
	if segment.EndMS < segment.StartMS {
		return fmt.Errorf("%s.end_ms must be >= start_ms", name)
	}
	return nil
}

func validatePair(name string, pair VideoSyncPair, sourceCount int, translatedCount int) error {
	switch pair.Status {
	case "paired":
		for _, required := range []struct {
			field string
			value *float64
		}{
			{field: "source_index", value: pair.SourceIndex},
			{field: "translated_index", value: pair.TranslatedIndex},
			{field: "source_start_ms", value: pair.SourceStartMS},
			{field: "source_end_ms", value: pair.SourceEndMS},
			{field: "translated_start_ms", value: pair.TranslatedStartMS},
			{field: "translated_end_ms", value: pair.TranslatedEndMS},
			{field: "onset_lag_ms", value: pair.OnsetLagMS},
			{field: "duration_ratio", value: pair.DurationRatio},
		} {
			if required.value == nil {
				return fmt.Errorf("%s.%s is required for paired rows", name, required.field)
			}
		}
	case "missing_translation":
		if pair.SourceIndex == nil {
			return fmt.Errorf("%s.source_index is required for missing_translation rows", name)
		}
	default:
		return fmt.Errorf("%s.status must be paired or missing_translation", name)
	}
	if err := validateIndex(name+".source_index", pair.SourceIndex, sourceCount); err != nil {
		return err
	}
	if err := validateIndex(name+".translated_index", pair.TranslatedIndex, translatedCount); err != nil {
		return err
	}
	for _, metric := range []struct {
		name  string
		value *float64
		min   float64
		max   float64
	}{
		{name: name + ".source_start_ms", value: pair.SourceStartMS, min: 0, max: math.Inf(1)},
		{name: name + ".source_end_ms", value: pair.SourceEndMS, min: 0, max: math.Inf(1)},
		{name: name + ".translated_start_ms", value: pair.TranslatedStartMS, min: 0, max: math.Inf(1)},
		{name: name + ".translated_end_ms", value: pair.TranslatedEndMS, min: 0, max: math.Inf(1)},
		{name: name + ".onset_lag_ms", value: pair.OnsetLagMS, min: math.Inf(-1), max: math.Inf(1)},
		{name: name + ".duration_ratio", value: pair.DurationRatio, min: 0, max: math.Inf(1)},
	} {
		if err := validateVideoSyncMetric(metric.name, metric.value, metric.min, metric.max, false); err != nil {
			return err
		}
	}
	if pair.SourceStartMS != nil && pair.SourceEndMS != nil && *pair.SourceEndMS < *pair.SourceStartMS {
		return fmt.Errorf("%s.source_end_ms must be >= source_start_ms", name)
	}
	if pair.TranslatedStartMS != nil && pair.TranslatedEndMS != nil && *pair.TranslatedEndMS < *pair.TranslatedStartMS {
		return fmt.Errorf("%s.translated_end_ms must be >= translated_start_ms", name)
	}
	return nil
}

func validateIndex(name string, value *float64, count int) error {
	if value == nil {
		return nil
	}
	if err := validateVideoSyncMetric(name, value, 0, math.Inf(1), true); err != nil {
		return err
	}
	if count == 0 || int(*value) >= count {
		return fmt.Errorf("%s is out of range", name)
	}
	return nil
}

func requireCountMatch(name string, value *float64, expected int) error {
	if value == nil {
		return nil
	}
	if int(*value) != expected {
		return fmt.Errorf("%s must match pair rows: got %d, want %d", name, int(*value), expected)
	}
	return nil
}
