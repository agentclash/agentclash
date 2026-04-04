package challengepack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"gopkg.in/yaml.v3"
)

type Bundle struct {
	Pack       PackMetadata          `yaml:"pack" json:"pack"`
	Version    VersionMetadata       `yaml:"version" json:"version"`
	Challenges []ChallengeDefinition `yaml:"challenges" json:"challenges"`
	InputSets  []InputSetDefinition  `yaml:"input_sets" json:"input_sets"`
}

type PackMetadata struct {
	Slug        string  `yaml:"slug" json:"slug"`
	Name        string  `yaml:"name" json:"name"`
	Family      string  `yaml:"family" json:"family"`
	Description *string `yaml:"description,omitempty" json:"description,omitempty"`
}

type VersionMetadata struct {
	Number         int32                  `yaml:"number" json:"number"`
	ToolPolicy     map[string]any         `yaml:"tool_policy,omitempty" json:"tool_policy,omitempty"`
	Filesystem     map[string]any         `yaml:"filesystem,omitempty" json:"filesystem,omitempty"`
	EvaluationSpec scoring.EvaluationSpec `yaml:"evaluation_spec" json:"evaluation_spec"`
	Assets         []AssetReference       `yaml:"assets,omitempty" json:"assets,omitempty"`
}

type ChallengeDefinition struct {
	Key          string           `yaml:"key" json:"key"`
	Title        string           `yaml:"title" json:"title"`
	Category     string           `yaml:"category" json:"category"`
	Difficulty   string           `yaml:"difficulty" json:"difficulty"`
	Instructions string           `yaml:"instructions,omitempty" json:"instructions,omitempty"`
	Definition   map[string]any   `yaml:"definition,omitempty" json:"definition,omitempty"`
	Assets       []AssetReference `yaml:"assets,omitempty" json:"assets,omitempty"`
	ArtifactRefs []ArtifactRef    `yaml:"artifact_refs,omitempty" json:"artifact_refs,omitempty"`
}

type InputSetDefinition struct {
	Key         string                 `yaml:"key" json:"key"`
	Name        string                 `yaml:"name" json:"name"`
	Description *string                `yaml:"description,omitempty" json:"description,omitempty"`
	Cases       []CaseDefinition       `yaml:"cases,omitempty" json:"cases,omitempty"`
	Items       []LegacyItemDefinition `yaml:"items,omitempty" json:"-"`
}

type CaseDefinition struct {
	ChallengeKey string            `yaml:"challenge_key" json:"challenge_key"`
	CaseKey      string            `yaml:"case_key,omitempty" json:"case_key,omitempty"`
	ItemKey      string            `yaml:"item_key,omitempty" json:"item_key,omitempty"`
	Payload      map[string]any    `yaml:"payload,omitempty" json:"payload,omitempty"`
	Inputs       []CaseInput       `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Expectations []CaseExpectation `yaml:"expectations,omitempty" json:"expectations,omitempty"`
	Artifacts    []ArtifactRef     `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
	Assets       []AssetReference  `yaml:"assets,omitempty" json:"assets,omitempty"`
}

type AssetReference struct {
	Key       string `yaml:"key" json:"key"`
	Kind      string `yaml:"kind,omitempty" json:"kind,omitempty"`
	Path      string `yaml:"path" json:"path"`
	MediaType string `yaml:"media_type,omitempty" json:"media_type,omitempty"`
}

type ArtifactRef struct {
	Key string `yaml:"key" json:"key"`
}

type CaseInput struct {
	Key         string `yaml:"key" json:"key"`
	Kind        string `yaml:"kind" json:"kind"`
	Value       any    `yaml:"value,omitempty" json:"value,omitempty"`
	ArtifactKey string `yaml:"artifact_key,omitempty" json:"artifact_key,omitempty"`
	Path        string `yaml:"path,omitempty" json:"path,omitempty"`
}

