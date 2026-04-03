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
}

type InputSetDefinition struct {
	Key         string                `yaml:"key" json:"key"`
	Name        string                `yaml:"name" json:"name"`
	Description *string               `yaml:"description,omitempty" json:"description,omitempty"`
	Items       []InputItemDefinition `yaml:"items" json:"items"`
}

type InputItemDefinition struct {
	ChallengeKey string           `yaml:"challenge_key" json:"challenge_key"`
	ItemKey      string           `yaml:"item_key" json:"item_key"`
	Payload      map[string]any   `yaml:"payload,omitempty" json:"payload,omitempty"`
	Assets       []AssetReference `yaml:"assets,omitempty" json:"assets,omitempty"`
}

type AssetReference struct {
	Key       string `yaml:"key" json:"key"`
	Path      string `yaml:"path" json:"path"`
	MediaType string `yaml:"media_type,omitempty" json:"media_type,omitempty"`
}

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
		bundle.InputSets[i].Items = append([]InputItemDefinition(nil), bundle.InputSets[i].Items...)
		for itemIndex := range bundle.InputSets[i].Items {
			bundle.InputSets[i].Items[itemIndex].ChallengeKey = strings.TrimSpace(bundle.InputSets[i].Items[itemIndex].ChallengeKey)
			bundle.InputSets[i].Items[itemIndex].ItemKey = strings.TrimSpace(bundle.InputSets[i].Items[itemIndex].ItemKey)
			bundle.InputSets[i].Items[itemIndex].Assets = normalizeAssets(bundle.InputSets[i].Items[itemIndex].Assets)
			if bundle.InputSets[i].Items[itemIndex].Payload == nil {
				bundle.InputSets[i].Items[itemIndex].Payload = map[string]any{}
			}
		}
	}

	bundle.Version.Assets = normalizeAssets(bundle.Version.Assets)
}

func normalizeAssets(assets []AssetReference) []AssetReference {
	normalized := append([]AssetReference(nil), assets...)
	for i := range normalized {
		normalized[i].Key = strings.TrimSpace(normalized[i].Key)
		normalized[i].Path = strings.TrimSpace(normalized[i].Path)
		normalized[i].MediaType = strings.TrimSpace(normalized[i].MediaType)
	}
	return normalized
}
