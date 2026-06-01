package engine

import (
	"encoding/json"
	"strings"
)

func executionModeFromManifest(manifest json.RawMessage) string {
	if len(manifest) == 0 {
		return ""
	}
	var decoded struct {
		Version struct {
			ExecutionMode string `json:"execution_mode"`
		} `json:"version"`
	}
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		return ""
	}
	return strings.TrimSpace(decoded.Version.ExecutionMode)
}