type CaseExpectation struct {
	Key         string `yaml:"key" json:"key"`
	Kind        string `yaml:"kind" json:"kind"`
	Value       any    `yaml:"value,omitempty" json:"value,omitempty"`
	ArtifactKey string `yaml:"artifact_key,omitempty" json:"artifact_key,omitempty"`
	Source      string `yaml:"source,omitempty" json:"source,omitempty"`
}

type StoredCaseDocument struct {
	SchemaVersion int32             `json:"schema_version,omitempty"`
	CaseKey       string            `json:"case_key,omitempty"`
	Payload       map[string]any    `json:"payload,omitempty"`
	Inputs        []CaseInput       `json:"inputs,omitempty"`
	Expectations  []CaseExpectation `json:"expectations,omitempty"`
	Artifacts     []ArtifactRef     `json:"artifacts,omitempty"`
	Assets        []AssetReference  `json:"assets,omitempty"`
}

type LegacyItemDefinition struct {
	ChallengeKey string           `yaml:"challenge_key"`
	ItemKey      string           `yaml:"item_key"`
	Payload      map[string]any   `yaml:"payload,omitempty"`
	Assets       []AssetReference `yaml:"assets,omitempty"`
}

type InputItemDefinition = LegacyItemDefinition

func ParseYAML(data []byte) (Bundle, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Bundle{}, ValidationErrors{{Field: "bundle", Message: "is required"}}
	}

	type rawVersionMetadata struct {
		Number         int32            `yaml:"number"`
		ToolPolicy     map[string]any   `yaml:"tool_policy,omitempty"`
		Filesystem     map[string]any   `yaml:"filesystem,omitempty"`
		EvaluationSpec map[string]any   `yaml:"evaluation_spec"`
		Assets         []AssetReference `yaml:"assets,omitempty"`
	}
	type rawBundle struct {
		Pack       PackMetadata          `yaml:"pack"`
		Version    rawVersionMetadata    `yaml:"version"`
		Challenges []ChallengeDefinition `yaml:"challenges"`
		InputSets  []InputSetDefinition  `yaml:"input_sets"`
	}

	var raw rawBundle
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Bundle{}, fmt.Errorf("decode challenge-pack bundle yaml: %w", err)
	}

	var evaluationSpec scoring.EvaluationSpec
	if len(raw.Version.EvaluationSpec) > 0 {
		encoded, err := json.Marshal(raw.Version.EvaluationSpec)
		if err != nil {
			return Bundle{}, fmt.Errorf("marshal yaml evaluation spec: %w", err)
		}
		if err := json.Unmarshal(encoded, &evaluationSpec); err != nil {
			return Bundle{}, fmt.Errorf("decode evaluation spec from yaml: %w", err)
		}
	}

	bundle := Bundle{
		Pack: raw.Pack,
		Version: VersionMetadata{
			Number:         raw.Version.Number,
			ToolPolicy:     raw.Version.ToolPolicy,
			Filesystem:     raw.Version.Filesystem,
			EvaluationSpec: evaluationSpec,
			Assets:         raw.Version.Assets,
		},
		Challenges: raw.Challenges,
		InputSets:  raw.InputSets,
	}

	normalizeBundle(&bundle)
	if err := ValidateBundle(bundle); err != nil {
		return Bundle{}, err
	}

	return bundle, nil
}

