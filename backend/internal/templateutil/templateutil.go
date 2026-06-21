package templateutil

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

type ValidationError struct {
	Path    string
	Message string
}

func (e ValidationError) Error() string {
	if strings.TrimSpace(e.Path) == "" {
		return e.Message
	}
	return fmt.Sprintf("%s %s", e.Path, e.Message)
}

func ValidateTemplatePlaceholders(value any, path string) error {
	switch v := value.(type) {
	case string:
		return ValidatePlaceholderSyntax(v, path)
	case map[string]any:
		for key, child := range v {
			childPath := path + "." + key
			if err := ValidateTemplatePlaceholders(child, childPath); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range v {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			if err := ValidateTemplatePlaceholders(child, childPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func ValidatePlaceholderSyntax(value string, path string) error {
	rest := value
	for {
		idx := strings.Index(rest, "${")
		if idx == -1 {
			return nil
		}
		after := rest[idx+2:]
		closeIdx := strings.Index(after, "}")
		if closeIdx == -1 {
			return ValidationError{
				Path:    path,
				Message: fmt.Sprintf("contains an unclosed placeholder: %q", rest[idx:]),
			}
		}
		if strings.TrimSpace(after[:closeIdx]) == "" {
			return ValidationError{
				Path:    path,
				Message: fmt.Sprintf("contains an empty placeholder: %q", rest[idx:idx+2+closeIdx+1]),
			}
		}
		rest = after[closeIdx+1:]
	}
}

func ValidateTemplateReferences(value any, path string, declaredParams map[string]struct{}) error {
	switch v := value.(type) {
	case string:
		return validateTemplateStringReferences(v, path, declaredParams)
	case map[string]any:
		for key, child := range v {
			childPath := path + "." + key
			if err := ValidateTemplateReferences(child, childPath, declaredParams); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range v {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			if err := ValidateTemplateReferences(child, childPath, declaredParams); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTemplateStringReferences(value string, path string, declaredParams map[string]struct{}) error {
	if err := ValidatePlaceholderSyntax(value, path); err != nil {
		return err
	}

	rest := value
	for {
		idx := strings.Index(rest, "${")
		if idx == -1 {
			return nil
		}
		after := rest[idx+2:]
		closeIdx := strings.Index(after, "}")
		expr := unwrapTemplateEncoding(after[:closeIdx])

		switch {
		case expr == "parameters":
		case strings.HasPrefix(expr, "secrets.") && strings.TrimSpace(strings.TrimPrefix(expr, "secrets.")) != "":
		default:
			root := strings.Split(expr, ".")[0]
			if _, ok := declaredParams[root]; !ok {
				return ValidationError{
					Path:    path,
					Message: fmt.Sprintf("contains unknown placeholder %q", "${"+expr+"}"),
				}
			}
		}

		rest = after[closeIdx+1:]
	}
}

func unwrapTemplateEncoding(expr string) string {
	for _, encoding := range []string{"json", "query", "path"} {
		if strings.HasPrefix(expr, encoding+":") {
			return strings.TrimPrefix(expr, encoding+":")
		}
	}
	return expr
}

func DeclaredToolParameters(parameters json.RawMessage) (map[string]struct{}, error) {
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

func ValidateToolParameterSchema(parameters json.RawMessage) error {
	var schema jsonschema.Schema
	if err := json.Unmarshal(parameters, &schema); err != nil {
		return fmt.Errorf("must be a valid JSON Schema: %w", err)
	}
	if _, err := schema.Resolve(nil); err != nil {
		return fmt.Errorf("must resolve as a valid JSON Schema: %w", err)
	}
	return nil
}
