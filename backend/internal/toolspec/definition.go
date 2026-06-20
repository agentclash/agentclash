package toolspec

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Tool type discriminators. These must match the `tool_kind` column on the
// tools table and the `tool_type` field inside the definition.
const (
	ToolTypePrimitive = "primitive"
	ToolTypeComposed  = "composed"
)

// Primitive implementation modes.
const (
	ModeDelegate = "delegate"
	ModeMock     = "mock"
)

var mockStrategies = map[string]struct{}{"static": {}, "lookup": {}, "echo": {}}

// ValidationError is a single field-scoped problem with a tool definition.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string { return e.Field + ": " + e.Message }

// ValidationErrors is a collection of validation problems.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	parts := make([]string, len(e))
	for i, ve := range e {
		parts[i] = ve.Error()
	}
	return strings.Join(parts, "; ")
}

// ValidateOptions carries context the validator needs beyond the definition itself.
type ValidateOptions struct {
	// KnownToolSlugs is the set of saved tool slugs in the same workspace, used to
	// validate composed step refs of type "tool". When nil, tool-ref existence is
	// not checked (structural validation only).
	KnownToolSlugs map[string]struct{}
	// SelfSlug is the slug of the tool being saved, used to reject self-reference.
	SelfSlug string
}

var placeholderRe = regexp.MustCompile(`\$\{([^}]*)\}`)

