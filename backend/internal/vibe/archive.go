package vibe

import "github.com/google/uuid"

// Decodes retained V1 project provenance; there is no continuation executor.
type retiredArchiveSource struct {
	SessionID  uuid.UUID `json:"session_id"`
	ArtifactID uuid.UUID `json:"artifact_id"`
}
