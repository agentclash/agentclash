package challengepack

import (
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/jsonschema-go/jsonschema"
)

var (
	envVarKeyPattern  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	aptPackagePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.+\-]+$`)
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	parts := make([]string, 0, len(e))
	for _, item := range e {
		parts = append(parts, item.Error())
	}
	return strings.Join(parts, "; ")
}

func ValidateBundle(bundle Bundle) error {
	var errs ValidationErrors

	if bundle.Pack.Slug == "" {
		errs = append(errs, ValidationError{Field: "pack.slug", Message: "is required"})
	}
	if bundle.Pack.Name == "" {
		errs = append(errs, ValidationError{Field: "pack.name", Message: "is required"})
	}
	if bundle.Pack.Family == "" {
		errs = append(errs, ValidationError{Field: "pack.family", Message: "is required"})
	}
	if bundle.Version.Number <= 0 {
		errs = append(errs, ValidationError{Field: "version.number", Message: "must be greater than 0"})
	}
	if len(bundle.Challenges) == 0 {
		errs = append(errs, ValidationError{Field: "challenges", Message: "must contain at least one challenge"})
	}

	challengeKeys := map[string]struct{}{}
	versionAssetKeys := map[string]struct{}{}
	for _, asset := range bundle.Version.Assets {
		if asset.Key != "" {
			versionAssetKeys[asset.Key] = struct{}{}
		}
	}
	for i, challenge := range bundle.Challenges {
		path := fmt.Sprintf("challenges[%d]", i)
		if challenge.Key == "" {
			errs = append(errs, ValidationError{Field: path + ".key", Message: "is required"})
		} else {
			if _, exists := challengeKeys[challenge.Key]; exists {
				errs = append(errs, ValidationError{Field: path + ".key", Message: "must be unique"})
			}
			challengeKeys[challenge.Key] = struct{}{}
		}
		if challenge.Title == "" {
			errs = append(errs, ValidationError{Field: path + ".title", Message: "is required"})
		}
		if challenge.Category == "" {
			errs = append(errs, ValidationError{Field: path + ".category", Message: "is required"})
		}
		switch challenge.Difficulty {
		case "easy", "medium", "hard", "expert":
		default:
			errs = append(errs, ValidationError{Field: path + ".difficulty", Message: "must be one of easy, medium, hard, expert"})
		}
		errs = append(errs, validateAssets(path+".assets", challenge.Assets)...)
		errs = append(errs, validateArtifactRefs(path+".artifact_refs", challenge.ArtifactRefs, versionAssetKeys)...)
	}

	inputSetKeys := map[string]struct{}{}
	for i, inputSet := range bundle.InputSets {
		path := fmt.Sprintf("input_sets[%d]", i)
		if inputSet.Key == "" {
			errs = append(errs, ValidationError{Field: path + ".key", Message: "is required"})
		} else {
			if _, exists := inputSetKeys[inputSet.Key]; exists {
				errs = append(errs, ValidationError{Field: path + ".key", Message: "must be unique"})
			}
			inputSetKeys[inputSet.Key] = struct{}{}
		}
		if inputSet.Name == "" {
			errs = append(errs, ValidationError{Field: path + ".name", Message: "is required"})
		}
		if len(inputSet.Cases) == 0 {
			errs = append(errs, ValidationError{Field: path + ".cases", Message: "must contain at least one case"})
		}

		caseKeys := map[string]struct{}{}
		for caseIndex, item := range inputSet.Cases {
			itemPath := fmt.Sprintf("%s.cases[%d]", path, caseIndex)
			if item.ChallengeKey == "" {
				errs = append(errs, ValidationError{Field: itemPath + ".challenge_key", Message: "is required"})
			} else {
				if _, exists := challengeKeys[item.ChallengeKey]; !exists {
					errs = append(errs, ValidationError{Field: itemPath + ".challenge_key", Message: "must reference a declared challenge"})
				}
			}
			caseKey := item.CaseKey
			if caseKey == "" {
				caseKey = item.ItemKey
			}
			if caseKey == "" {
				errs = append(errs, ValidationError{Field: itemPath + ".case_key", Message: "is required"})
			} else {
				composite := item.ChallengeKey + "\x00" + caseKey
				if _, exists := caseKeys[composite]; exists {
					errs = append(errs, ValidationError{Field: itemPath + ".case_key", Message: "must be unique per challenge in the input set"})
				}
				caseKeys[composite] = struct{}{}
			}
			errs = append(errs, validateAssets(itemPath+".assets", item.Assets)...)
			errs = append(errs, validateArtifactRefs(itemPath+".artifacts", item.Artifacts, versionAssetKeys)...)
			errs = append(errs, validateCaseInputs(itemPath+".inputs", item.Inputs, versionAssetKeys)...)
			errs = append(errs, validateCaseExpectations(itemPath+".expectations", item.Expectations, item.Inputs, versionAssetKeys)...)
		}
	}

	errs = append(errs, validateToolsConfig("tools", bundle.Tools)...)

	if bundle.Version.Sandbox != nil {
		errs = append(errs, validateSandboxConfig("version.sandbox", bundle.Version.Sandbox)...)
	}

	if err := scoring.ValidateEvaluationSpec(bundle.Version.EvaluationSpec); err != nil {
		if scoringErrs, ok := err.(scoring.ValidationErrors); ok {
			for _, item := range scoringErrs {
				errs = append(errs, ValidationError{Field: "version." + item.Field, Message: item.Message})
			}
		} else {
			errs = append(errs, ValidationError{Field: "version.evaluation_spec", Message: err.Error()})
		}
	}
	errs = append(errs, validateAssets("version.assets", bundle.Version.Assets)...)

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validateToolsConfig(path string, tools map[string]any) ValidationErrors {
	if len(tools) == 0 {
		return nil
	}

	encoded, err := json.Marshal(tools)
	if err != nil {
		return ValidationErrors{{
			Field:   path,
			Message: fmt.Sprintf("must be JSON-serializable: %v", err),
		}}
	}

	var decoded struct {
		Custom []struct {
			Name           string          `json:"name"`
			Parameters     json.RawMessage `json:"parameters"`
			Implementation json.RawMessage `json:"implementation"`
		} `json:"custom"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return ValidationErrors{{
			Field:   path,
			Message: fmt.Sprintf("must be a valid tools object: %v", err),
		}}
	}

	var errs ValidationErrors
	for i, custom := range decoded.Custom {
		toolPath := fmt.Sprintf("%s.custom[%d]", path, i)
		errs = append(errs, validateComposedToolConfig(toolPath, custom.Name, custom.Parameters, custom.Implementation)...)
	}
	return errs
}

