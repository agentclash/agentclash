package voiceimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
	"github.com/agentclash/agentclash/backend/internal/multimodaltrace"
	"github.com/agentclash/agentclash/backend/internal/voiceartifacts"
	"github.com/google/uuid"
)

const SchemaVersionV1 = "2026-05-13"

var (
	ErrInvalidSchemaVersion   = errors.New("invalid voice call import schema version")
	ErrInvalidRedactionStatus = errors.New("invalid voice call import redaction status")
	ErrMissingRedaction       = errors.New("voice call import redaction metadata is required")
	ErrPromotionNotApproved   = errors.New("voice call import is not approved for regression")
)

type RedactionStatus string

const (
	RedactionStatusUnreviewed            RedactionStatus = "unreviewed"
	RedactionStatusRedacted              RedactionStatus = "redacted"
	RedactionStatusApprovedForRegression RedactionStatus = "approved_for_regression"
	RedactionStatusRejected              RedactionStatus = "rejected"
)

type Fixture struct {
	SchemaVersion    string                  `json:"schema_version"`
	ImportID         string                  `json:"import_id"`
	TraceID          string                  `json:"trace_id"`
	RunID            uuid.UUID               `json:"run_id"`
	RunAgentID       uuid.UUID               `json:"run_agent_id"`
	VoiceSessionID   string                  `json:"voice_session_id"`
	Source           SourceMetadata          `json:"source"`
	Transcript       []TranscriptEntry       `json:"transcript"`
	ProviderEvents   []ProviderEventFragment `json:"provider_events"`
	ArtifactManifest voiceartifacts.Manifest `json:"artifact_manifest"`
	Redaction        *RedactionMetadata      `json:"redaction"`
	ReviewerLabels   []ReviewerLabel         `json:"reviewer_labels,omitempty"`
	ExpectedOutcome  string                  `json:"expected_outcome"`
	FailureCategory  string                  `json:"failure_category"`
	PromotionTarget  PromotionTarget         `json:"promotion_target"`
}

type SourceMetadata struct {
	Provider   string    `json:"provider"`
	CallID     string    `json:"call_id"`
	CapturedAt time.Time `json:"captured_at"`
}

type TranscriptEntry struct {
	SegmentID  string          `json:"segment_id"`
	TurnID     string          `json:"turn_id,omitempty"`
	Speaker    string          `json:"speaker"`
	Text       string          `json:"text"`
	Language   string          `json:"language,omitempty"`
	Confidence *float64        `json:"confidence,omitempty"`
	Final      bool            `json:"final"`
	OccurredAt time.Time       `json:"occurred_at"`
	Audio      *AudioReference `json:"audio,omitempty"`
}

type AudioReference struct {
	SegmentID   string `json:"segment_id"`
	ArtifactKey string `json:"artifact_key"`
	ArtifactRef string `json:"artifact_ref"`
	Format      string `json:"format"`
	Channel     string `json:"channel,omitempty"`
	DurationMS  int64  `json:"duration_ms,omitempty"`
}

