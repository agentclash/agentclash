package voicelive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/voiceartifacts"
	"github.com/agentclash/agentclash/runtime/runevents"
	"github.com/google/uuid"
)

const ScriptSchemaVersionV1 = "2026-05-13"

var (
	ErrInvalidSession    = errors.New("invalid live voice session")
	ErrInvalidScript     = errors.New("invalid fake live voice transport script")
	ErrInvalidOperation  = errors.New("invalid live voice transport operation")
	ErrMissingMediaFrame = errors.New("missing live voice media frame")
	ErrSessionClosed     = errors.New("live voice session is closed")
)

type Status string

const (
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Direction string

const (
	DirectionInbound  Direction = "inbound"
	DirectionOutbound Direction = "outbound"
)

type StopReason string

const (
	StopReasonCompleted        StopReason = "completed"
	StopReasonAgentHangup      StopReason = "agent_hangup"
	StopReasonTransportFailure StopReason = "transport_failure"
)

type ControlAction string

const (
	ControlActionBargeIn     ControlAction = "barge_in_detected"
	ControlActionClearBuffer ControlAction = "clear_buffer"
)

type Transport interface {
	StartSession(context.Context, StartSessionInput) (Session, error)
}

type Session interface {
	ID() string
	ReceiveMediaFrame(context.Context, MediaFrame) error
	ReceiveTranscript(context.Context, TranscriptSegment) error
	SendOutboundMediaFrame(context.Context, MediaFrame) error
	SendOutboundArtifact(context.Context, ArtifactOutput) error
	SendMediaControl(context.Context, MediaControl) error
	Stop(context.Context, StopRequest) error
	Fail(context.Context, LifecycleError) error
	Events() []runevents.Envelope
	ArtifactManifest() voiceartifacts.Manifest
}

type StartSessionInput struct {
	RunID          uuid.UUID
	RunAgentID     uuid.UUID
	VoiceSessionID string
	StartedAt      time.Time
	ArtifactBucket string
}

type MediaFrame struct {
	FrameID     string
	TurnID      string
	Direction   Direction
	ArtifactRef string
	Format      string
	ContentType string
	DurationMS  int64
	OccurredAt  time.Time
}

type TranscriptSegment struct {
	SegmentID  string
	TurnID     string
	Text       string
	Language   string
	Confidence *float64
	Final      bool
	OccurredAt time.Time
}

type ArtifactOutput struct {
	ArtifactRef string
	TurnID      string
	Format      string
	ContentType string
	DurationMS  int64
	OccurredAt  time.Time
}

type MediaControl struct {
	Action         ControlAction
	TurnID         string
	TargetArtifact string
	ClearBuffer    bool
	OccurredAt     time.Time
}

type StopRequest struct {
	Reason     StopReason
	Message    string
	OccurredAt time.Time
}

type LifecycleError struct {
	Code       string
	Message    string
	OccurredAt time.Time
}

type Result struct {
	Status        Status
	FailureReason string
	Events        []runevents.Envelope
	Manifest      voiceartifacts.Manifest
}

type FakeTransport struct {
	artifactBucket string
}

var _ Transport = (*FakeTransport)(nil)
var _ Session = (*fakeSession)(nil)

func NewFakeTransport(artifactBucket string) *FakeTransport {
	return &FakeTransport{artifactBucket: strings.TrimSpace(artifactBucket)}
}

func (t *FakeTransport) StartSession(ctx context.Context, input StartSessionInput) (Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.RunID == uuid.Nil {
		return nil, fmt.Errorf("%w: run_id is required", ErrInvalidSession)
	}
	if input.RunAgentID == uuid.Nil {
		return nil, fmt.Errorf("%w: run_agent_id is required", ErrInvalidSession)
	}
	if strings.TrimSpace(input.VoiceSessionID) == "" {
		return nil, fmt.Errorf("%w: voice_session_id is required", ErrInvalidSession)
	}
	if input.StartedAt.IsZero() {
		return nil, fmt.Errorf("%w: started_at is required", ErrInvalidSession)
	}
	bucket := firstNonEmpty(input.ArtifactBucket, t.artifactBucket, "agentclash-fake-voice-artifacts")
	session := &fakeSession{
		runID:          input.RunID,
		runAgentID:     input.RunAgentID,
		voiceSessionID: strings.TrimSpace(input.VoiceSessionID),
		startedAt:      input.StartedAt.UTC(),
		artifactBucket: bucket,
		artifacts:      make(map[voiceartifacts.ArtifactKind]voiceartifacts.ArtifactRef),
	}
	if err := session.appendEvent(runevents.EventTypeMediaSessionStarted, session.startedAt, map[string]any{
		"voice_session_id": session.voiceSessionID,
		"transport":        "fake",
		"mode":             "live-call",
	}, runevents.SummaryMetadata{
		Status:         "running",
		Channel:        "media",
		IdempotencyKey: session.voiceSessionID + ":session-started",
	}); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *fakeSession) ID() string {
	return s.voiceSessionID
}

func (s *fakeSession) ReceiveMediaFrame(ctx context.Context, frame MediaFrame) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	frame.Direction = DirectionInbound
	if err := validateMediaFrame(frame); err != nil {
		return s.failForOperation(frame.OccurredAt, "missing_media_frame", err)
	}
	s.ensureArtifact(voiceartifacts.ArtifactKindCallerAudio)
	return s.appendEvent(runevents.EventTypeMediaFrameReceived, frame.OccurredAt, map[string]any{
		"voice_session_id": s.voiceSessionID,
		"turn_id":          strings.TrimSpace(frame.TurnID),
		"frame_id":         strings.TrimSpace(frame.FrameID),
		"direction":        string(DirectionInbound),
		"artifact_ref":     strings.TrimSpace(frame.ArtifactRef),
		"format":           strings.TrimSpace(frame.Format),
		"duration_ms":      frame.DurationMS,
	}, turnSummary(frame.TurnID, "caller", "inbound_audio"))
}

