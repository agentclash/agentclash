package voiceartifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const SchemaVersionV1 = "2026-05-13"

type ArtifactKind string

const (
	ArtifactKindCallerAudio          ArtifactKind = "caller_audio"
	ArtifactKindAgentAudio           ArtifactKind = "agent_audio"
	ArtifactKindMixedAudio           ArtifactKind = "mixed_audio"
	ArtifactKindTranscriptJSON       ArtifactKind = "transcript_json"
	ArtifactKindWaveformTimeline     ArtifactKind = "waveform_timeline_json"
	ArtifactKindMediaPolicyReport    ArtifactKind = "media_policy_report_json"
	ArtifactKindLiveContinuityReport ArtifactKind = "live_continuity_report_json"
	ArtifactKindVideoSyncReport      ArtifactKind = "video_sync_report_json"
	ArtifactKindRawProviderTrace     ArtifactKind = "raw_provider_trace_json"
	ArtifactKindStructuredOutput     ArtifactKind = "structured_output_json"
	ArtifactKindRedactionMetadata    ArtifactKind = "redaction_metadata_json"
)

type ArtifactLocationType string

const (
	ArtifactLocationLocalPath     ArtifactLocationType = "local_path"
	ArtifactLocationObjectStorage ArtifactLocationType = "object_storage"
)

var (
	ErrInvalidSchemaVersion      = errors.New("invalid voice artifact manifest schema version")
	ErrInvalidArtifactKind       = errors.New("invalid voice artifact kind")
	ErrInvalidArtifactLocation   = errors.New("invalid voice artifact location")
	ErrMissingRequiredArtifact   = errors.New("missing required voice artifact")
	ErrArtifactChecksumMismatch  = errors.New("voice artifact checksum mismatch")
	errArtifactChecksumRequired  = errors.New("voice artifact checksum_sha256 is required")
	errArtifactReferenceRequired = errors.New("voice artifact reference is required")
)