// ValidateDefinition validates a tool definition against the canonical schema.
// toolKind is the value stored in tools.tool_kind ("primitive" or "composed").
func ValidateDefinition(toolKind string, raw json.RawMessage, opts ValidateOptions) ValidationErrors {
	var errs ValidationErrors

	if len(strings.TrimSpace(string(raw))) == 0 {
		return ValidationErrors{{Field: "definition", Message: "is required"}}
	}

	var top struct {
		ToolType   string          `json:"tool_type"`
		Parameters json.RawMessage `json:"parameters"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return ValidationErrors{{Field: "definition", Message: fmt.Sprintf("must be valid JSON: %v", err)}}
	}

	if top.ToolType == "" {
		errs = append(errs, ValidationError{Field: "definition.tool_type", Message: "is required"})
	} else if top.ToolType != toolKind {
		errs = append(errs, ValidationError{
			Field:   "definition.tool_type",
			Message: fmt.Sprintf("must match tool_kind %q, got %q", toolKind, top.ToolType),
		})
	}

	declared, paramErrs := declaredParameters(top.Parameters)
	errs = append(errs, paramErrs...)

	switch toolKind {
	case ToolTypePrimitive:
		errs = append(errs, validatePrimitive(raw, declared)...)
	case ToolTypeComposed:
		errs = append(errs, validateComposed(raw, declared, opts)...)
	default:
		errs = append(errs, ValidationError{
			Field:   "tool_kind",
			Message: fmt.Sprintf("must be %q or %q", ToolTypePrimitive, ToolTypeComposed),
		})
	}

	return errs
}

// declaredParameters returns the set of declared parameter names from a JSON
// Schema object and any structural errors.
func declaredParameters(raw json.RawMessage) (map[string]struct{}, ValidationErrors) {
	declared := map[string]struct{}{}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return declared, nil
	}
	var schema struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return declared, ValidationErrors{{Field: "definition.parameters", Message: "must be a valid JSON Schema object"}}
	}
	if schema.Type != "" && schema.Type != "object" {
		return declared, ValidationErrors{{Field: "definition.parameters.type", Message: `must be "object"`}}
	}
	for name := range schema.Properties {
		declared[name] = struct{}{}
	}
	return declared, nil
}

func validatePrimitive(raw json.RawMessage, declared map[string]struct{}) ValidationErrors {
	var errs ValidationErrors
	var def struct {
		Implementation *struct {
			Mode      string          `json:"mode"`
			Primitive string          `json:"primitive"`
			Args      json.RawMessage `json:"args"`
			Mock      json.RawMessage `json:"mock"`
		} `json:"implementation"`
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		return ValidationErrors{{Field: "definition.implementation", Message: "must be valid JSON"}}
	}
	if def.Implementation == nil {
		return ValidationErrors{{Field: "definition.implementation", Message: "is required"}}
	}
	impl := def.Implementation

	mode := strings.TrimSpace(impl.Mode)
	if mode == "" {
		if len(impl.Mock) > 0 {
			mode = ModeMock
		} else {
			mode = ModeDelegate
		}
	}

	switch mode {
	case ModeDelegate:
		primitive := strings.TrimSpace(impl.Primitive)
		if primitive == "" {
			errs = append(errs, ValidationError{Field: "definition.implementation.primitive", Message: "is required for delegate mode"})
			break
		}
		spec, ok := PrimitiveByName(primitive)
		if !ok {
			errs = append(errs, ValidationError{Field: "definition.implementation.primitive", Message: fmt.Sprintf("unknown primitive %q", primitive)})
			break
		}
		if !spec.Delegatable {
			errs = append(errs, ValidationError{Field: "definition.implementation.primitive", Message: fmt.Sprintf("primitive %q cannot be used as a building block", primitive)})
		}
		errs = append(errs, validateArgsPlaceholders("definition.implementation.args", impl.Args, declared, primitive == PrimitiveHTTPRequest)...)
	case ModeMock:
		errs = append(errs, validateMock("definition.implementation.mock", impl.Mock)...)
	default:
		errs = append(errs, ValidationError{Field: "definition.implementation.mode", Message: fmt.Sprintf("must be %q or %q", ModeDelegate, ModeMock)})
	}

	return errs
}

func validateMock(field string, raw json.RawMessage) ValidationErrors {
	if len(raw) == 0 {
		return ValidationErrors{{Field: field, Message: "is required for mock mode"}}
	}
	var mock struct {
		Strategy string `json:"strategy"`
	}
	if err := json.Unmarshal(raw, &mock); err != nil {
		return ValidationErrors{{Field: field, Message: "must be valid JSON"}}
	}
	if _, ok := mockStrategies[strings.TrimSpace(mock.Strategy)]; !ok {
		return ValidationErrors{{Field: field + ".strategy", Message: `must be one of "static", "lookup", "echo"`}}
	}
	return nil
}

func validateComposed(raw json.RawMessage, declared map[string]struct{}, opts ValidateOptions) ValidationErrors {
	var errs ValidationErrors
	var def struct {
		Steps []struct {
			ID  string `json:"id"`
			Ref struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"ref"`
			Inputs json.RawMessage `json:"inputs"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		return ValidationErrors{{Field: "definition.steps", Message: "must be valid JSON"}}
	}
	if len(def.Steps) == 0 {
		return ValidationErrors{{Field: "definition.steps", Message: "must contain at least one step"}}
	}

	seenStepIDs := map[string]struct{}{}
	for i, step := range def.Steps {
		field := fmt.Sprintf("definition.steps[%d]", i)
		id := strings.TrimSpace(step.ID)
		if id == "" {
			errs = append(errs, ValidationError{Field: field + ".id", Message: "is required"})
		} else if _, dup := seenStepIDs[id]; dup {
			errs = append(errs, ValidationError{Field: field + ".id", Message: fmt.Sprintf("duplicate step id %q", id)})
		}

		refType := strings.TrimSpace(step.Ref.Type)
		refName := strings.TrimSpace(step.Ref.Name)
		isHTTP := false
		switch refType {
		case "primitive":
			spec, ok := PrimitiveByName(refName)
			if !ok {
				errs = append(errs, ValidationError{Field: field + ".ref.name", Message: fmt.Sprintf("unknown primitive %q", refName)})
			} else if !spec.Delegatable {
				errs = append(errs, ValidationError{Field: field + ".ref.name", Message: fmt.Sprintf("primitive %q cannot be used as a step", refName)})
			}
			isHTTP = refName == PrimitiveHTTPRequest
		case "tool":
			if refName == "" {
				errs = append(errs, ValidationError{Field: field + ".ref.name", Message: "is required"})
			}
			if opts.SelfSlug != "" && refName == opts.SelfSlug {
				errs = append(errs, ValidationError{Field: field + ".ref.name", Message: "a tool cannot reference itself"})
			}
			if opts.KnownToolSlugs != nil && refName != "" {
				if _, ok := opts.KnownToolSlugs[refName]; !ok {
					errs = append(errs, ValidationError{Field: field + ".ref.name", Message: fmt.Sprintf("references unknown tool %q in this workspace", refName)})
				}
			}
		default:
			errs = append(errs, ValidationError{Field: field + ".ref.type", Message: `must be "primitive" or "tool"`})
		}

		errs = append(errs, validateComposedInputs(field+".inputs", step.Inputs, declared, seenStepIDs, isHTTP)...)

		if id != "" {
			seenStepIDs[id] = struct{}{}
		}
	}

	return errs
}

// validateComposedInputs checks ${params.X} and ${steps.SID.*} placeholders. Only
// earlier step ids (already in priorStepIDs) may be referenced — forward refs fail.
func validateComposedInputs(field string, inputs json.RawMessage, declared, priorStepIDs map[string]struct{}, allowSecrets bool) ValidationErrors {
	var errs ValidationErrors
	for _, ph := range placeholdersIn(inputs) {
		expr := strings.TrimSpace(ph)
		switch {
		case expr == "":
			errs = append(errs, ValidationError{Field: field, Message: "empty placeholder ${}"})
		case strings.HasPrefix(expr, "params."):
			name := strings.TrimPrefix(expr, "params.")
			if _, ok := declared[name]; !ok {
				errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("references undeclared parameter %q", name)})
			}
		case strings.HasPrefix(expr, "steps."):
			rest := strings.TrimPrefix(expr, "steps.")
			sid := rest
			if dot := strings.IndexByte(rest, '.'); dot >= 0 {
				sid = rest[:dot]
			}
			if _, ok := priorStepIDs[sid]; !ok {
				errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("references step %q which is not an earlier step", sid)})
			}
		case strings.HasPrefix(expr, "secrets."):
			if !allowSecrets {
				errs = append(errs, ValidationError{Field: field, Message: "secrets are only allowed in http_request steps"})
			}
		default:
			errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("unsupported placeholder ${%s}; use ${params.X}, ${steps.ID.field} or ${secrets.NAME}", expr)})
		}
	}
	return errs
}

// validateArgsPlaceholders checks ${param}, ${parameters} and ${secrets.X}
// placeholders for primitive delegate args (engine-compatible syntax).
func validateArgsPlaceholders(field string, args json.RawMessage, declared map[string]struct{}, allowSecrets bool) ValidationErrors {
	var errs ValidationErrors
	for _, ph := range placeholdersIn(args) {
		expr := strings.TrimSpace(ph)
		switch {
		case expr == "":
			errs = append(errs, ValidationError{Field: field, Message: "empty placeholder ${}"})
		case expr == "parameters":
			// whole-parameters object reference — always allowed
		case strings.HasPrefix(expr, "secrets."):
			if !allowSecrets {
				errs = append(errs, ValidationError{Field: field, Message: "secrets are only allowed for http_request"})
			}
		default:
			if _, ok := declared[expr]; !ok {
				errs = append(errs, ValidationError{Field: field, Message: fmt.Sprintf("references undeclared parameter %q", expr)})
			}
		}
	}
	return errs
}

// placeholdersIn walks an arbitrary JSON value and returns every ${...} token
// found inside any string value.
func placeholdersIn(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	var out []string
	var walk func(any)
	walk = func(node any) {
		switch t := node.(type) {
		case string:
			for _, m := range placeholderRe.FindAllStringSubmatch(t, -1) {
				out = append(out, m[1])
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(v)
	return out
}