func ManifestJSON(bundle Bundle) (json.RawMessage, error) {
	normalized := bundle
	normalizeBundle(&normalized)
	if err := ValidateBundle(normalized); err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(map[string]any{
		"schema_version":  1,
		"pack":            normalized.Pack,
		"version":         map[string]any{"number": normalized.Version.Number, "assets": normalized.Version.Assets},
		"tool_policy":     normalized.Version.ToolPolicy,
		"filesystem":      normalized.Version.Filesystem,
		"evaluation_spec": normalized.Version.EvaluationSpec,
		"challenges":      normalized.Challenges,
		"input_sets":      normalized.InputSets,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal challenge-pack manifest: %w", err)
	}

	return encoded, nil
}

func normalizeBundle(bundle *Bundle) {
	bundle.Pack.Slug = strings.TrimSpace(bundle.Pack.Slug)
	bundle.Pack.Name = strings.TrimSpace(bundle.Pack.Name)
	bundle.Pack.Family = strings.TrimSpace(bundle.Pack.Family)
	bundle.Challenges = append([]ChallengeDefinition(nil), bundle.Challenges...)
	bundle.InputSets = append([]InputSetDefinition(nil), bundle.InputSets...)
	bundle.Version.Assets = append([]AssetReference(nil), bundle.Version.Assets...)

	for i := range bundle.Challenges {
		bundle.Challenges[i].Key = strings.TrimSpace(bundle.Challenges[i].Key)
		bundle.Challenges[i].Title = strings.TrimSpace(bundle.Challenges[i].Title)
		bundle.Challenges[i].Category = strings.TrimSpace(bundle.Challenges[i].Category)
		bundle.Challenges[i].Difficulty = strings.TrimSpace(bundle.Challenges[i].Difficulty)
		bundle.Challenges[i].Instructions = strings.TrimSpace(bundle.Challenges[i].Instructions)
		bundle.Challenges[i].Assets = normalizeAssets(bundle.Challenges[i].Assets)
		bundle.Challenges[i].ArtifactRefs = normalizeArtifactRefs(bundle.Challenges[i].ArtifactRefs)
		if bundle.Challenges[i].Definition == nil {
			bundle.Challenges[i].Definition = map[string]any{}
		}
		if bundle.Challenges[i].Instructions != "" {
			if _, exists := bundle.Challenges[i].Definition["instructions"]; !exists {
				bundle.Challenges[i].Definition["instructions"] = bundle.Challenges[i].Instructions
			}
		}
	}

	for i := range bundle.InputSets {
		bundle.InputSets[i].Key = strings.TrimSpace(bundle.InputSets[i].Key)
		bundle.InputSets[i].Name = strings.TrimSpace(bundle.InputSets[i].Name)
		if len(bundle.InputSets[i].Cases) == 0 && len(bundle.InputSets[i].Items) > 0 {
			bundle.InputSets[i].Cases = make([]CaseDefinition, 0, len(bundle.InputSets[i].Items))
			for _, item := range bundle.InputSets[i].Items {
				bundle.InputSets[i].Cases = append(bundle.InputSets[i].Cases, CaseDefinition{
					ChallengeKey: item.ChallengeKey,
					CaseKey:      item.ItemKey,
					ItemKey:      item.ItemKey,
					Payload:      cloneObject(item.Payload),
					Assets:       normalizeAssets(item.Assets),
				})
			}
		}
		bundle.InputSets[i].Cases = append([]CaseDefinition(nil), bundle.InputSets[i].Cases...)
		bundle.InputSets[i].Items = nil
		for caseIndex := range bundle.InputSets[i].Cases {
			bundle.InputSets[i].Cases[caseIndex].ChallengeKey = strings.TrimSpace(bundle.InputSets[i].Cases[caseIndex].ChallengeKey)
			bundle.InputSets[i].Cases[caseIndex].CaseKey = strings.TrimSpace(bundle.InputSets[i].Cases[caseIndex].CaseKey)
			bundle.InputSets[i].Cases[caseIndex].ItemKey = strings.TrimSpace(bundle.InputSets[i].Cases[caseIndex].ItemKey)
			if bundle.InputSets[i].Cases[caseIndex].CaseKey == "" {
				bundle.InputSets[i].Cases[caseIndex].CaseKey = bundle.InputSets[i].Cases[caseIndex].ItemKey
			}
			if bundle.InputSets[i].Cases[caseIndex].ItemKey == "" {
				bundle.InputSets[i].Cases[caseIndex].ItemKey = bundle.InputSets[i].Cases[caseIndex].CaseKey
			}
			bundle.InputSets[i].Cases[caseIndex].Assets = normalizeAssets(bundle.InputSets[i].Cases[caseIndex].Assets)
			bundle.InputSets[i].Cases[caseIndex].Artifacts = normalizeArtifactRefs(bundle.InputSets[i].Cases[caseIndex].Artifacts)
			bundle.InputSets[i].Cases[caseIndex].Inputs = normalizeCaseInputs(bundle.InputSets[i].Cases[caseIndex].Inputs)
			bundle.InputSets[i].Cases[caseIndex].Expectations = normalizeCaseExpectations(bundle.InputSets[i].Cases[caseIndex].Expectations)
			if bundle.InputSets[i].Cases[caseIndex].Payload == nil {
				bundle.InputSets[i].Cases[caseIndex].Payload = map[string]any{}
			}
		}
	}

	bundle.Version.Assets = normalizeAssets(bundle.Version.Assets)
}

func normalizeAssets(assets []AssetReference) []AssetReference {
	normalized := append([]AssetReference(nil), assets...)
	for i := range normalized {
		normalized[i].Key = strings.TrimSpace(normalized[i].Key)
		normalized[i].Kind = strings.TrimSpace(normalized[i].Kind)
		normalized[i].Path = strings.TrimSpace(normalized[i].Path)
		normalized[i].MediaType = strings.TrimSpace(normalized[i].MediaType)
	}
	return normalized
}

func normalizeArtifactRefs(refs []ArtifactRef) []ArtifactRef {
	normalized := append([]ArtifactRef(nil), refs...)
	for i := range normalized {
		normalized[i].Key = strings.TrimSpace(normalized[i].Key)
	}
	return normalized
}

func normalizeCaseInputs(inputs []CaseInput) []CaseInput {
	normalized := append([]CaseInput(nil), inputs...)
	for i := range normalized {
		normalized[i].Key = strings.TrimSpace(normalized[i].Key)
		normalized[i].Kind = strings.TrimSpace(normalized[i].Kind)
		normalized[i].ArtifactKey = strings.TrimSpace(normalized[i].ArtifactKey)
		normalized[i].Path = strings.TrimSpace(normalized[i].Path)
	}
	return normalized
}

func normalizeCaseExpectations(expectations []CaseExpectation) []CaseExpectation {
	normalized := append([]CaseExpectation(nil), expectations...)
	for i := range normalized {
		normalized[i].Key = strings.TrimSpace(normalized[i].Key)
		normalized[i].Kind = strings.TrimSpace(normalized[i].Kind)
		normalized[i].ArtifactKey = strings.TrimSpace(normalized[i].ArtifactKey)
		normalized[i].Source = strings.TrimSpace(normalized[i].Source)
	}
	return normalized
}

func cloneObject(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = deepCloneValue(item)
	}
	return cloned
}

func deepCloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, item := range typed {
			cloned[key] = deepCloneValue(item)
		}
		return cloned
	case []any:
		cloned := make([]any, 0, len(typed))
		for _, item := range typed {
			cloned = append(cloned, deepCloneValue(item))
		}
		return cloned
	default:
		return typed
	}
}

func (c CaseDefinition) EffectiveKey() string {
	if strings.TrimSpace(c.CaseKey) != "" {
		return strings.TrimSpace(c.CaseKey)
	}
	return strings.TrimSpace(c.ItemKey)
}

func (c CaseDefinition) IsLegacyPayloadOnly() bool {
	// Legacy packs only used raw payload blobs keyed by item_key. We keep that
	// storage shape for backward compatibility when no generalized case fields
	// are present.
	return len(c.Inputs) == 0 && len(c.Expectations) == 0 && len(c.Artifacts) == 0 && len(c.Assets) == 0
}

func (c CaseDefinition) StoredPayload() (json.RawMessage, error) {
	if c.IsLegacyPayloadOnly() {
		if c.Payload == nil {
			return json.RawMessage(`{}`), nil
		}
		return json.Marshal(c.Payload)
	}

	return json.Marshal(StoredCaseDocument{
		SchemaVersion: 1,
		CaseKey:       c.EffectiveKey(),
		Payload:       cloneObject(c.Payload),
		Inputs:        append([]CaseInput(nil), c.Inputs...),
		Expectations:  append([]CaseExpectation(nil), c.Expectations...),
		Artifacts:     append([]ArtifactRef(nil), c.Artifacts...),
		Assets:        normalizeAssets(c.Assets),
	})
}