func (s *fakeSession) ReceiveTranscript(ctx context.Context, segment TranscriptSegment) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(segment.Text) == "" {
		return s.failForOperation(segment.OccurredAt, "invalid_transcript", fmt.Errorf("%w: transcript text is required", ErrInvalidOperation))
	}
	eventType := runevents.EventTypeTranscriptPartial
	if segment.Final {
		eventType = runevents.EventTypeTranscriptFinal
	}
	s.ensureArtifact(voiceartifacts.ArtifactKindTranscriptJSON)
	payload := map[string]any{
		"voice_session_id": s.voiceSessionID,
		"turn_id":          strings.TrimSpace(segment.TurnID),
		"segment_id":       firstNonEmpty(segment.SegmentID, segment.TurnID+":transcript"),
		"text":             segment.Text,
		"language":         strings.TrimSpace(segment.Language),
		"final":            segment.Final,
	}
	if segment.Confidence != nil {
		payload["confidence"] = *segment.Confidence
	}
	return s.appendEvent(eventType, segment.OccurredAt, payload, turnSummary(segment.TurnID, "caller", "transcript"))
}

func (s *fakeSession) SendOutboundMediaFrame(ctx context.Context, frame MediaFrame) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	frame.Direction = DirectionOutbound
	if err := validateMediaFrame(frame); err != nil {
		return s.failForOperation(frame.OccurredAt, "missing_media_frame", err)
	}
	return s.SendOutboundArtifact(ctx, ArtifactOutput{
		ArtifactRef: frame.ArtifactRef,
		TurnID:      frame.TurnID,
		Format:      frame.Format,
		ContentType: frame.ContentType,
		DurationMS:  frame.DurationMS,
		OccurredAt:  frame.OccurredAt,
	})
}

