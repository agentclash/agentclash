package challengepack

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
)

var (
	envVarKeyPattern    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	aptPackagePattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9.+\-]+$`)
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
