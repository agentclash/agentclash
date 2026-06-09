package api

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

// ArtifactContentSigner mints signed, time-limited public content URLs for
// artifacts so they can be embedded in shared (unauthenticated) tryout
// payloads. Implemented by *ArtifactManager. It performs NO authorization;
// callers must independently confirm the artifact is safe to expose (here:
// template-allowlisted on a redaction-passed share).
type ArtifactContentSigner interface {
	SignedArtifactContentURL(artifactID uuid.UUID, baseURL string, now time.Time) (url string, expiresAt time.Time, err error)
}

// sharedAgentTryoutResponse is the public, redaction-safe view of a tryout
// rendered behind a share token. It embeds the public tryout fields (which
// already omit org/workspace/user identifiers) and adds the template-approved,
// redacted, signed artifact descriptors.
type sharedAgentTryoutResponse struct {
	publicAgentTryoutResponse
	Artifacts []sharedAgentTryoutArtifact `json:"artifacts"`
}

// sharedAgentTryoutArtifact is a redacted artifact descriptor safe for public
// shares. Its key/type/path come from the template allowlist (trusted,
// template-defined) rather than raw artifact metadata; storage bucket/key,
// org/workspace/run identifiers, and arbitrary metadata are never included.
type sharedAgentTryoutArtifact struct {
	Key               string     `json:"key"`
	Type              string     `json:"type"`
	Path              string     `json:"path"`
	ContentType       *string    `json:"content_type,omitempty"`
	SizeBytes         *int64     `json:"size_bytes,omitempty"`
	ChecksumSHA256    *string    `json:"checksum_sha256,omitempty"`
	DownloadURL       string     `json:"download_url,omitempty"`
	DownloadExpiresAt *time.Time `json:"download_expires_at,omitempty"`
}

// sensitiveSummaryKeySubstrings lists object-key substrings whose values must
// never appear in a public tryout summary. Matching is by key name (not value)
// so it stays resilient to the execution-defined, evolving summary schema.
// Entries are targeted (e.g. "access_token", "session_id") rather than bare
// "token"/"session" so legitimate public metrics like "total_tokens",
// "output_tokens", or "session_count" survive redaction.
var sensitiveSummaryKeySubstrings = []string{
	"secret", "credential", "password", "api_key", "apikey",
	"access_token", "auth_token", "session_token", "id_token", "refresh_token",
	"authorization", "bearer", "private_key", "access_key", "session_id",
	"cookie", "organization_id", "org_id", "workspace_id", "user_id",
	"created_by", "claimed_by", "fingerprint",
}

func sensitiveSummaryKey(name string) bool {
	lower := strings.ToLower(name)
	for _, substr := range sensitiveSummaryKeySubstrings {
		if strings.Contains(lower, substr) {
			return true
		}
	}
	return false
}

// redactTryoutSummaryForPublic recursively strips object keys whose names look
// sensitive from a tryout summary before public exposure. The summary has
// already cleared the execution-side redaction pipeline (share rendering is
// gated on AgentTryoutRedactionStatus.ShareReady), so this is defense-in-depth
// against identifier/secret leakage rather than the primary control. Invalid
// JSON is dropped entirely rather than passed through.
func redactTryoutSummaryForPublic(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	cleaned, err := json.Marshal(scrubSensitiveSummary(decoded))
	if err != nil {
		return nil
	}
	return cleaned
}

func scrubSensitiveSummary(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			if sensitiveSummaryKey(key) {
				continue
			}
			out[key] = scrubSensitiveSummary(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = scrubSensitiveSummary(item)
		}
		return out
	default:
		return v
	}
}

// templateAllowedArtifact is one entry of a template's expected/approved
// artifact outputs, used as the allowlist for shared/downloadable artifacts.
type templateAllowedArtifact struct {
	Key  string
	Type string
	Path string
}