func (s *fakeSession) SendOutboundArtifact(ctx context.Context, artifact ArtifactOutput) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(artifact.ArtifactRef) == "" {
		return s.failForOperation(artifact.OccurredAt, "missing_agent_artifact", fmt.Errorf("%w: artifact_ref is required", ErrMissingMediaFrame))
	}
	if artifact.DurationMS < 0 {
		return s.failForOperation(artifact.OccurredAt, "invalid_agent_artifact", fmt.Errorf("%w: duration_ms must be non-negative", ErrInvalidOperation))
	}
	s.ensureArtifact(voiceartifacts.ArtifactKindAgentAudio)
	payload := map[string]any{
		"voice_session_id": s.voiceSessionID,
		"turn_id":          strings.TrimSpace(artifact.TurnID),
		"artifact_ref":     strings.TrimSpace(artifact.ArtifactRef),
		"format":           strings.TrimSpace(artifact.Format),
	}
	if err := s.appendEvent(runevents.EventTypeAgentAudioStarted, artifact.OccurredAt, payload, turnSummary(artifact.TurnID, "agent", "outbound_audio")); err != nil {
		return err
	}
	completedPayload := map[string]any{
		"voice_session_id": s.voiceSessionID,
		"turn_id":          strings.TrimSpace(artifact.TurnID),
		"artifact_ref":     strings.TrimSpace(artifact.ArtifactRef),
		"format":           strings.TrimSpace(artifact.Format),
		"duration_ms":      artifact.DurationMS,
	}
	completedAt := artifact.OccurredAt.Add(time.Duration(artifact.DurationMS) * time.Millisecond).UTC()
	return s.appendEvent(runevents.EventTypeAgentAudioCompleted, completedAt, completedPayload, turnSummary(artifact.TurnID, "agent", "outbound_audio"))
}

func (s *fakeSession) SendMediaControl(ctx context.Context, control MediaControl) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	switch control.Action {
	case ControlActionBargeIn:
		if err := s.appendEvent(runevents.EventTypeBargeInDetected, control.OccurredAt, map[string]any{
			"voice_session_id": s.voiceSessionID,
			"turn_id":          strings.TrimSpace(control.TurnID),
			"target_artifact":  strings.TrimSpace(control.TargetArtifact),
		}, turnSummary(control.TurnID, "caller", "control")); err != nil {
			return err
		}
		if control.ClearBuffer {
			return s.appendEvent(runevents.EventTypeAudioBufferCleared, control.OccurredAt, map[string]any{
				"voice_session_id": s.voiceSessionID,
				"turn_id":          strings.TrimSpace(control.TurnID),
				"target_artifact":  strings.TrimSpace(control.TargetArtifact),
			}, turnSummary(control.TurnID, "system", "control"))
		}
		return nil
	case ControlActionClearBuffer:
		return s.appendEvent(runevents.EventTypeAudioBufferCleared, control.OccurredAt, map[string]any{
			"voice_session_id": s.voiceSessionID,
			"turn_id":          strings.TrimSpace(control.TurnID),
			"target_artifact":  strings.TrimSpace(control.TargetArtifact),
		}, turnSummary(control.TurnID, "system", "control"))
	default:
		return s.failForOperation(control.OccurredAt, "invalid_media_control", fmt.Errorf("%w: unsupported media control action %q", ErrInvalidOperation, control.Action))
	}
}

func (s *fakeSession) Stop(ctx context.Context, request StopRequest) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	reason := request.Reason
	if reason == "" {
		reason = StopReasonCompleted
	}
	switch reason {
	case StopReasonCompleted, StopReasonAgentHangup:
		s.ensureRequiredArtifacts()
		if err := s.appendEvent(runevents.EventTypeSystemRunCompleted, request.OccurredAt, map[string]any{
			"voice_session_id": s.voiceSessionID,
			"reason":           string(reason),
			"message":          strings.TrimSpace(request.Message),
		}, runevents.SummaryMetadata{
			Status:         string(StatusCompleted),
			Channel:        "lifecycle",
			IdempotencyKey: fmt.Sprintf("%s:%s", s.voiceSessionID, reason),
		}); err != nil {
			return err
		}
		s.closed = true
		s.status = StatusCompleted
		return nil
	case StopReasonTransportFailure:
		return s.Fail(ctx, LifecycleError{Code: string(StopReasonTransportFailure), Message: firstNonEmpty(request.Message, "transport failed"), OccurredAt: request.OccurredAt})
	default:
		return s.failForOperation(request.OccurredAt, "invalid_stop_reason", fmt.Errorf("%w: unsupported stop reason %q", ErrInvalidOperation, reason))
	}
}

