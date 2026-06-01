package traces

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type RedactionConfig struct {
	DropMetadataKeys  []string `json:"drop_metadata_keys,omitempty"`
	HashMetadataKeys  []string `json:"hash_metadata_keys,omitempty"`
	DropMetadataPaths []string `json:"drop_metadata_paths,omitempty"`
}

func RedactCandidate(candidate Candidate, cfg RedactionConfig) (Candidate, error) {
	if len(cfg.DropMetadataKeys) == 0 && len(cfg.HashMetadataKeys) == 0 && len(cfg.DropMetadataPaths) == 0 {
		return candidate, nil
	}
	metadata := map[string]any{}
	if len(candidate.Metadata) > 0 {
		if err := json.Unmarshal(candidate.Metadata, &metadata); err != nil {
			return Candidate{}, fmt.Errorf("unmarshal candidate metadata: %w", err)
		}
	}
	for _, key := range cfg.DropMetadataKeys {
		delete(metadata, strings.TrimSpace(key))
	}
	for _, key := range cfg.HashMetadataKeys {
		key = strings.TrimSpace(key)
		value, ok := metadata[key]
		if !ok {
			continue
		}
		metadata[key] = hashValue(value)
	}
	for _, path := range cfg.DropMetadataPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		delete(metadata, path)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return Candidate{}, fmt.Errorf("marshal redacted metadata: %w", err)
	}
	candidate.Metadata = encoded
	return candidate, nil
}

func hashValue(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		raw = []byte(fmt.Sprint(value))
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