// parseTemplateArtifactAllowlist extracts the per-template artifact allowlist
// from a stored template snapshot (runtime.expected_artifacts). An empty result
// means nothing is approved for public download (default-deny).
func parseTemplateArtifactAllowlist(templateSnapshot json.RawMessage) []templateAllowedArtifact {
	if len(templateSnapshot) == 0 {
		return nil
	}
	var snapshot struct {
		Runtime struct {
			ExpectedArtifacts []struct {
				Key  string `json:"key"`
				Type string `json:"type"`
				Path string `json:"path"`
			} `json:"expected_artifacts"`
		} `json:"runtime"`
	}
	if err := json.Unmarshal(templateSnapshot, &snapshot); err != nil {
		return nil
	}
	allow := make([]templateAllowedArtifact, 0, len(snapshot.Runtime.ExpectedArtifacts))
	for _, entry := range snapshot.Runtime.ExpectedArtifacts {
		key := strings.TrimSpace(entry.Key)
		path := strings.TrimSpace(entry.Path)
		if key == "" && path == "" {
			continue
		}
		allow = append(allow, templateAllowedArtifact{Key: key, Type: strings.TrimSpace(entry.Type), Path: path})
	}
	if len(allow) == 0 {
		return nil
	}
	return allow
}

// artifactClaimedIdentity extracts the logical key/path an artifact claims via
// its metadata, used to match it against the template allowlist. Returns empty
// strings when the artifact carries no recognized identity, which (combined
// with allowlist matching) yields default-deny.
func artifactClaimedIdentity(metadata json.RawMessage) (key string, path string) {
	if len(metadata) == 0 {
		return "", ""
	}
	var meta map[string]any
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return "", ""
	}
	key = metadataString(meta, "artifact_key")
	path = metadataString(meta, "relative_path")
	if path == "" {
		path = metadataString(meta, "path")
	}
	return key, path
}

func metadataString(meta map[string]any, key string) string {
	if value, ok := meta[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

// buildSharedTryoutArtifacts filters a run's artifacts down to the
// template-approved allowlist, emits redacted descriptors (canonical key/type/
// path from the template, non-sensitive size/checksum/content-type from the
// artifact row), and attaches signed download URLs when a signer is available.
// Unmatched artifacts are dropped (default-deny) so non-approved outputs are
// never exposed on a public share.
func buildSharedTryoutArtifacts(artifacts []repository.Artifact, allow []templateAllowedArtifact, signer ArtifactContentSigner, baseURL string, now time.Time) []sharedAgentTryoutArtifact {
	if len(allow) == 0 || len(artifacts) == 0 {
		return nil
	}
	byKey := make(map[string]templateAllowedArtifact, len(allow))
	byPath := make(map[string]templateAllowedArtifact, len(allow))
	for _, entry := range allow {
		if entry.Key != "" {
			byKey[entry.Key] = entry
		}
		if entry.Path != "" {
			byPath[entry.Path] = entry
		}
	}

	var out []sharedAgentTryoutArtifact
	for _, artifact := range artifacts {
		key, path := artifactClaimedIdentity(artifact.Metadata)
		matched, ok := templateAllowedArtifact{}, false
		if key != "" {
			matched, ok = byKey[key]
		}
		if !ok && path != "" {
			matched, ok = byPath[path]
		}
		if !ok {
			continue
		}
		descriptor := sharedAgentTryoutArtifact{
			Key:            matched.Key,
			Type:           matched.Type,
			Path:           matched.Path,
			ContentType:    artifact.ContentType,
			SizeBytes:      artifact.SizeBytes,
			ChecksumSHA256: artifact.ChecksumSHA256,
		}
		if signer != nil && baseURL != "" {
			if signedURL, expiresAt, err := signer.SignedArtifactContentURL(artifact.ID, baseURL, now); err == nil {
				expires := expiresAt
				descriptor.DownloadURL = signedURL
				descriptor.DownloadExpiresAt = &expires
			}
		}
		out = append(out, descriptor)
	}
	return out
}