func (s *fakeSession) Fail(ctx context.Context, failure LifecycleError) error {
	if err := s.ready(ctx); err != nil {
		return err
	}
	return s.failInternal(failure.OccurredAt, firstNonEmpty(failure.Code, "transport_failure"), firstNonEmpty(failure.Message, "transport failed"))
}

func (s *fakeSession) Events() []runevents.Envelope {
	return append([]runevents.Envelope(nil), s.events...)
}

func (s *fakeSession) ArtifactManifest() voiceartifacts.Manifest {
	manifest := voiceartifacts.Manifest{
		SchemaVersion:  voiceartifacts.SchemaVersionV1,
		RunID:          s.runID,
		RunAgentID:     s.runAgentID,
		VoiceSessionID: s.voiceSessionID,
		Artifacts:      make([]voiceartifacts.ArtifactRef, 0, len(s.artifacts)),
	}
	for _, artifact := range s.artifacts {
		manifest.Artifacts = append(manifest.Artifacts, artifact)
	}
	sort.SliceStable(manifest.Artifacts, func(i, j int) bool {
		return manifest.Artifacts[i].Key < manifest.Artifacts[j].Key
	})
	return manifest
}

type fakeSession struct {
	runID          uuid.UUID
	runAgentID     uuid.UUID
	voiceSessionID string
	startedAt      time.Time
	artifactBucket string
	nextSequence   int64
	events         []runevents.Envelope
	artifacts      map[voiceartifacts.ArtifactKind]voiceartifacts.ArtifactRef
	closed         bool
	status         Status
	failureReason  string
}

func (s *fakeSession) ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.closed {
		return ErrSessionClosed
	}
	return nil
}

func (s *fakeSession) appendEvent(eventType runevents.Type, occurredAt time.Time, payload any, summary runevents.SummaryMetadata) error {
	if occurredAt.IsZero() {
		occurredAt = s.startedAt
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal live voice event payload: %w", err)
	}
	if summary.EvidenceLevel == "" {
		summary.EvidenceLevel = runevents.EvidenceLevelVoiceStructured
	}
	if summary.IdempotencyKey == "" {
		summary.IdempotencyKey = fmt.Sprintf("%s:%03d:%s", s.voiceSessionID, s.nextSequence+1, eventType)
	}
	s.nextSequence++
	envelope := runevents.Envelope{
		EventID:        summary.IdempotencyKey,
		SchemaVersion:  runevents.SchemaVersionV1,
		RunID:          s.runID,
		RunAgentID:     s.runAgentID,
		SequenceNumber: s.nextSequence,
		EventType:      eventType,
		Source:         runevents.SourceVoiceAdapter,
		OccurredAt:     occurredAt.UTC(),
		Payload:        rawPayload,
		Summary:        summary,
	}
	if err := envelope.ValidatePersisted(); err != nil {
		return err
	}
	s.events = append(s.events, envelope)
	return nil
}

func (s *fakeSession) failForOperation(occurredAt time.Time, code string, cause error) error {
	if err := s.failInternal(occurredAt, code, cause.Error()); err != nil {
		return err
	}
	return cause
}

func (s *fakeSession) failInternal(occurredAt time.Time, code string, message string) error {
	s.ensureRequiredArtifacts()
	if err := s.appendEvent(runevents.EventTypeSystemRunFailed, occurredAt, map[string]any{
		"voice_session_id": s.voiceSessionID,
		"code":             strings.TrimSpace(code),
		"message":          strings.TrimSpace(message),
	}, runevents.SummaryMetadata{
		Status:         string(StatusFailed),
		Channel:        "lifecycle",
		IdempotencyKey: fmt.Sprintf("%s:failed:%s", s.voiceSessionID, strings.TrimSpace(code)),
	}); err != nil {
		return err
	}
	s.closed = true
	s.status = StatusFailed
	s.failureReason = strings.TrimSpace(message)
	return nil
}