func validateComposedToolConfig(path string, name string, parameters json.RawMessage, implementationRaw json.RawMessage) ValidationErrors {
	var errs ValidationErrors

	var implementation struct {
		Type      string          `json:"type"`
		Primitive string          `json:"primitive"`
		Args      json.RawMessage `json:"args"`
	}
	if err := json.Unmarshal(implementationRaw, &implementation); err != nil {
		return ValidationErrors{{
			Field:   path + ".implementation",
			Message: fmt.Sprintf("must be valid JSON: %v", err),
		}}
	}

	if strings.EqualFold(strings.TrimSpace(implementation.Type), "mock") {
		return nil
	}

	name = strings.TrimSpace(name)
	primitive := strings.TrimSpace(implementation.Primitive)
	if primitive == "" {
		errs = append(errs, ValidationError{Field: path + ".implementation.primitive", Message: "is required"})
	}
	if name != "" && primitive == name {
		errs = append(errs, ValidationError{Field: path + ".implementation.primitive", Message: "cannot reference the tool's own name"})
	}

	if len(parameters) == 0 {
		parameters = json.RawMessage(`{"type":"object","additionalProperties":false}`)
	}
	if err := validateToolParameterSchema(parameters); err != nil {
		errs = append(errs, ValidationError{Field: path + ".parameters", Message: err.Error()})
	}
	declaredParams, err := declaredToolParameters(parameters)
	if err != nil {
		errs = append(errs, ValidationError{Field: path + ".parameters", Message: fmt.Sprintf("must be valid JSON: %v", err)})
	}

	if len(implementation.Args) == 0 {
		errs = append(errs, ValidationError{Field: path + ".implementation.args", Message: "is required"})
		return errs
	}

	var args map[string]any
	if err := json.Unmarshal(implementation.Args, &args); err != nil {
		errs = append(errs, ValidationError{Field: path + ".implementation.args", Message: fmt.Sprintf("must be a JSON object: %v", err)})
		return errs
	}

	errs = append(errs, validateTemplateSyntax(path+".implementation.args", args)...)
	errs = append(errs, validateTemplateReferences(path+".implementation.args", args, declaredParams)...)
	return errs
}

func validateToolParameterSchema(parameters json.RawMessage) error {
	var schema jsonschema.Schema
	if err := json.Unmarshal(parameters, &schema); err != nil {
		return fmt.Errorf("must be a valid JSON Schema: %w", err)
	}
	if _, err := schema.Resolve(nil); err != nil {
		return fmt.Errorf("must resolve as a valid JSON Schema: %w", err)
	}
	return nil
}

