package challengepack

import (
	"strings"
	"testing"
)

func TestParseYAML_AllowsValidComposedToolDefinition(t *testing.T) {
	_, err := ParseYAML([]byte(`
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  evaluation_spec:
    name: support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: medium
input_sets:
  - key: default
    name: Default Inputs
    cases:
      - challenge_key: ticket-1
        case_key: sample-1
        inputs:
          - key: prompt
            kind: text
            value: hello
        expectations:
          - key: answer
            kind: text
            source: input:prompt
tools:
  custom:
    - name: check_inventory
      description: Check inventory
      parameters:
        type: object
        properties:
          sku:
            type: string
      implementation:
        primitive: http_request
        args:
          method: GET
          url: https://api.example.com/inventory/${sku}
          headers:
            Authorization: Bearer ${secrets.INVENTORY_API_KEY}
`))
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
}

func TestParseYAML_RejectsUnknownComposedToolPlaceholder(t *testing.T) {
	_, err := ParseYAML([]byte(`
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  evaluation_spec:
    name: support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: medium
input_sets:
  - key: default
    name: Default Inputs
    cases:
      - challenge_key: ticket-1
        case_key: sample-1
        inputs:
          - key: prompt
            kind: text
            value: hello
        expectations:
          - key: answer
            kind: text
            source: input:prompt
tools:
  custom:
    - name: check_inventory
      description: Check inventory
      parameters:
        type: object
        properties:
          sku:
            type: string
      implementation:
        primitive: http_request
        args:
          url: https://api.example.com/inventory/${missing}
`))
	if err == nil {
		t.Fatal("expected ParseYAML to fail")
	}
	if !strings.Contains(err.Error(), `tools.custom[0].implementation.args.url contains unknown placeholder "${missing}"`) {
		t.Fatalf("error = %v, want unknown placeholder validation", err)
	}
}

func TestParseYAML_RejectsSelfReferencingComposedToolPrimitive(t *testing.T) {
	_, err := ParseYAML([]byte(`
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  evaluation_spec:
    name: support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: medium
input_sets:
  - key: default
    name: Default Inputs
    cases:
      - challenge_key: ticket-1
        case_key: sample-1
        inputs:
          - key: prompt
            kind: text
            value: hello
        expectations:
          - key: answer
            kind: text
            source: input:prompt
tools:
  custom:
    - name: check_inventory
      description: Check inventory
      parameters:
        type: object
        properties:
          sku:
            type: string
      implementation:
        primitive: check_inventory
        args:
          url: https://api.example.com/inventory/${sku}
`))
	if err == nil {
		t.Fatal("expected ParseYAML to fail")
	}
	if !strings.Contains(err.Error(), "tools.custom[0].implementation.primitive cannot reference the tool's own name") {
		t.Fatalf("error = %v, want self-reference validation", err)
	}
}

func TestParseYAML_RejectsInvalidComposedToolParameterSchema(t *testing.T) {
	_, err := ParseYAML([]byte(`
pack:
  slug: support-eval
  name: Support Eval
  family: support
version:
  number: 1
  evaluation_spec:
    name: support-v1
    version_number: 1
    judge_mode: deterministic
    validators:
      - key: exact
        type: exact_match
        target: final_output
        expected_from: challenge_input
    scorecard:
      dimensions: [correctness]
challenges:
  - key: ticket-1
    title: Ticket One
    category: support
    difficulty: medium
input_sets:
  - key: default
    name: Default Inputs
    cases:
      - challenge_key: ticket-1
        case_key: sample-1
        inputs:
          - key: prompt
            kind: text
            value: hello
        expectations:
          - key: answer
            kind: text
            source: input:prompt
tools:
  custom:
    - name: check_inventory
      description: Check inventory
      parameters:
        type: object
        properties: invalid
      implementation:
        primitive: http_request
        args:
          url: https://api.example.com/inventory/${sku}
`))
	if err == nil {
		t.Fatal("expected ParseYAML to fail")
	}
	if !strings.Contains(err.Error(), "tools.custom[0].parameters") {
		t.Fatalf("error = %v, want parameter schema validation", err)
	}
}
