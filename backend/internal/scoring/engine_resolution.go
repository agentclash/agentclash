package scoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func ResolveEvidenceValueForJudge(source string, input EvaluationInput) (*string, string, error) {
	evidence := buildEvidence(input.ChallengeInputs, input.Events)
	value, _, reason, err := resolveEvidenceValue(source, evidence)
	return value, reason, err
}

func resolveEvidenceValue(source string, evidence extractedEvidence) (*string, *uuid.UUID, string, error) {
	switch {
	case source == "final_output":
		if evidence.finalOutput == nil {
			return nil, evidence.finalOutputChallengeID, "final output evidence is unavailable", nil
		}
		return stringPtr(*evidence.finalOutput), evidence.finalOutputChallengeID, "", nil
	case source == "run.final_output":
		if evidence.finalOutput == nil {
			return nil, evidence.finalOutputChallengeID, "final output evidence is unavailable", nil
		}
		return stringPtr(*evidence.finalOutput), evidence.finalOutputChallengeID, "", nil
	case source == "challenge_input":
		if evidence.challengeInputValue == nil {
			return nil, evidence.challengeInputChallengeID, "challenge input evidence is unavailable", nil
		}
		return stringPtr(*evidence.challengeInputValue), evidence.challengeInputChallengeID, "", nil
	case strings.HasPrefix(source, "case."):
		if evidence.caseInput == nil {
			return nil, evidence.challengeInputChallengeID, firstNonEmpty(evidence.caseInputReason, "case evidence is unavailable"), nil
		}
		return resolveCaseEvidence(source, *evidence.caseInput)
	case strings.HasPrefix(source, "artifact."):
		if evidence.caseInput == nil {
			return nil, evidence.challengeInputChallengeID, firstNonEmpty(evidence.caseInputReason, "case evidence is unavailable"), nil
		}
		return resolveArtifactEvidence(source, *evidence.caseInput)
	case strings.HasPrefix(source, "file:"):
		checkKey := strings.TrimPrefix(source, "file:")
		return resolveFileCaptureEvidence(checkKey, evidence)
	case strings.HasPrefix(source, "literal:"):
		value := strings.TrimPrefix(source, "literal:")
		return &value, nil, "", nil
	default:
		return nil, nil, "", fmt.Errorf("unsupported evidence source %q", source)
	}
}

func resolveChallengeInputValue(inputs []EvidenceInput) (*string, *uuid.UUID, *uuid.UUID, []string) {
	if len(inputs) == 0 {
		return nil, nil, nil, []string{"challenge input set is unavailable"}
	}
	if len(inputs) > 1 {
		return nil, nil, nil, []string{"challenge input is ambiguous across multiple items"}
	}

	var decoded any
	if err := json.Unmarshal(inputs[0].Payload, &decoded); err == nil {
		if value, ok := extractLooseString(decoded); ok {
			return &value, uuidPtrOrNil(inputs[0].ChallengeIdentityID), cloneUUIDPtr(inputs[0].RegressionCaseID), nil
		}
	}

	payload := decodePayload(inputs[0].Payload)
	if value, ok := extractLooseString(payload); ok {
		return &value, uuidPtrOrNil(inputs[0].ChallengeIdentityID), cloneUUIDPtr(inputs[0].RegressionCaseID), nil
	}

	normalized := bytes.TrimSpace(inputs[0].Payload)
	if len(normalized) == 0 {
		return nil, uuidPtrOrNil(inputs[0].ChallengeIdentityID), cloneUUIDPtr(inputs[0].RegressionCaseID), []string{"challenge input payload is empty"}
	}
	value := string(normalized)
	return &value, uuidPtrOrNil(inputs[0].ChallengeIdentityID), cloneUUIDPtr(inputs[0].RegressionCaseID), nil
}

func resolveCaseInput(inputs []EvidenceInput) (*EvidenceInput, string) {
	if len(inputs) == 0 {
		return nil, "case evidence is unavailable"
	}
	// Case-oriented evidence currently resolves only when one canonical case is
	// in scope. Multi-case packs will need per-case scoring expansion later.
	if len(inputs) > 1 {
		return nil, "case evidence is ambiguous across multiple cases"
	}
	selected := inputs[0]
	return &selected, ""
}

func resolveCaseEvidence(source string, input EvidenceInput) (*string, *uuid.UUID, string, error) {
	segments := strings.Split(source, ".")
	if len(segments) < 2 {
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", source)
	}

	switch segments[1] {
	case "payload":
		return resolveJSONEvidence(input.Payload, segments[2:], uuidPtrOrNil(input.ChallengeIdentityID))
	case "inputs":
		if len(segments) < 3 || strings.TrimSpace(segments[2]) == "" {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", source)
		}
		value, ok := input.Inputs[segments[2]]
		if !ok {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("case input %q is unavailable", segments[2]), nil
		}
		return resolveEvidenceField(value, input, segments[3:])
	case "expectations":
		if len(segments) < 3 || strings.TrimSpace(segments[2]) == "" {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", source)
		}
		value, ok := input.Expectations[segments[2]]
		if !ok {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("case expectation %q is unavailable", segments[2]), nil
		}
		return resolveEvidenceField(value, input, segments[3:])
	default:
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", source)
	}
}

