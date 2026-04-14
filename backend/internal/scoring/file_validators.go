package scoring

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
)

// --- file_exists ---

type fileExistsConfig struct {
	MustExist bool `json:"must_exist"`
}

func validateFileExists(actual string, config json.RawMessage) validatorOutcome {
	var cfg fileExistsConfig
	cfg.MustExist = true // default: file should exist
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return validatorOutcome{verdict: "error", reason: fmt.Sprintf("invalid file_exists config: %v", err)}
		}
	}

	// applyValidator is only called when evidence is non-nil, which means the
	// file exists. The not-exists case is handled earlier in evaluateValidators
	// via validateFileExistsUnavailable.
	if cfg.MustExist {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1)}
	}
	// must_exist: false — file should NOT exist, but it does.
	return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: "file exists but should not"}
}

// --- file_content_match ---

type fileContentMatchConfig struct {
	MatchMode        string `json:"match_mode"`
	IgnoreWhitespace bool   `json:"ignore_whitespace,omitempty"`
}

func validateFileContentMatch(actual, expected string, config json.RawMessage) validatorOutcome {
	var cfg fileContentMatchConfig
	cfg.MatchMode = "contains" // default match mode
	if len(config) > 0 {
		if err := json.Unmarshal(config, &cfg); err != nil {
			return validatorOutcome{verdict: "error", reason: fmt.Sprintf("invalid file_content_match config: %v", err)}
		}
	}

	actualVal := actual
	expectedVal := expected
	if cfg.IgnoreWhitespace {
		actualVal = collapseWhitespace(actualVal)
		expectedVal = collapseWhitespace(expectedVal)
	}

	switch cfg.MatchMode {
	case "exact":
		if actualVal == expectedVal {
			return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1)}
		}
		return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: "file content does not match exactly"}

	case "contains":
		if strings.Contains(actualVal, expectedVal) {
			return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1)}
		}
		return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: "file content does not contain expected string"}

	case "regex":
		pattern, err := regexp.Compile(expected) // Use original expected for regex (not whitespace-collapsed)
		if err != nil {
			return validatorOutcome{verdict: "error", reason: fmt.Sprintf("invalid regex pattern: %v", err)}
		}
		if pattern.MatchString(actual) {
			return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1)}
		}
		return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: "file content does not match regex pattern"}

	case "not_contains":
		if !strings.Contains(actualVal, expectedVal) {
			return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1)}
		}
		return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: "file content contains forbidden string"}

	case "json_equal":
		return validateJSONEqual(actual, expected)

	default:
		return validatorOutcome{verdict: "error", reason: fmt.Sprintf("unsupported match_mode %q", cfg.MatchMode)}
	}
}

func validateJSONEqual(actual, expected string) validatorOutcome {
	var actualJSON, expectedJSON any
	if err := json.Unmarshal([]byte(actual), &actualJSON); err != nil {
		return validatorOutcome{verdict: "error", reason: fmt.Sprintf("parse actual as JSON: %v", err)}
	}
	if err := json.Unmarshal([]byte(expected), &expectedJSON); err != nil {
		return validatorOutcome{verdict: "error", reason: fmt.Sprintf("parse expected as JSON: %v", err)}
	}
	if reflect.DeepEqual(actualJSON, expectedJSON) {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1)}
	}
	return validatorOutcome{verdict: "fail", normalizedScore: floatPtr(0), reason: "file JSON content is not structurally equal"}
}

// --- file_json_schema ---

type fileJSONSchemaConfig struct {
	Schema json.RawMessage `json:"schema"`
}

func validateFileJSONSchema(actual string, config json.RawMessage) validatorOutcome {
	var cfg fileJSONSchemaConfig
	if len(config) == 0 {
		return validatorOutcome{verdict: "error", reason: "file_json_schema config is required"}
	}
	if err := json.Unmarshal(config, &cfg); err != nil {
		return validatorOutcome{verdict: "error", reason: fmt.Sprintf("invalid file_json_schema config: %v", err)}
	}
	if len(cfg.Schema) == 0 {
		return validatorOutcome{verdict: "error", reason: "file_json_schema config.schema is required"}
	}
	// Reuse the existing JSON schema validator: it takes actual JSON and expected
	// (the schema) as strings.
	return validateJSONSchema(actual, string(cfg.Schema))
}

// --- directory_structure ---

type directoryStructureConfig struct {
	RequiredFiles       []string `json:"required_files,omitempty"`
	ForbiddenFiles      []string `json:"forbidden_files,omitempty"`
	RequiredDirectories []string `json:"required_directories,omitempty"`
}

func validateDirectoryStructure(actual string, config json.RawMessage) validatorOutcome {
	var cfg directoryStructureConfig
	if len(config) == 0 {
		return validatorOutcome{verdict: "error", reason: "directory_structure config is required"}
	}
	if err := json.Unmarshal(config, &cfg); err != nil {
		return validatorOutcome{verdict: "error", reason: fmt.Sprintf("invalid directory_structure config: %v", err)}
	}

	// Parse the directory listing from the actual value (JSON-serialized DirectoryListingResult).
	var listing DirectoryListingResult
	if err := json.Unmarshal([]byte(actual), &listing); err != nil {
		return validatorOutcome{verdict: "error", reason: fmt.Sprintf("parse directory listing: %v", err)}
	}

	filePaths := make(map[string]bool, len(listing.Entries))
	dirPaths := make(map[string]bool)
	for _, entry := range listing.Entries {
		// Use filepath.Rel for robust relative-path computation regardless of
		// trailing-slash inconsistencies between listing.Path and entry.Path.
		rel, err := filepath.Rel(listing.Path, entry.Path)
		if err != nil {
			rel = entry.Path
		}
		filePaths[rel] = true
		filePaths[entry.Path] = true
		if entry.IsDir {
			dirPaths[rel] = true
			dirPaths[entry.Path] = true
		}
	}

	var missing []string
	for _, required := range cfg.RequiredFiles {
		if !filePaths[required] {
			missing = append(missing, required)
		}
	}
	var forbidden []string
	for _, deny := range cfg.ForbiddenFiles {
		if filePaths[deny] {
			forbidden = append(forbidden, deny)
		}
	}
	var missingDirs []string
	for _, required := range cfg.RequiredDirectories {
		trimmed := strings.TrimSuffix(required, "/")
		if !dirPaths[trimmed] && !dirPaths[required] && !filePaths[trimmed] {
			missingDirs = append(missingDirs, required)
		}
	}

	if len(missing) == 0 && len(forbidden) == 0 && len(missingDirs) == 0 {
		return validatorOutcome{verdict: "pass", normalizedScore: floatPtr(1)}
	}

	var reasons []string
	if len(missing) > 0 {
		reasons = append(reasons, fmt.Sprintf("missing required files: %s", strings.Join(missing, ", ")))
	}
	if len(forbidden) > 0 {
		reasons = append(reasons, fmt.Sprintf("forbidden files present: %s", strings.Join(forbidden, ", ")))
	}
	if len(missingDirs) > 0 {
		reasons = append(reasons, fmt.Sprintf("missing required directories: %s", strings.Join(missingDirs, ", ")))
	}
	return validatorOutcome{
		verdict:         "fail",
		normalizedScore: floatPtr(0),
		reason:          strings.Join(reasons, "; "),
	}
}

// collapseWhitespace is defined in string_validators.go and shared across
// all validators in the scoring package.
