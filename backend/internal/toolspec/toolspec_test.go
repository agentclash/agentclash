package toolspec

import (
	"encoding/json"
	"testing"
)

func TestPrimitives_NonEmpty_WellFormed(t *testing.T) {
	prims := Primitives()
	if len(prims) == 0 {
		t.Fatal("expected a non-empty primitive catalog")
	}
	seen := map[string]struct{}{}
	for _, p := range prims {
		if p.Name == "" {
			t.Errorf("primitive with empty name: %+v", p)
		}
		if _, dup := seen[p.Name]; dup {
			t.Errorf("duplicate primitive name %q", p.Name)
		}
		seen[p.Name] = struct{}{}
		if p.Kind == "" {
			t.Errorf("primitive %q has empty kind", p.Name)
		}
		var schema map[string]any
		if err := json.Unmarshal(p.Parameters, &schema); err != nil {
			t.Errorf("primitive %q has invalid parameters JSON: %v", p.Name, err)
		}
	}
	if _, ok := PrimitiveByName(PrimitiveHTTPRequest); !ok {
		t.Errorf("expected http_request in catalog")
	}
	if _, ok := PrimitiveByName("does_not_exist"); ok {
		t.Errorf("did not expect unknown primitive to resolve")
	}
	if submit, _ := PrimitiveByName(PrimitiveSubmit); submit.Delegatable {
		t.Errorf("submit should not be delegatable")
	}
}

func mustErrField(t *testing.T, errs ValidationErrors, field string) {
	t.Helper()
	for _, e := range errs {
		if e.Field == field {
			return
		}
	}
	t.Fatalf("expected a validation error on field %q; got %v", field, errs)
}

func TestValidateDefinition_Primitive_Delegate_OK(t *testing.T) {
	def := json.RawMessage(`{
		"tool_type":"primitive",
		"parameters":{"type":"object","properties":{"order_id":{"type":"string"}},"required":["order_id"]},
		"implementation":{"mode":"delegate","primitive":"http_request","args":{"method":"GET","url":"https://api/orders/${order_id}","headers":{"Authorization":"Bearer ${secrets.API_KEY}"}}}
	}`)
	if errs := ValidateDefinition(ToolTypePrimitive, def, ValidateOptions{}); len(errs) != 0 {
		t.Fatalf("expected no errors; got %v", errs)
	}
}

func TestValidateDefinition_Primitive_UnknownPrimitive(t *testing.T) {
	def := json.RawMessage(`{"tool_type":"primitive","implementation":{"mode":"delegate","primitive":"teleport","args":{}}}`)
	errs := ValidateDefinition(ToolTypePrimitive, def, ValidateOptions{})
	mustErrField(t, errs, "definition.implementation.primitive")
}

func TestValidateDefinition_Primitive_BadPlaceholder(t *testing.T) {
	def := json.RawMessage(`{
		"tool_type":"primitive",
		"parameters":{"type":"object","properties":{"order_id":{"type":"string"}}},
		"implementation":{"mode":"delegate","primitive":"http_request","args":{"url":"https://api/${missing}"}}
	}`)
	errs := ValidateDefinition(ToolTypePrimitive, def, ValidateOptions{})
	mustErrField(t, errs, "definition.implementation.args")
}

func TestValidateDefinition_Primitive_SecretsOnlyHTTP(t *testing.T) {
	def := json.RawMessage(`{
		"tool_type":"primitive",
		"parameters":{"type":"object","properties":{"p":{"type":"string"}}},
		"implementation":{"mode":"delegate","primitive":"read_file","args":{"path":"${secrets.X}"}}
	}`)
	errs := ValidateDefinition(ToolTypePrimitive, def, ValidateOptions{})
	mustErrField(t, errs, "definition.implementation.args")
}

func TestValidateDefinition_Primitive_Mock_OK(t *testing.T) {
	def := json.RawMessage(`{"tool_type":"primitive","implementation":{"mode":"mock","mock":{"strategy":"static","response":{"ok":true}}}}`)
	if errs := ValidateDefinition(ToolTypePrimitive, def, ValidateOptions{}); len(errs) != 0 {
		t.Fatalf("expected no errors; got %v", errs)
	}
}