func resolveArtifactEvidence(source string, input EvidenceInput) (*string, *uuid.UUID, string, error) {
	segments := strings.Split(source, ".")
	if len(segments) < 2 || strings.TrimSpace(segments[1]) == "" {
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", source)
	}

	artifact, ok := input.Artifacts[segments[1]]
	if !ok {
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("artifact %q is unavailable", segments[1]), nil
	}
	return resolveArtifactValue(artifact, segments[2:], uuidPtrOrNil(input.ChallengeIdentityID))
}

func resolveEvidenceField(value EvidenceValue, input EvidenceInput, extra []string) (*string, *uuid.UUID, string, error) {
	return resolveEvidenceFieldWithDepth(value, input, extra, 0)
}

func resolveEvidenceFieldWithDepth(value EvidenceValue, input EvidenceInput, extra []string, depth int) (*string, *uuid.UUID, string, error) {
	if depth > 8 {
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("evidence reference chain exceeds maximum depth")
	}
	switch {
	case len(bytes.TrimSpace(value.Value)) > 0:
		return resolveJSONEvidence(value.Value, extra, uuidPtrOrNil(input.ChallengeIdentityID))
	case value.Source != "":
		switch {
		case strings.HasPrefix(value.Source, "input:"):
			inputKey := strings.TrimSpace(strings.TrimPrefix(value.Source, "input:"))
			if inputKey == "" {
				return nil, uuidPtrOrNil(input.ChallengeIdentityID), "referenced input is unavailable", nil
			}
			referenced, ok := input.Inputs[inputKey]
			if !ok {
				return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("case input %q is unavailable", inputKey), nil
			}
			return resolveEvidenceFieldWithDepth(referenced, input, extra, depth+1)
		case strings.HasPrefix(value.Source, "artifact:"):
			artifactKey := strings.TrimSpace(strings.TrimPrefix(value.Source, "artifact:"))
			if artifactKey == "" {
				return nil, uuidPtrOrNil(input.ChallengeIdentityID), "referenced artifact is unavailable", nil
			}
			artifact, ok := input.Artifacts[artifactKey]
			if !ok {
				return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("artifact %q is unavailable", artifactKey), nil
			}
			return resolveArtifactValue(artifact, extra, uuidPtrOrNil(input.ChallengeIdentityID))
		default:
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", fmt.Errorf("unsupported evidence source %q", value.Source)
		}
	case value.ArtifactKey != "":
		artifact, ok := input.Artifacts[value.ArtifactKey]
		if !ok {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), fmt.Sprintf("artifact %q is unavailable", value.ArtifactKey), nil
		}
		return resolveArtifactValue(artifact, extra, uuidPtrOrNil(input.ChallengeIdentityID))
	case strings.TrimSpace(value.Path) != "":
		encoded, err := json.Marshal(value.Path)
		if err != nil {
			return nil, uuidPtrOrNil(input.ChallengeIdentityID), "", err
		}
		return resolveJSONString(encoded, extra, uuidPtrOrNil(input.ChallengeIdentityID))
	default:
		return nil, uuidPtrOrNil(input.ChallengeIdentityID), "evidence is unavailable", nil
	}
}

func resolveArtifactValue(artifact EvidenceArtifact, extra []string, challengeID *uuid.UUID) (*string, *uuid.UUID, string, error) {
	if len(extra) == 0 {
		return stringPtr(artifact.Path), challengeID, "", nil
	}
	payload, err := json.Marshal(map[string]any{
		"key":        artifact.Key,
		"kind":       artifact.Kind,
		"path":       artifact.Path,
		"media_type": artifact.MediaType,
	})
	if err != nil {
		return nil, challengeID, "", err
	}
	return resolveJSONEvidence(payload, extra, challengeID)
}

func resolveJSONEvidence(raw json.RawMessage, extra []string, challengeID *uuid.UUID) (*string, *uuid.UUID, string, error) {
	return resolveJSONString(raw, extra, challengeID)
}

func resolveJSONString(raw json.RawMessage, extra []string, challengeID *uuid.UUID) (*string, *uuid.UUID, string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, challengeID, "evidence is unavailable", nil
	}
	if len(extra) == 0 {
		value := stringifyEvidenceJSON(trimmed)
		return &value, challengeID, "", nil
	}

	var decoded any
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return nil, challengeID, "", fmt.Errorf("resolve evidence path: %w", err)
	}
	value, ok := walkEvidenceValue(decoded, extra)
	if !ok {
		return nil, challengeID, "evidence path is unavailable", nil
	}
	stringified, err := stringifyEvidenceValue(value)
	if err != nil {
		return nil, challengeID, "", err
	}
	return &stringified, challengeID, "", nil
}

func walkEvidenceValue(value any, segments []string) (any, bool) {
	current := value
	for _, segment := range segments {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

// resolveFileCaptureEvidence looks up a captured file or directory listing by
// its check key and returns the content as a string suitable for validators.
func resolveFileCaptureEvidence(checkKey string, evidence extractedEvidence) (*string, *uuid.UUID, string, error) {
	if capture, ok := evidence.capturedFiles[checkKey]; ok {
		if !capture.Exists {
			return nil, nil, fmt.Sprintf("captured file %q does not exist at path %q", checkKey, capture.Path), nil
		}
		return &capture.Content, nil, "", nil
	}
	if listing, ok := evidence.capturedDirListings[checkKey]; ok {
		encoded, err := json.Marshal(listing)
		if err != nil {
			return nil, nil, "", fmt.Errorf("marshal directory listing evidence: %w", err)
		}
		value := string(encoded)
		return &value, nil, "", nil
	}
	return nil, nil, fmt.Sprintf("file capture key %q is unavailable", checkKey), nil
}
