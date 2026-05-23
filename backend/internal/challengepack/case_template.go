package challengepack

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var caseTemplatePattern = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_.]*)\s*\}\}`)

// CaseTemplateContext holds per-case values available to {{placeholder}} rendering.
type CaseTemplateContext map[string]any

// BuildCaseTemplateContext merges case payload and inputs. Input keys override payload keys.
func BuildCaseTemplateContext(payload map[string]any, inputs []CaseInput) CaseTemplateContext {
	ctx := CaseTemplateContext{}
	for key, value := range payload {
		ctx[key] = cloneCaseTemplateValue(value)
	}
	for _, input := range inputs {
		key := strings.TrimSpace(input.Key)
		if key == "" {
			continue
		}
		if value, ok := caseInputTemplateValue(input); ok {
			ctx[key] = value
		}
	}
	return ctx
}

// BuildCaseTemplateContextFromPayload decodes a stored case payload blob then merges inputs.
func BuildCaseTemplateContextFromPayload(payload json.RawMessage, inputs []CaseInput) (CaseTemplateContext, error) {
	decoded, err := decodeCaseTemplatePayload(payload)
	if err != nil {
		return nil, err
	}
	return BuildCaseTemplateContext(decoded, inputs), nil
}

func decodeCaseTemplatePayload(payload json.RawMessage) (map[string]any, error) {
	if len(payload) == 0 {
		return map[string]any{}, nil
	}

	var stored StoredCaseDocument
	if err := json.Unmarshal(payload, &stored); err == nil && (stored.SchemaVersion > 0 || len(stored.Inputs) > 0 || len(stored.Expectations) > 0) {
		return cloneObject(stored.Payload), nil
	}

	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		return nil, fmt.Errorf("decode case payload: %w", err)
	}
	return object, nil
}

func caseInputTemplateValue(input CaseInput) (any, bool) {
	if input.Value == nil {
		return nil, false
	}
	return cloneCaseTemplateValue(input.Value), true
}

// ExtractCaseTemplatePlaceholders returns unique placeholder paths in template order of first appearance.
func ExtractCaseTemplatePlaceholders(template string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, match := range caseTemplatePattern.FindAllStringSubmatch(template, -1) {
		if len(match) != 2 {
			continue
		}
		key := strings.TrimSpace(match[1])
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

// RenderCaseTemplate replaces {{placeholders}} using ctx. Unresolved placeholders return an error.
func RenderCaseTemplate(template string, ctx CaseTemplateContext) (string, error) {
	var firstErr error
	rendered := caseTemplatePattern.ReplaceAllStringFunc(template, func(match string) string {
		groups := caseTemplatePattern.FindStringSubmatch(match)
		if len(groups) != 2 {
			return match
		}
		value, ok, err := resolveCaseTemplatePath(groups[1], ctx)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return match
		}
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("unresolved placeholder {{%s}}", strings.TrimSpace(groups[1]))
			}
			return match
		}
		return formatCaseTemplateValue(value)
	})
	if firstErr != nil {
		return rendered, firstErr
	}
	return rendered, nil
}

// FindUnresolvedCaseTemplateLiterals returns {{...}} tokens still present after lenient rendering.
func FindUnresolvedCaseTemplateLiterals(rendered string) []string {
	return caseTemplatePattern.FindAllString(rendered, -1)
}

// RenderCaseTemplateLenient replaces known placeholders and leaves unknown {{tokens}} literal.
func RenderCaseTemplateLenient(template string, ctx CaseTemplateContext) string {
	return caseTemplatePattern.ReplaceAllStringFunc(template, func(match string) string {
		groups := caseTemplatePattern.FindStringSubmatch(match)
		if len(groups) != 2 {
			return match
		}
		value, ok, err := resolveCaseTemplatePath(groups[1], ctx)
		if err != nil || !ok {
			return match
		}
		return formatCaseTemplateValue(value)
	})
}

// ValidateCaseTemplate ensures every placeholder in template resolves in ctx.
func ValidateCaseTemplate(template string, ctx CaseTemplateContext, fieldPath string) error {
	for _, placeholder := range ExtractCaseTemplatePlaceholders(template) {
		if _, ok, err := resolveCaseTemplatePath(placeholder, ctx); err != nil {
			return ValidationError{Field: fieldPath, Message: err.Error()}
		} else if !ok {
			return ValidationError{
				Field:   fieldPath,
				Message: fmt.Sprintf("unresolved placeholder {{%s}} for case template context", placeholder),
			}
		}
	}
	return nil
}

func resolveCaseTemplatePath(path string, ctx CaseTemplateContext) (any, bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, false, fmt.Errorf("empty placeholder path")
	}
	segments := strings.Split(path, ".")
	current := any(ctx)
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return nil, false, fmt.Errorf("invalid placeholder path %q", path)
		}
		object, ok := current.(map[string]any)
		if !ok {
			if typed, isCaseCtx := current.(CaseTemplateContext); isCaseCtx {
				object = map[string]any(typed)
			} else {
				return nil, false, nil
			}
		}
		next, exists := object[segment]
		if !exists {
			return nil, false, nil
		}
		current = next
	}
	return current, true, nil
}

func formatCaseTemplateValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", typed)
	case int32:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", typed), "0"), ".")
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(encoded)
	}
}

func cloneCaseTemplateValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneObject(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = cloneCaseTemplateValue(item)
		}
		return cloned
	default:
		return typed
	}
}