func TestValidateDefinition_Primitive_Mock_BadStrategy(t *testing.T) {
	def := json.RawMessage(`{"tool_type":"primitive","implementation":{"mode":"mock","mock":{"strategy":"telepathy"}}}`)
	errs := ValidateDefinition(ToolTypePrimitive, def, ValidateOptions{})
	mustErrField(t, errs, "definition.implementation.mock.strategy")
}

func TestValidateDefinition_Composed_OK(t *testing.T) {
	def := json.RawMessage(`{
		"tool_type":"composed",
		"parameters":{"type":"object","properties":{"order_id":{"type":"string"},"amount":{"type":"number"}}},
		"steps":[
			{"id":"s1","ref":{"type":"primitive","name":"http_request"},"inputs":{"method":"GET","url":"https://api/orders/${params.order_id}"}},
			{"id":"s2","ref":{"type":"tool","name":"check_policy"},"inputs":{"total":"${params.amount}","order":"${steps.s1.body}"}}
		]
	}`)
	opts := ValidateOptions{KnownToolSlugs: map[string]struct{}{"check_policy": {}}, SelfSlug: "refund_flow"}
	if errs := ValidateDefinition(ToolTypeComposed, def, opts); len(errs) != 0 {
		t.Fatalf("expected no errors; got %v", errs)
	}
}

func TestValidateDefinition_Composed_EmptySteps(t *testing.T) {
	def := json.RawMessage(`{"tool_type":"composed","steps":[]}`)
	errs := ValidateDefinition(ToolTypeComposed, def, ValidateOptions{})
	mustErrField(t, errs, "definition.steps")
}

func TestValidateDefinition_Composed_ForwardStepRef(t *testing.T) {
	def := json.RawMessage(`{
		"tool_type":"composed",
		"parameters":{"type":"object","properties":{}},
		"steps":[
			{"id":"s1","ref":{"type":"primitive","name":"http_request"},"inputs":{"url":"${steps.s2.x}"}},
			{"id":"s2","ref":{"type":"primitive","name":"http_request"},"inputs":{"url":"https://x"}}
		]
	}`)
	errs := ValidateDefinition(ToolTypeComposed, def, ValidateOptions{})
	mustErrField(t, errs, "definition.steps[0].inputs")
}

func TestValidateDefinition_Composed_SelfReference(t *testing.T) {
	def := json.RawMessage(`{
		"tool_type":"composed",
		"steps":[{"id":"s1","ref":{"type":"tool","name":"refund_flow"},"inputs":{}}]
	}`)
	opts := ValidateOptions{KnownToolSlugs: map[string]struct{}{"refund_flow": {}}, SelfSlug: "refund_flow"}
	errs := ValidateDefinition(ToolTypeComposed, def, opts)
	mustErrField(t, errs, "definition.steps[0].ref.name")
}

func TestValidateDefinition_Composed_UnknownToolRef(t *testing.T) {
	def := json.RawMessage(`{
		"tool_type":"composed",
		"steps":[{"id":"s1","ref":{"type":"tool","name":"ghost"},"inputs":{}}]
	}`)
	opts := ValidateOptions{KnownToolSlugs: map[string]struct{}{"check_policy": {}}}
	errs := ValidateDefinition(ToolTypeComposed, def, opts)
	mustErrField(t, errs, "definition.steps[0].ref.name")
}

func TestValidateDefinition_ToolTypeMismatch(t *testing.T) {
	def := json.RawMessage(`{"tool_type":"composed","implementation":{"mode":"mock","mock":{"strategy":"static"}}}`)
	errs := ValidateDefinition(ToolTypePrimitive, def, ValidateOptions{})
	mustErrField(t, errs, "definition.tool_type")
}

func TestValidateDefinition_EmptyDefinition(t *testing.T) {
	errs := ValidateDefinition(ToolTypePrimitive, json.RawMessage(``), ValidateOptions{})
	mustErrField(t, errs, "definition")
}

func TestValidateDefinition_BadParametersType(t *testing.T) {
	def := json.RawMessage(`{"tool_type":"primitive","parameters":{"type":"array"},"implementation":{"mode":"mock","mock":{"strategy":"static"}}}`)
	errs := ValidateDefinition(ToolTypePrimitive, def, ValidateOptions{})
	mustErrField(t, errs, "definition.parameters.type")
}
