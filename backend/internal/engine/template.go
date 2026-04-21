package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/templateutil"
)

type templateResolutionOptions struct {
	parameters           map[string]any
	declaredParams       map[string]struct{}
	secrets              map[string]string
	errorOnMissingParams bool
	errorOnMissingSecret bool
}

func substituteTemplate(template map[string]any, args map[string]any) map[string]any {
	resolved, err := resolveTemplateMap(template, templateResolutionOptions{
		parameters:     cloneMapAny(args),
		declaredParams: declaredTemplateParams(args),
	})
	if err != nil {
		return cloneMapAny(template)
	}
	return resolved
}

func resolveTemplateMap(template map[string]any, opts templateResolutionOptions) (map[string]any, error) {
	result := make(map[string]any, len(template))
	for key, value := range template {
		resolved, err := resolveTemplateValue(value, opts)
		if err != nil {
			return nil, err
		}
		result[key] = resolved
	}
	return result, nil
}

func resolveTemplateValue(value any, opts templateResolutionOptions) (any, error) {
	switch v := value.(type) {
	case string:
		return resolveTemplateString(v, opts)
	case map[string]any:
		return resolveTemplateMap(v, opts)
	case []any:
		out := make([]any, len(v))
		for i, elem := range v {
			resolved, err := resolveTemplateValue(elem, opts)
			if err != nil {
				return nil, err
			}
			out[i] = resolved
		}
		return out, nil
	default:
		return value, nil
	}
}

func resolveTemplateString(s string, opts templateResolutionOptions) (any, error) {
	if s == "${parameters}" {
		return cloneMapAny(nonNilParameters(opts.parameters)), nil
	}

	var builder strings.Builder
	remaining := s
	for {
		idx := strings.Index(remaining, "${")
		if idx == -1 {
			builder.WriteString(remaining)
			return builder.String(), nil
		}

		builder.WriteString(remaining[:idx])
		after := remaining[idx+2:]
		closeIdx := strings.Index(after, "}")
		if closeIdx == -1 {
			return nil, fmt.Errorf("unclosed placeholder: %q", remaining[idx:])
		}

		expr := after[:closeIdx]
		replacement, resolved, err := resolveTemplatePlaceholder(expr, opts)
		if err != nil {
			return nil, err
		}
		if resolved {
			builder.WriteString(replacement)
		} else {
			builder.WriteString("${")
			builder.WriteString(expr)
			builder.WriteString("}")
		}

		remaining = after[closeIdx+1:]
	}
}

func resolveTemplatePlaceholder(expr string, opts templateResolutionOptions) (string, bool, error) {
	if expr == "parameters" {
		return encodeTemplateValue(nonNilParameters(opts.parameters)), true, nil
	}

	if strings.HasPrefix(expr, "secrets.") {
		key := strings.TrimPrefix(expr, "secrets.")
		value, ok := opts.secrets[key]
		if ok {
			return value, true, nil
		}
		if opts.errorOnMissingSecret {
			return "", false, fmt.Errorf("cannot resolve secret %q", key)
		}
		return "", false, nil
	}

	_, resolvedValue, ok, err := resolveParameterReference(expr, opts)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	return encodeTemplateValue(resolvedValue), true, nil
}

func resolveParameterReference(expr string, opts templateResolutionOptions) (string, any, bool, error) {
	segments := strings.Split(expr, ".")
	if len(segments) == 0 {
		return "", nil, false, nil
	}
	root := segments[0]
	if _, declared := opts.declaredParams[root]; !declared {
		return "", nil, false, nil
	}

	current, ok := opts.parameters[root]
	if !ok {
		if opts.errorOnMissingParams {
			return root, nil, false, fmt.Errorf("cannot resolve path %q: key %q not found", expr, root)
		}
		return root, nil, false, nil
	}

	for _, segment := range segments[1:] {
		object, isObject := current.(map[string]any)
		if !isObject {
			if opts.errorOnMissingParams {
				return root, nil, false, fmt.Errorf("cannot resolve path %q: key %q not found", expr, segment)
			}
			return root, nil, false, nil
		}
		next, exists := object[segment]
		if !exists {
			if opts.errorOnMissingParams {
				return root, nil, false, fmt.Errorf("cannot resolve path %q: key %q not found", expr, segment)
			}
			return root, nil, false, nil
		}
		current = next
	}

	return root, cloneTemplateValue(current), true, nil
}

func validateTemplatePlaceholders(value any, path string) error {
	return templateutil.ValidateTemplatePlaceholders(value, path)
}

func validateTemplateReferences(value any, path string, declaredParams map[string]struct{}) error {
	return templateutil.ValidateTemplateReferences(value, path, declaredParams)
}

func declaredTemplateParams(args map[string]any) map[string]struct{} {
	if len(args) == 0 {
		return nil
	}
	declared := make(map[string]struct{}, len(args))
	for key := range args {
		declared[key] = struct{}{}
	}
	return declared
}

func encodeTemplateValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(encoded)
	}
}

func cloneMapAny(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	cloned := make(map[string]any, len(value))
	for key, item := range value {
		cloned[key] = cloneTemplateValue(item)
	}
	return cloned
}

func cloneTemplateValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMapAny(v)
	case []any:
		cloned := make([]any, len(v))
		for i, item := range v {
			cloned[i] = cloneTemplateValue(item)
		}
		return cloned
	default:
		return v
	}
}

func nonNilParameters(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}