func (s *fakeSession) ensureRequiredArtifacts() {
	s.ensureArtifact(voiceartifacts.ArtifactKindCallerAudio)
	s.ensureArtifact(voiceartifacts.ArtifactKindAgentAudio)
	s.ensureArtifact(voiceartifacts.ArtifactKindTranscriptJSON)
	s.ensureArtifact(voiceartifacts.ArtifactKindWaveformTimeline)
	s.ensureArtifact(voiceartifacts.ArtifactKindStructuredOutput)
	s.ensureArtifact(voiceartifacts.ArtifactKindRawProviderTrace)
}

func (s *fakeSession) ensureArtifact(kind voiceartifacts.ArtifactKind) {
	if _, ok := s.artifacts[kind]; ok {
		return
	}
	key := artifactKey(kind)
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", s.voiceSessionID, kind, key)))
	s.artifacts[kind] = voiceartifacts.ArtifactRef{
		Key:            key,
		Kind:           kind,
		Location:       voiceartifacts.ArtifactLocationObjectStorage,
		Bucket:         s.artifactBucket,
		ObjectKey:      fmt.Sprintf("voice/sessions/%s/%s", s.voiceSessionID, key),
		ContentType:    artifactContentType(kind),
		SizeBytes:      int64(len(s.voiceSessionID) + len(key)),
		ChecksumSHA256: hex.EncodeToString(hash[:]),
	}
}

func validateMediaFrame(frame MediaFrame) error {
	if strings.TrimSpace(frame.FrameID) == "" {
		return fmt.Errorf("%w: frame_id is required", ErrMissingMediaFrame)
	}
	if strings.TrimSpace(frame.ArtifactRef) == "" {
		return fmt.Errorf("%w: artifact_ref is required", ErrMissingMediaFrame)
	}
	if strings.TrimSpace(frame.Format) == "" {
		return fmt.Errorf("%w: format is required", ErrMissingMediaFrame)
	}
	if frame.DurationMS < 0 {
		return fmt.Errorf("%w: duration_ms must be non-negative", ErrInvalidOperation)
	}
	return nil
}

func turnSummary(turnID string, speaker string, channel string) runevents.SummaryMetadata {
	summary := runevents.SummaryMetadata{
		Speaker:       speaker,
		Channel:       channel,
		EvidenceLevel: runevents.EvidenceLevelVoiceStructured,
	}
	if index, ok := parseTurnIndex(turnID); ok {
		summary.TurnIndex = &index
	}
	return summary
}

func parseTurnIndex(turnID string) (int, bool) {
	trimmed := strings.TrimSpace(turnID)
	if trimmed == "" {
		return 0, false
	}
	var index int
	if _, err := fmt.Sscanf(trimmed, "turn-%d", &index); err == nil && index >= 0 {
		return index, true
	}
	return 0, false
}

func artifactKey(kind voiceartifacts.ArtifactKind) string {
	switch kind {
	case voiceartifacts.ArtifactKindCallerAudio:
		return "caller_audio"
	case voiceartifacts.ArtifactKindAgentAudio:
		return "agent_audio"
	case voiceartifacts.ArtifactKindTranscriptJSON:
		return "transcript"
	case voiceartifacts.ArtifactKindWaveformTimeline:
		return "waveform_timeline"
	case voiceartifacts.ArtifactKindStructuredOutput:
		return "structured_output"
	case voiceartifacts.ArtifactKindRawProviderTrace:
		return "raw_provider_trace"
	case voiceartifacts.ArtifactKindMixedAudio:
		return "mixed_audio"
	case voiceartifacts.ArtifactKindRedactionMetadata:
		return "redaction_metadata"
	default:
		return strings.TrimSpace(string(kind))
	}
}