func declaredToolParameters(parameters json.RawMessage) (map[string]struct{}, error) {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(parameters, &schema); err != nil {
		return nil, err
	}
	declared := make(map[string]struct{}, len(schema.Properties))
	for key := range schema.Properties {
		declared[strings.TrimSpace(key)] = struct{}{}
	}
	return declared, nil
}

func validateTemplateSyntax(path string, value any) ValidationErrors {
	switch v := value.(type) {
	case string:
		if err := validatePlaceholderSyntax(path, v); err != nil {
			return ValidationErrors{{Field: path, Message: err.Error()}}
		}
	case map[string]any:
		var errs ValidationErrors
		for key, child := range v {
			errs = append(errs, validateTemplateSyntax(path+"."+key, child)...)
		}
		return errs
	case []any:
		var errs ValidationErrors
		for i, child := range v {
			errs = append(errs, validateTemplateSyntax(fmt.Sprintf("%s[%d]", path, i), child)...)
		}
		return errs
	}
	return nil
}

func validateTemplateReferences(path string, value any, declaredParams map[string]struct{}) ValidationErrors {
	switch v := value.(type) {
	case string:
		return validateTemplateStringReferences(path, v, declaredParams)
	case map[string]any:
		var errs ValidationErrors
		for key, child := range v {
			errs = append(errs, validateTemplateReferences(path+"."+key, child, declaredParams)...)
		}
		return errs
	case []any:
		var errs ValidationErrors
		for i, child := range v {
			errs = append(errs, validateTemplateReferences(fmt.Sprintf("%s[%d]", path, i), child, declaredParams)...)
		}
		return errs
	}
	return nil
}

func validateTemplateStringReferences(path string, value string, declaredParams map[string]struct{}) ValidationErrors {
	if err := validatePlaceholderSyntax(path, value); err != nil {
		return ValidationErrors{{Field: path, Message: err.Error()}}
	}

	rest := value
	for {
		idx := strings.Index(rest, "${")
		if idx == -1 {
			return nil
		}
		after := rest[idx+2:]
		closeIdx := strings.Index(after, "}")
		expr := after[:closeIdx]

		switch {
		case expr == "parameters":
		case strings.HasPrefix(expr, "secrets.") && strings.TrimSpace(strings.TrimPrefix(expr, "secrets.")) != "":
		default:
			root := strings.Split(expr, ".")[0]
			if _, ok := declaredParams[root]; !ok {
				return ValidationErrors{{
					Field:   path,
					Message: fmt.Sprintf("contains unknown placeholder %q", "${"+expr+"}"),
				}}
			}
		}

		rest = after[closeIdx+1:]
	}
}

func validatePlaceholderSyntax(path string, value string) error {
	rest := value
	for {
		idx := strings.Index(rest, "${")
		if idx == -1 {
			return nil
		}
		after := rest[idx+2:]
		closeIdx := strings.Index(after, "}")
		if closeIdx == -1 {
			return fmt.Errorf("contains an unclosed placeholder")
		}
		if strings.TrimSpace(after[:closeIdx]) == "" {
			return fmt.Errorf("contains an empty placeholder")
		}
		rest = after[closeIdx+1:]
	}
}

func validateSandboxConfig(path string, config *SandboxConfig) ValidationErrors {
	var errs ValidationErrors

	for i, cidr := range config.NetworkAllowlist {
		cidrPath := fmt.Sprintf("%s.network_allowlist[%d]", path, i)
		_, _, err := net.ParseCIDR(cidr)
		if err != nil {
			errs = append(errs, ValidationError{Field: cidrPath, Message: "must be a valid CIDR (e.g. 10.0.0.0/8)"})
		}
	}

	for key := range config.EnvVars {
		if !envVarKeyPattern.MatchString(key) {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("%s.env_vars[%s]", path, key),
				Message: "key must match [A-Za-z_][A-Za-z0-9_]*",
			})
		}
	}

	for i, pkg := range config.AdditionalPackages {
		pkgPath := fmt.Sprintf("%s.additional_packages[%d]", path, i)
		if !aptPackagePattern.MatchString(pkg) {
			errs = append(errs, ValidationError{Field: pkgPath, Message: "must be a valid apt package name"})
		}
	}

	return errs
}