type ProviderEventFragment struct {
	EventID    string          `json:"event_id"`
	Type       string          `json:"type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

type RedactionMetadata struct {
	Status     RedactionStatus    `json:"status"`
	ReviewedBy string             `json:"reviewed_by,omitempty"`
	ReviewedAt time.Time          `json:"reviewed_at,omitempty"`
	Findings   []RedactionFinding `json:"findings"`
	Notes      string             `json:"notes,omitempty"`
}

type RedactionFinding struct {
	Class  string `json:"class"`
	Count  int    `json:"count"`
	Action string `json:"action"`
}

type ReviewerLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PromotionTarget struct {
	ChallengePackSlug string `json:"challenge_pack_slug"`
	InputSetKey       string `json:"input_set_key"`
	ChallengeKey      string `json:"challenge_key"`
	CaseKey           string `json:"case_key"`
}

func Decode(data []byte) (Fixture, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var fixture Fixture
	if err := decoder.Decode(&fixture); err != nil {
		return Fixture{}, fmt.Errorf("decode voice call import fixture: %w", err)
	}
	if err := fixture.Validate(); err != nil {
		return Fixture{}, err
	}
	return fixture, nil
}

func (f Fixture) Validate() error {
	if f.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("%w: %q", ErrInvalidSchemaVersion, f.SchemaVersion)
	}
	if strings.TrimSpace(f.ImportID) == "" {
		return errors.New("import_id is required")
	}
	if strings.TrimSpace(f.TraceID) == "" {
		return errors.New("trace_id is required")
	}
	if f.RunID == uuid.Nil {
		return errors.New("run_id is required")
	}
	if f.RunAgentID == uuid.Nil {
		return errors.New("run_agent_id is required")
	}
	if strings.TrimSpace(f.VoiceSessionID) == "" {
		return errors.New("voice_session_id is required")
	}
	if err := f.Source.Validate(); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if len(f.Transcript) == 0 {
		return errors.New("transcript must contain at least one entry")
	}
	artifactKeys, err := f.validateArtifactManifest()
	if err != nil {
		return err
	}
	for idx, entry := range f.Transcript {
		if err := entry.Validate(artifactKeys); err != nil {
			return fmt.Errorf("transcript[%d]: %w", idx, err)
		}
	}
	if len(f.ProviderEvents) == 0 {
		return errors.New("provider_events must contain at least one fragment")
	}
	for idx, event := range f.ProviderEvents {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("provider_events[%d]: %w", idx, err)
		}
	}
	if f.Redaction == nil {
		return ErrMissingRedaction
	}
	if err := f.Redaction.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(f.ExpectedOutcome) == "" {
		return errors.New("expected_outcome is required")
	}
	if strings.TrimSpace(f.FailureCategory) == "" {
		return errors.New("failure_category is required")
	}
	if err := f.PromotionTarget.Validate(); err != nil {
		return err
	}
	for idx, label := range f.ReviewerLabels {
		if err := label.Validate(); err != nil {
			return fmt.Errorf("reviewer_labels[%d]: %w", idx, err)
		}
	}
	return nil
}

func (f Fixture) ToTrace() (multimodaltrace.Trace, error) {
	if err := f.Validate(); err != nil {
		return multimodaltrace.Trace{}, err
	}
	trace := multimodaltrace.Trace{
		TraceID:       strings.TrimSpace(f.TraceID),
		SchemaVersion: multimodaltrace.SchemaVersionV1,
		RunID:         f.RunID,
		RunAgentID:    f.RunAgentID,
	}
	var sequence int64
	for _, entry := range f.Transcript {
		var sourceSegmentID string
		if entry.Audio != nil {
			sequence++
			sourceSegmentID = strings.TrimSpace(entry.Audio.SegmentID)
			trace.Segments = append(trace.Segments, multimodaltrace.Segment{
				SegmentID:      sourceSegmentID,
				SequenceNumber: sequence,
				Kind:           audioKindForSpeaker(entry.Speaker),
				Actor:          actorForSpeaker(entry.Speaker),
				OccurredAt:     entry.OccurredAt.UTC(),
				Audio: &multimodaltrace.AudioPayload{
					ArtifactRef: strings.TrimSpace(entry.Audio.ArtifactRef),
					Format:      strings.TrimSpace(entry.Audio.Format),
					Channel:     strings.TrimSpace(entry.Audio.Channel),
					DurationMS:  entry.Audio.DurationMS,
				},
			})
		}
		sequence++
		kind := multimodaltrace.SegmentKindTranscriptPartial
		if entry.Final {
			kind = multimodaltrace.SegmentKindTranscriptFinal
		}
		trace.Segments = append(trace.Segments, multimodaltrace.Segment{
			SegmentID:      strings.TrimSpace(entry.SegmentID),
			SequenceNumber: sequence,
			Kind:           kind,
			Actor:          actorForSpeaker(entry.Speaker),
			OccurredAt:     entry.OccurredAt.UTC(),
			Transcript: &multimodaltrace.TranscriptPayload{
				Text:            entry.Text,
				Language:        strings.TrimSpace(entry.Language),
				Confidence:      entry.Confidence,
				SourceSegmentID: sourceSegmentID,
			},
		})
	}
	if err := trace.Validate(); err != nil {
		return multimodaltrace.Trace{}, err
	}
	return trace, nil
}

func (f Fixture) ToArtifactManifest() (voiceartifacts.Manifest, error) {
	if err := f.Validate(); err != nil {
		return voiceartifacts.Manifest{}, err
	}
	manifest := f.ArtifactManifest
	manifest.Artifacts = append([]voiceartifacts.ArtifactRef(nil), f.ArtifactManifest.Artifacts...)
	return manifest, nil
}

func (f Fixture) PromoteToChallengeCase() (challengepack.CaseDefinition, error) {
	if err := f.Validate(); err != nil {
		return challengepack.CaseDefinition{}, err
	}
	if f.Redaction.Status != RedactionStatusApprovedForRegression {
		return challengepack.CaseDefinition{}, fmt.Errorf("%w: status %q", ErrPromotionNotApproved, f.Redaction.Status)
	}
	trace, err := f.ToTrace()
	if err != nil {
		return challengepack.CaseDefinition{}, err
	}
	manifest, err := f.ToArtifactManifest()
	if err != nil {
		return challengepack.CaseDefinition{}, err
	}
	labels := append([]ReviewerLabel(nil), f.ReviewerLabels...)
	sort.SliceStable(labels, func(i, j int) bool {
		if labels[i].Key == labels[j].Key {
			return labels[i].Value < labels[j].Value
		}
		return labels[i].Key < labels[j].Key
	})

	return challengepack.CaseDefinition{
		ChallengeKey: strings.TrimSpace(f.PromotionTarget.ChallengeKey),
		CaseKey:      strings.TrimSpace(f.PromotionTarget.CaseKey),
		ItemKey:      strings.TrimSpace(f.PromotionTarget.CaseKey),
		Payload: map[string]any{
			"source_import_id":    strings.TrimSpace(f.ImportID),
			"source_provider":     strings.TrimSpace(f.Source.Provider),
			"source_call_id":      strings.TrimSpace(f.Source.CallID),
			"voice_session_id":    strings.TrimSpace(f.VoiceSessionID),
			"redaction_status":    string(f.Redaction.Status),
			"failure_category":    strings.TrimSpace(f.FailureCategory),
			"promotion_pack_slug": strings.TrimSpace(f.PromotionTarget.ChallengePackSlug),
			"promotion_input_set": strings.TrimSpace(f.PromotionTarget.InputSetKey),
			"reviewer_labels":     labels,
		},
		Inputs: []challengepack.CaseInput{
			{Key: "multimodal_trace", Kind: "multimodal_trace", Value: trace},
			{Key: "voice_artifact_manifest", Kind: "voice_artifact_manifest", Value: manifest},
		},
		Expectations: []challengepack.CaseExpectation{
			{Key: "expected_outcome", Kind: "exact", Value: strings.TrimSpace(f.ExpectedOutcome)},
			{Key: "failure_category", Kind: "classification", Value: strings.TrimSpace(f.FailureCategory)},
		},
		Artifacts: challengeArtifactRefs(manifest.Artifacts),
	}, nil
}

func (f Fixture) validateArtifactManifest() (map[string]voiceartifacts.ArtifactRef, error) {
	if err := f.ArtifactManifest.Validate(); err != nil {
		return nil, fmt.Errorf("artifact_manifest: %w", err)
	}
	if f.ArtifactManifest.RunID != f.RunID {
		return nil, errors.New("artifact_manifest.run_id must match run_id")
	}
	if f.ArtifactManifest.RunAgentID != f.RunAgentID {
		return nil, errors.New("artifact_manifest.run_agent_id must match run_agent_id")
	}
	if f.ArtifactManifest.VoiceSessionID != f.VoiceSessionID {
		return nil, errors.New("artifact_manifest.voice_session_id must match voice_session_id")
	}
	artifactKeys := make(map[string]voiceartifacts.ArtifactRef, len(f.ArtifactManifest.Artifacts))
	kinds := make(map[voiceartifacts.ArtifactKind]bool, len(f.ArtifactManifest.Artifacts))
	for _, artifact := range f.ArtifactManifest.Artifacts {
		artifactKeys[artifact.Key] = artifact
		kinds[artifact.Kind] = true
	}
	if !kinds[voiceartifacts.ArtifactKindRawProviderTrace] {
		return nil, errors.New("artifact_manifest must include raw_provider_trace_json artifact")
	}
	if !kinds[voiceartifacts.ArtifactKindRedactionMetadata] {
		return nil, errors.New("artifact_manifest must include redaction_metadata_json artifact")
	}
	return artifactKeys, nil
}

func (s SourceMetadata) Validate() error {
	if strings.TrimSpace(s.Provider) == "" {
		return errors.New("provider is required")
	}
	if strings.TrimSpace(s.CallID) == "" {
		return errors.New("call_id is required")
	}
	if s.CapturedAt.IsZero() {
		return errors.New("captured_at is required")
	}
	return nil
}

func (e TranscriptEntry) Validate(artifactKeys map[string]voiceartifacts.ArtifactRef) error {
	if strings.TrimSpace(e.SegmentID) == "" {
		return errors.New("segment_id is required")
	}
	if actorForSpeaker(e.Speaker) == "" {
		return fmt.Errorf("speaker must be %q or %q", multimodaltrace.ActorUser, multimodaltrace.ActorAgent)
	}
	if strings.TrimSpace(e.Text) == "" {
		return errors.New("text is required")
	}
	if e.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	if e.Confidence != nil && (*e.Confidence < 0 || *e.Confidence > 1) {
		return errors.New("confidence must be between 0 and 1")
	}
	if e.Audio != nil {
		if err := e.Audio.Validate(artifactKeys); err != nil {
			return fmt.Errorf("audio: %w", err)
		}
	}
	return nil
}

func (a AudioReference) Validate(artifactKeys map[string]voiceartifacts.ArtifactRef) error {
	if strings.TrimSpace(a.SegmentID) == "" {
		return errors.New("segment_id is required")
	}
	if strings.TrimSpace(a.ArtifactKey) == "" {
		return errors.New("artifact_key is required")
	}
	if _, ok := artifactKeys[strings.TrimSpace(a.ArtifactKey)]; !ok {
		return fmt.Errorf("artifact_key %q does not reference artifact_manifest", a.ArtifactKey)
	}
	if strings.TrimSpace(a.ArtifactRef) == "" {
		return errors.New("artifact_ref is required")
	}
	if strings.TrimSpace(a.Format) == "" {
		return errors.New("format is required")
	}
	if a.DurationMS < 0 {
		return errors.New("duration_ms must be non-negative")
	}
	return nil
}

func (e ProviderEventFragment) Validate() error {
	if strings.TrimSpace(e.EventID) == "" {
		return errors.New("event_id is required")
	}
	if strings.TrimSpace(e.Type) == "" {
		return errors.New("type is required")
	}
	if e.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	if len(e.Payload) == 0 {
		return errors.New("payload is required")
	}
	if !json.Valid(e.Payload) {
		return errors.New("payload must be valid JSON")
	}
	return nil
}

func (m RedactionMetadata) Validate() error {
	if !m.Status.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidRedactionStatus, m.Status)
	}
	if len(m.Findings) == 0 {
		return errors.New("redaction.findings must be explicit")
	}
	for idx, finding := range m.Findings {
		if err := finding.Validate(); err != nil {
			return fmt.Errorf("redaction.findings[%d]: %w", idx, err)
		}
	}
	if m.Status == RedactionStatusRedacted || m.Status == RedactionStatusApprovedForRegression || m.Status == RedactionStatusRejected {
		if strings.TrimSpace(m.ReviewedBy) == "" {
			return errors.New("redaction.reviewed_by is required for reviewed statuses")
		}
		if m.ReviewedAt.IsZero() {
			return errors.New("redaction.reviewed_at is required for reviewed statuses")
		}
	}
	return nil
}

func (f RedactionFinding) Validate() error {
	if strings.TrimSpace(f.Class) == "" {
		return errors.New("class is required")
	}
	if f.Count < 0 {
		return errors.New("count must be non-negative")
	}
	if strings.TrimSpace(f.Action) == "" {
		return errors.New("action is required")
	}
	return nil
}

func (s RedactionStatus) Valid() bool {
	switch s {
	case RedactionStatusUnreviewed,
		RedactionStatusRedacted,
		RedactionStatusApprovedForRegression,
		RedactionStatusRejected:
		return true
	default:
		return false
	}
}

func (l ReviewerLabel) Validate() error {
	if strings.TrimSpace(l.Key) == "" {
		return errors.New("key is required")
	}
	if strings.TrimSpace(l.Value) == "" {
		return errors.New("value is required")
	}
	return nil
}

func (t PromotionTarget) Validate() error {
	if strings.TrimSpace(t.ChallengePackSlug) == "" {
		return errors.New("promotion_target.challenge_pack_slug is required")
	}
	if strings.TrimSpace(t.InputSetKey) == "" {
		return errors.New("promotion_target.input_set_key is required")
	}
	if strings.TrimSpace(t.ChallengeKey) == "" {
		return errors.New("promotion_target.challenge_key is required")
	}
	if strings.TrimSpace(t.CaseKey) == "" {
		return errors.New("promotion_target.case_key is required")
	}
	return nil
}

func actorForSpeaker(speaker string) multimodaltrace.Actor {
	switch strings.TrimSpace(speaker) {
	case string(multimodaltrace.ActorUser):
		return multimodaltrace.ActorUser
	case string(multimodaltrace.ActorAgent):
		return multimodaltrace.ActorAgent
	default:
		return ""
	}
}

func audioKindForSpeaker(speaker string) multimodaltrace.SegmentKind {
	if actorForSpeaker(speaker) == multimodaltrace.ActorAgent {
		return multimodaltrace.SegmentKindAudioOutput
	}
	return multimodaltrace.SegmentKindAudioInput
}

func challengeArtifactRefs(artifacts []voiceartifacts.ArtifactRef) []challengepack.ArtifactRef {
	refs := make([]challengepack.ArtifactRef, 0, len(artifacts))
	for _, artifact := range artifacts {
		refs = append(refs, challengepack.ArtifactRef{Key: artifact.Key})
	}
	sort.SliceStable(refs, func(i, j int) bool {
		return refs[i].Key < refs[j].Key
	})
	return refs
}