type Manifest struct {
	SchemaVersion  string         `json:"schema_version"`
	RunID          uuid.UUID      `json:"run_id"`
	RunAgentID     uuid.UUID      `json:"run_agent_id"`
	VoiceSessionID string         `json:"voice_session_id"`
	Artifacts      []ArtifactRef  `json:"artifacts"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type ArtifactRef struct {
	Key            string               `json:"key"`
	Kind           ArtifactKind         `json:"kind"`
	Location       ArtifactLocationType `json:"location"`
	Path           string               `json:"path,omitempty"`
	Bucket         string               `json:"bucket,omitempty"`
	ObjectKey      string               `json:"object_key,omitempty"`
	ContentType    string               `json:"content_type,omitempty"`
	SizeBytes      int64                `json:"size_bytes,omitempty"`
	ChecksumSHA256 string               `json:"checksum_sha256"`
}

func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode voice artifact manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("%w: %q", ErrInvalidSchemaVersion, m.SchemaVersion)
	}
	if m.RunID == uuid.Nil {
		return errors.New("run_id is required")
	}
	if m.RunAgentID == uuid.Nil {
		return errors.New("run_agent_id is required")
	}
	if strings.TrimSpace(m.VoiceSessionID) == "" {
		return errors.New("voice_session_id is required")
	}
	if len(m.Artifacts) == 0 {
		return errors.New("artifacts must contain at least one artifact")
	}

	seenKeys := make(map[string]struct{}, len(m.Artifacts))
	seenKinds := make(map[ArtifactKind]struct{}, len(m.Artifacts))
	for idx, artifact := range m.Artifacts {
		if err := artifact.Validate(); err != nil {
			return fmt.Errorf("artifacts[%d]: %w", idx, err)
		}
		if _, ok := seenKeys[artifact.Key]; ok {
			return fmt.Errorf("artifacts[%d]: duplicate key %q", idx, artifact.Key)
		}
		seenKeys[artifact.Key] = struct{}{}
		seenKinds[artifact.Kind] = struct{}{}
	}

	for _, kind := range requiredArtifactKinds() {
		if _, ok := seenKinds[kind]; !ok {
			return fmt.Errorf("%w: %s", ErrMissingRequiredArtifact, kind)
		}
	}
	return nil
}

func (a ArtifactRef) Validate() error {
	if strings.TrimSpace(a.Key) == "" {
		return errors.New("key is required")
	}
	if !a.Kind.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidArtifactKind, a.Kind)
	}
	if strings.TrimSpace(a.ChecksumSHA256) == "" {
		return errArtifactChecksumRequired
	}
	if !isLowerHexSHA256(a.ChecksumSHA256) {
		return errors.New("checksum_sha256 must be 64 lowercase hex characters")
	}
	switch a.Location {
	case ArtifactLocationLocalPath:
		if strings.TrimSpace(a.Path) == "" {
			return errArtifactReferenceRequired
		}
		cleanPath := filepath.Clean(filepath.FromSlash(a.Path))
		if filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
			return errors.New("local_path path must be relative and must not traverse directories")
		}
		if a.Bucket != "" || a.ObjectKey != "" {
			return errors.New("local_path artifacts must not set bucket or object_key")
		}
	case ArtifactLocationObjectStorage:
		if strings.TrimSpace(a.ObjectKey) == "" {
			return errArtifactReferenceRequired
		}
		if strings.TrimSpace(a.Bucket) == "" {
			return errors.New("object_storage artifacts must set bucket")
		}
		if a.Path != "" {
			return errors.New("object_storage artifacts must not set path")
		}
	default:
		return fmt.Errorf("%w: %q", ErrInvalidArtifactLocation, a.Location)
	}
	if a.SizeBytes < 0 {
		return errors.New("size_bytes must be non-negative")
	}
	return nil
}

func (k ArtifactKind) IsValid() bool {
	switch k {
	case ArtifactKindCallerAudio,
		ArtifactKindAgentAudio,
		ArtifactKindMixedAudio,
		ArtifactKindTranscriptJSON,
		ArtifactKindWaveformTimeline,
		ArtifactKindMediaPolicyReport,
		ArtifactKindLiveContinuityReport,
		ArtifactKindVideoSyncReport,
		ArtifactKindRawProviderTrace,
		ArtifactKindStructuredOutput,
		ArtifactKindRedactionMetadata:
		return true
	default:
		return false
	}
}

func (m Manifest) VerifyLocalChecksums(root string) error {
	if err := m.Validate(); err != nil {
		return err
	}
	for idx, artifact := range m.Artifacts {
		if artifact.Location != ArtifactLocationLocalPath {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(artifact.Path))
		checksum, size, err := FileSHA256(path)
		if err != nil {
			return fmt.Errorf("artifacts[%d]: verify %s: %w", idx, artifact.Key, err)
		}
		if checksum != artifact.ChecksumSHA256 {
			return fmt.Errorf("%w: artifacts[%d] %s got %s want %s", ErrArtifactChecksumMismatch, idx, artifact.Key, checksum, artifact.ChecksumSHA256)
		}
		if artifact.SizeBytes > 0 && artifact.SizeBytes != size {
			return fmt.Errorf("artifacts[%d]: %s size mismatch: got %d bytes, manifest declares %d", idx, artifact.Key, size, artifact.SizeBytes)
		}
	}
	return nil
}

func (m Manifest) StableJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	normalized := m
	normalized.Artifacts = append([]ArtifactRef(nil), m.Artifacts...)
	sort.SliceStable(normalized.Artifacts, func(i, j int) bool {
		return normalized.Artifacts[i].Key < normalized.Artifacts[j].Key
	})
	encoded, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func FileSHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func requiredArtifactKinds() []ArtifactKind {
	return []ArtifactKind{
		ArtifactKindCallerAudio,
		ArtifactKindAgentAudio,
		ArtifactKindTranscriptJSON,
		ArtifactKindWaveformTimeline,
		ArtifactKindStructuredOutput,
	}
}

func isLowerHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}