func validateAssets(path string, assets []AssetReference) ValidationErrors {
	var errs ValidationErrors
	seen := map[string]struct{}{}
	for i, asset := range assets {
		assetPath := fmt.Sprintf("%s[%d]", path, i)
		if asset.Key == "" {
			errs = append(errs, ValidationError{Field: assetPath + ".key", Message: "is required"})
		} else {
			if _, exists := seen[asset.Key]; exists {
				errs = append(errs, ValidationError{Field: assetPath + ".key", Message: "must be unique"})
			}
			seen[asset.Key] = struct{}{}
		}
		if asset.Path == "" {
			errs = append(errs, ValidationError{Field: assetPath + ".path", Message: "is required"})
		}
	}
	return errs
}

func validateArtifactRefs(path string, refs []ArtifactRef, validKeys map[string]struct{}) ValidationErrors {
	var errs ValidationErrors
	seen := map[string]struct{}{}
	for i, ref := range refs {
		refPath := fmt.Sprintf("%s[%d]", path, i)
		if ref.Key == "" {
			errs = append(errs, ValidationError{Field: refPath + ".key", Message: "is required"})
			continue
		}
		if _, exists := seen[ref.Key]; exists {
			errs = append(errs, ValidationError{Field: refPath + ".key", Message: "must be unique"})
		}
		seen[ref.Key] = struct{}{}
		if _, exists := validKeys[ref.Key]; !exists {
			errs = append(errs, ValidationError{Field: refPath + ".key", Message: "must reference a declared version asset"})
		}
	}
	return errs
}

func validateCaseInputs(path string, inputs []CaseInput, validAssets map[string]struct{}) ValidationErrors {
	var errs ValidationErrors
	seen := map[string]struct{}{}
	for i, input := range inputs {
		inputPath := fmt.Sprintf("%s[%d]", path, i)
		if input.Key == "" {
			errs = append(errs, ValidationError{Field: inputPath + ".key", Message: "is required"})
		} else {
			if _, exists := seen[input.Key]; exists {
				errs = append(errs, ValidationError{Field: inputPath + ".key", Message: "must be unique"})
			}
			seen[input.Key] = struct{}{}
		}
		if input.Kind == "" {
			errs = append(errs, ValidationError{Field: inputPath + ".kind", Message: "is required"})
		}
		if input.ArtifactKey != "" {
			if _, exists := validAssets[input.ArtifactKey]; !exists {
				errs = append(errs, ValidationError{Field: inputPath + ".artifact_key", Message: "must reference a declared version asset"})
			}
		}
	}
	return errs
}

func validateCaseExpectations(path string, expectations []CaseExpectation, inputs []CaseInput, validAssets map[string]struct{}) ValidationErrors {
	var errs ValidationErrors
	seen := map[string]struct{}{}
	validInputs := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		if input.Key != "" {
			validInputs[input.Key] = struct{}{}
		}
	}

	for i, expectation := range expectations {
		expectationPath := fmt.Sprintf("%s[%d]", path, i)
		if expectation.Key == "" {
			errs = append(errs, ValidationError{Field: expectationPath + ".key", Message: "is required"})
		} else {
			if _, exists := seen[expectation.Key]; exists {
				errs = append(errs, ValidationError{Field: expectationPath + ".key", Message: "must be unique"})
			}
			seen[expectation.Key] = struct{}{}
		}
		if expectation.Kind == "" {
			errs = append(errs, ValidationError{Field: expectationPath + ".kind", Message: "is required"})
		}
		if expectation.ArtifactKey != "" {
			if _, exists := validAssets[expectation.ArtifactKey]; !exists {
				errs = append(errs, ValidationError{Field: expectationPath + ".artifact_key", Message: "must reference a declared version asset"})
			}
		}
		switch {
		case expectation.Source == "":
		case strings.HasPrefix(expectation.Source, "input:"):
			inputKey := strings.TrimSpace(strings.TrimPrefix(expectation.Source, "input:"))
			if inputKey == "" {
				errs = append(errs, ValidationError{Field: expectationPath + ".source", Message: "must reference a non-empty input key"})
			} else if _, exists := validInputs[inputKey]; !exists {
				errs = append(errs, ValidationError{Field: expectationPath + ".source", Message: "must reference a declared case input"})
			}
		case strings.HasPrefix(expectation.Source, "artifact:"):
			artifactKey := strings.TrimSpace(strings.TrimPrefix(expectation.Source, "artifact:"))
			if artifactKey == "" {
				errs = append(errs, ValidationError{Field: expectationPath + ".source", Message: "must reference a non-empty artifact key"})
			} else if _, exists := validAssets[artifactKey]; !exists {
				errs = append(errs, ValidationError{Field: expectationPath + ".source", Message: "must reference a declared version asset"})
			}
		default:
			errs = append(errs, ValidationError{Field: expectationPath + ".source", Message: "must start with input: or artifact:"})
		}
	}

	return errs
}