func artifactContentType(kind voiceartifacts.ArtifactKind) string {
	switch kind {
	case voiceartifacts.ArtifactKindCallerAudio,
		voiceartifacts.ArtifactKindAgentAudio,
		voiceartifacts.ArtifactKindMixedAudio:
		return "audio/wav"
	default:
		return "application/json"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type Script struct {
	SchemaVersion  string         `json:"schema_version"`
	RunID          uuid.UUID      `json:"run_id"`
	RunAgentID     uuid.UUID      `json:"run_agent_id"`
	VoiceSessionID string         `json:"voice_session_id"`
	BaseTime       time.Time      `json:"base_time"`
	ArtifactBucket string         `json:"artifact_bucket,omitempty"`
	Actions        []ScriptAction `json:"actions"`
}

type ScriptAction struct {
	OffsetMS   int64              `json:"offset_ms"`
	Kind       ActionKind         `json:"kind"`
	Frame      *MediaFrame        `json:"frame,omitempty"`
	Transcript *TranscriptSegment `json:"transcript,omitempty"`
	Artifact   *ArtifactOutput    `json:"artifact,omitempty"`
	Control    *MediaControl      `json:"control,omitempty"`
	Failure    *LifecycleError    `json:"failure,omitempty"`
}

type ActionKind string

const (
	ActionInboundMedia      ActionKind = "inbound_media"
	ActionInboundTranscript ActionKind = "inbound_transcript"
	ActionOutboundMedia     ActionKind = "outbound_media"
	ActionOutboundArtifact  ActionKind = "outbound_artifact"
	ActionMediaControl      ActionKind = "media_control"
	ActionTransportFailure  ActionKind = "transport_failure"
	ActionAgentHangup       ActionKind = "agent_hangup"
	ActionStop              ActionKind = "stop"
)

func (t *FakeTransport) Run(ctx context.Context, script Script) (Result, error) {
	if err := script.Validate(); err != nil {
		return Result{}, err
	}
	session, err := t.StartSession(ctx, StartSessionInput{
		RunID:          script.RunID,
		RunAgentID:     script.RunAgentID,
		VoiceSessionID: script.VoiceSessionID,
		StartedAt:      script.BaseTime,
		ArtifactBucket: script.ArtifactBucket,
	})
	if err != nil {
		return Result{}, err
	}
	fake := session.(*fakeSession)
	var runErr error
	for _, action := range script.Actions {
		occurredAt := script.BaseTime.Add(time.Duration(action.OffsetMS) * time.Millisecond).UTC()
		runErr = runAction(ctx, session, action, occurredAt)
		if runErr != nil || fake.closed {
			break
		}
	}
	if runErr == nil && !fake.closed {
		runErr = session.Stop(ctx, StopRequest{Reason: StopReasonCompleted, OccurredAt: latestEventTime(fake.events, script.BaseTime)})
	}
	result := Result{
		Status:        fake.status,
		FailureReason: fake.failureReason,
		Events:        session.Events(),
		Manifest:      session.ArtifactManifest(),
	}
	if result.Status == "" {
		result.Status = StatusCompleted
	}
	if err := validateResultManifest(result.Manifest, runErr); err != nil {
		return Result{}, err
	}
	return result, runErr
}

func validateResultManifest(manifest voiceartifacts.Manifest, runErr error) error {
	if err := manifest.Validate(); err != nil {
		if runErr != nil {
			return fmt.Errorf("%w; original run error: %v", err, runErr)
		}
		return err
	}
	return nil
}

func runAction(ctx context.Context, session Session, action ScriptAction, occurredAt time.Time) error {
	switch action.Kind {
	case ActionInboundMedia:
		frame := MediaFrame{}
		if action.Frame != nil {
			frame = *action.Frame
		}
		frame.OccurredAt = occurredAt
		return session.ReceiveMediaFrame(ctx, frame)
	case ActionInboundTranscript:
		if action.Transcript == nil {
			return fmt.Errorf("%w: transcript action requires transcript", ErrInvalidOperation)
		}
		segment := *action.Transcript
		segment.OccurredAt = occurredAt
		return session.ReceiveTranscript(ctx, segment)
	case ActionOutboundMedia:
		frame := MediaFrame{}
		if action.Frame != nil {
			frame = *action.Frame
		}
		frame.OccurredAt = occurredAt
		return session.SendOutboundMediaFrame(ctx, frame)
	case ActionOutboundArtifact:
		if action.Artifact == nil {
			return fmt.Errorf("%w: outbound artifact action requires artifact", ErrInvalidOperation)
		}
		artifact := *action.Artifact
		artifact.OccurredAt = occurredAt
		return session.SendOutboundArtifact(ctx, artifact)
	case ActionMediaControl:
		if action.Control == nil {
			return fmt.Errorf("%w: media control action requires control", ErrInvalidOperation)
		}
		control := *action.Control
		control.OccurredAt = occurredAt
		return session.SendMediaControl(ctx, control)
	case ActionTransportFailure:
		failure := LifecycleError{Code: string(StopReasonTransportFailure), Message: "transport failed"}
		if action.Failure != nil {
			failure = *action.Failure
		}
		failure.OccurredAt = occurredAt
		return session.Fail(ctx, failure)
	case ActionAgentHangup:
		return session.Stop(ctx, StopRequest{Reason: StopReasonAgentHangup, Message: "agent ended call", OccurredAt: occurredAt})
	case ActionStop:
		return session.Stop(ctx, StopRequest{Reason: StopReasonCompleted, OccurredAt: occurredAt})
	default:
		return fmt.Errorf("%w: unsupported action kind %q", ErrInvalidOperation, action.Kind)
	}
}

func (s Script) Validate() error {
	if s.SchemaVersion != ScriptSchemaVersionV1 {
		return fmt.Errorf("%w: schema_version must be %q", ErrInvalidScript, ScriptSchemaVersionV1)
	}
	if s.RunID == uuid.Nil {
		return fmt.Errorf("%w: run_id is required", ErrInvalidScript)
	}
	if s.RunAgentID == uuid.Nil {
		return fmt.Errorf("%w: run_agent_id is required", ErrInvalidScript)
	}
	if strings.TrimSpace(s.VoiceSessionID) == "" {
		return fmt.Errorf("%w: voice_session_id is required", ErrInvalidScript)
	}
	if s.BaseTime.IsZero() {
		return fmt.Errorf("%w: base_time is required", ErrInvalidScript)
	}
	if len(s.Actions) == 0 {
		return fmt.Errorf("%w: actions must contain at least one action", ErrInvalidScript)
	}
	previousOffset := int64(-1)
	for idx, action := range s.Actions {
		if action.OffsetMS < 0 {
			return fmt.Errorf("actions[%d]: %w: offset_ms must be non-negative", idx, ErrInvalidScript)
		}
		if action.OffsetMS < previousOffset {
			return fmt.Errorf("actions[%d]: %w: offset_ms must not go backward", idx, ErrInvalidScript)
		}
		if !action.Kind.Valid() {
			return fmt.Errorf("actions[%d]: %w: unsupported kind %q", idx, ErrInvalidScript, action.Kind)
		}
		if err := action.Validate(); err != nil {
			return fmt.Errorf("actions[%d]: %w", idx, err)
		}
		previousOffset = action.OffsetMS
	}
	return nil
}

func (a ScriptAction) Validate() error {
	switch a.Kind {
	case ActionInboundMedia, ActionOutboundMedia:
		return nil
	case ActionInboundTranscript:
		if a.Transcript == nil {
			return fmt.Errorf("%w: transcript action requires transcript", ErrInvalidScript)
		}
	case ActionOutboundArtifact:
		if a.Artifact == nil {
			return fmt.Errorf("%w: outbound artifact action requires artifact", ErrInvalidScript)
		}
	case ActionMediaControl:
		if a.Control == nil {
			return fmt.Errorf("%w: media control action requires control", ErrInvalidScript)
		}
	case ActionTransportFailure, ActionAgentHangup, ActionStop:
		return nil
	default:
		return fmt.Errorf("%w: unsupported kind %q", ErrInvalidScript, a.Kind)
	}
	return nil
}

func (k ActionKind) Valid() bool {
	switch k {
	case ActionInboundMedia,
		ActionInboundTranscript,
		ActionOutboundMedia,
		ActionOutboundArtifact,
		ActionMediaControl,
		ActionTransportFailure,
		ActionAgentHangup,
		ActionStop:
		return true
	default:
		return false
	}
}

func latestEventTime(events []runevents.Envelope, fallback time.Time) time.Time {
	latest := fallback.UTC()
	for _, event := range events {
		if event.OccurredAt.After(latest) {
			latest = event.OccurredAt
		}
	}
	return latest
}
