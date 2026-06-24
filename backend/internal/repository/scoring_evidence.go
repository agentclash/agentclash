package repository

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
	"github.com/agentclash/agentclash/backend/internal/scoring"
)

func BuildScoringEvidenceInputs(manifest json.RawMessage, inputSet *ChallengeInputSetExecutionContext) ([]scoring.EvidenceInput, error) {
	if inputSet == nil {
		return nil, nil
	}

	manifestAssets, err := manifestEvidenceAssets(manifest)
	if err != nil {
		return nil, err
	}

	inputs := make([]scoring.EvidenceInput, 0, len(inputSet.Cases))
	for _, item := range inputSet.Cases {
		caseInput, err := buildScoringEvidenceInput(item, manifestAssets)
		if err != nil {
			return nil, fmt.Errorf("build scoring evidence for case %s/%s: %w", item.ChallengeKey, item.CaseKey, err)
		}
		inputs = append(inputs, caseInput)
	}
	return inputs, nil
}

func buildScoringEvidenceInput(item ChallengeCaseExecutionContext, manifestAssets map[string]scoring.EvidenceArtifact) (scoring.EvidenceInput, error) {
	inputValues, err := caseEvidenceValues(item.Inputs)
	if err != nil {
		return scoring.EvidenceInput{}, err
	}
	expectationValues, err := caseExpectationValues(item.Expectations)
	if err != nil {
		return scoring.EvidenceInput{}, err
	}

	return scoring.EvidenceInput{
		ChallengeIdentityID: item.ChallengeIdentityID,
		RegressionCaseID:    cloneUUIDPtr(item.RegressionCaseID),
		ChallengeKey:        item.ChallengeKey,
		CaseKey:             item.CaseKey,
		ItemKey:             item.ItemKey,
		Payload:             cloneJSON(item.Payload),
		Inputs:              inputValues,
		Expectations:        expectationValues,
		Artifacts:           caseEvidenceArtifacts(item, manifestAssets),
	}, nil
}

func manifestEvidenceAssets(manifest json.RawMessage) (map[string]scoring.EvidenceArtifact, error) {
	var decoded struct {
		Version struct {
			Assets []challengepack.AssetReference `json:"assets"`
		} `json:"version"`
	}
	if len(strings.TrimSpace(string(manifest))) == 0 {
		return map[string]scoring.EvidenceArtifact{}, nil
	}
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		return nil, fmt.Errorf("decode manifest assets: %w", err)
	}

	assets := make(map[string]scoring.EvidenceArtifact, len(decoded.Version.Assets))
	for _, asset := range decoded.Version.Assets {
		if asset.Key == "" {
			continue
		}
		assets[asset.Key] = scoring.EvidenceArtifact{
			Key:       asset.Key,
			Kind:      asset.Kind,
			Path:      asset.Path,
			MediaType: asset.MediaType,
		}
	}
	return assets, nil
}

func caseEvidenceArtifacts(item ChallengeCaseExecutionContext, manifestAssets map[string]scoring.EvidenceArtifact) map[string]scoring.EvidenceArtifact {
	artifacts := make(map[string]scoring.EvidenceArtifact)
	for _, asset := range item.Assets {
		if asset.Key == "" {
			continue
		}
		artifacts[asset.Key] = scoring.EvidenceArtifact{
			Key:       asset.Key,
			Kind:      asset.Kind,
			Path:      asset.Path,
			MediaType: asset.MediaType,
		}
	}
	for _, ref := range item.Artifacts {
		if artifact, ok := manifestAssets[ref.Key]; ok {
			artifacts[ref.Key] = artifact
		}
	}
	for _, input := range item.Inputs {
		if artifact, ok := manifestAssets[input.ArtifactKey]; ok {
			artifacts[input.ArtifactKey] = artifact
		}
	}
	for _, expectation := range item.Expectations {
		if artifact, ok := manifestAssets[expectation.ArtifactKey]; ok {
			artifacts[expectation.ArtifactKey] = artifact
		}
		if artifactKey, ok := evidenceArtifactKeyFromSource(expectation.Source); ok {
			if artifact, exists := manifestAssets[artifactKey]; exists {
				artifacts[artifactKey] = artifact
			}
		}
	}
	return artifacts
}

func caseEvidenceValues(inputs []challengepack.CaseInput) (map[string]scoring.EvidenceValue, error) {
	values := make(map[string]scoring.EvidenceValue, len(inputs))
	for _, input := range inputs {
		encoded, err := marshalEvidenceValue(input.Value)
		if err != nil {
			return nil, fmt.Errorf("marshal case input %q: %w", input.Key, err)
		}
		values[input.Key] = scoring.EvidenceValue{
			Kind:        input.Kind,
			Value:       encoded,
			ArtifactKey: input.ArtifactKey,
			Path:        input.Path,
		}
	}
	return values, nil
}

func caseExpectationValues(expectations []challengepack.CaseExpectation) (map[string]scoring.EvidenceValue, error) {
	values := make(map[string]scoring.EvidenceValue, len(expectations))
	for _, expectation := range expectations {
		encoded, err := marshalEvidenceValue(expectation.Value)
		if err != nil {
			return nil, fmt.Errorf("marshal case expectation %q: %w", expectation.Key, err)
		}
		values[expectation.Key] = scoring.EvidenceValue{
			Kind:        expectation.Kind,
			Value:       encoded,
			ArtifactKey: expectation.ArtifactKey,
			Source:      expectation.Source,
		}
	}
	return values, nil
}

func marshalEvidenceValue(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func evidenceArtifactKeyFromSource(source string) (string, bool) {
	trimmed := strings.TrimSpace(source)
	if !strings.HasPrefix(trimmed, "artifact:") {
		return "", false
	}
	key := strings.TrimSpace(strings.TrimPrefix(trimmed, "artifact:"))
	if key == "" {
		return "", false
	}
	return key, true
}
