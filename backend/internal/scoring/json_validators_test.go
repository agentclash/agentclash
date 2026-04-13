package scoring

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseJSONValue(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
		check     func(t *testing.T, value any)
	}{
		{
			name: "valid object",
			input: `{"key":"value"}`,
			check: func(t *testing.T, value any) {
				m, ok := value.(map[string]any)
				if !ok {
					t.Fatalf("expected map, got %T", value)
				}
				if m["key"] != "value" {
					t.Fatalf("expected key=value, got %v", m["key"])
				}
			},
		},
		{
			name: "number returns json.Number",
			input: `42`,
			check: func(t *testing.T, value any) {
				n, ok := value.(json.Number)
				if !ok {
					t.Fatalf("expected json.Number, got %T", value)
				}
				if n.String() != "42" {
					t.Fatalf("expected 42, got %s", n)
				}
			},
		},
		{
			name: "valid string",
			input: `"hello"`,
			check: func(t *testing.T, value any) {
				s, ok := value.(string)
				if !ok || s != "hello" {
					t.Fatalf("expected hello, got %v", value)
				}
			},
		},
		{
			name: "null",
			input: `null`,
			check: func(t *testing.T, value any) {
				if value != nil {
					t.Fatalf("expected nil, got %v", value)
				}
			},
		},
		{
			name: "valid array",
			input: `[1,2,3]`,
			check: func(t *testing.T, value any) {
				arr, ok := value.([]any)
				if !ok || len(arr) != 3 {
					t.Fatalf("expected array of 3, got %v", value)
				}
			},
		},
		{
			name: "boolean true",
			input: `true`,
			check: func(t *testing.T, value any) {
				b, ok := value.(bool)
				if !ok || !b {
					t.Fatalf("expected true, got %v", value)
				}
			},
		},
		{
			name: "unicode string",
			input: `"\u00e9\u4e16\u754c"`,
			check: func(t *testing.T, value any) {
				s, ok := value.(string)
				if !ok || s != "\u00e9\u4e16\u754c" {
					t.Fatalf("expected unicode string, got %v", value)
				}
			},
		},
		{
			name:    "empty string",
			input:   ``,
			wantErr: true,
		},
		{
			name:    "malformed JSON",
			input:   `{broken`,
			wantErr: true,
		},
		{
			name:      "multiple values",
			input:     `1 2`,
			wantErr:   true,
			errSubstr: "multiple JSON values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := parseJSONValue(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, value)
			}
		})
	}
}

func TestParseJSONSchema(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantDraft string
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "empty draft defaults to 2020-12",
			input:     `{"type":"object"}`,
			wantDraft: jsonSchemaDraft202012,
		},
		{
			name:      "explicit 2020-12",
			input:     `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`,
			wantDraft: jsonSchemaDraft202012,
		},
		{
			name:      "draft-07 http",
			input:     `{"$schema":"http://json-schema.org/draft-07/schema#","type":"object"}`,
			wantDraft: jsonSchemaDraft07,
		},
		{
			name:      "draft-07 https",
			input:     `{"$schema":"https://json-schema.org/draft-07/schema#","type":"object"}`,
			wantDraft: jsonSchemaDraft07HTTPS,
		},
		{
			name:      "unsupported draft",
			input:     `{"$schema":"http://example.com/unsupported","type":"object"}`,
			wantErr:   true,
			errSubstr: "unsupported JSON schema draft",
		},
		{
			name:    "malformed JSON",
			input:   `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, draft, err := parseJSONSchema(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if schema == nil {
				t.Fatalf("expected non-nil schema")
			}
			if draft != tt.wantDraft {
				t.Fatalf("draft = %q, want %q", draft, tt.wantDraft)
			}
		})
	}
}

func TestParseJSONPathExpectation(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantPath       string
		wantComparator string
		wantErr        bool
		errSubstr      string
	}{
		{
			name:           "dollar-prefix path infers exists",
			input:          `$.foo.bar`,
			wantPath:       "$.foo.bar",
			wantComparator: jsonPathComparatorExists,
		},
		{
			name:           "root dollar only",
			input:          `$`,
			wantPath:       "$",
			wantComparator: jsonPathComparatorExists,
		},
		{
			name:           "object with path and value defaults to equals",
			input:          `{"path":"$.x","value":42}`,
			wantPath:       "$.x",
			wantComparator: jsonPathComparatorEquals,
		},
		{
			name:           "object with path only defaults to exists",
			input:          `{"path":"$.x"}`,
			wantPath:       "$.x",
			wantComparator: jsonPathComparatorExists,
		},
		{
			name:           "explicit contains comparator",
			input:          `{"path":"$.name","comparator":"contains","value":"abc"}`,
			wantPath:       "$.name",
			wantComparator: jsonPathComparatorContains,
		},
		{
			name:           "greater_than comparator",
			input:          `{"path":"$.score","comparator":"greater_than","value":10}`,
			wantPath:       "$.score",
			wantComparator: jsonPathComparatorGreater,
		},
		{
			name:           "less_than comparator",
			input:          `{"path":"$.score","comparator":"less_than","value":100}`,
			wantPath:       "$.score",
			wantComparator: jsonPathComparatorLess,
		},
		{
			name:      "unsupported comparator",
			input:     `{"path":"$.x","comparator":"regex"}`,
			wantErr:   true,
			errSubstr: "unsupported comparator",
		},
		{
			name:      "empty input",
			input:     ``,
			wantErr:   true,
			errSubstr: "expectation is empty",
		},
		{
			name:      "whitespace only",
			input:     `   `,
			wantErr:   true,
			errSubstr: "expectation is empty",
		},
		{
			name:      "missing path in object",
			input:     `{"value":1}`,
			wantErr:   true,
			errSubstr: "path is required",
		},
		{
			name:      "multiple JSON values",
			input:     `{"path":"$.x"} {"path":"$.y"}`,
			wantErr:   true,
			errSubstr: "multiple JSON values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := parseJSONPathExpectation(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if exp.Path != tt.wantPath {
				t.Fatalf("path = %q, want %q", exp.Path, tt.wantPath)
			}
			if exp.Comparator != tt.wantComparator {
				t.Fatalf("comparator = %q, want %q", exp.Comparator, tt.wantComparator)
			}
		})
	}
}

func TestExtractJSONPathValue(t *testing.T) {
	mustParse := func(raw string) any {
		v, err := parseJSONValue(raw)
		if err != nil {
			t.Fatalf("failed to parse fixture: %v", err)
		}
		return v
	}

	tests := []struct {
		name      string
		document  any
		path      string
		wantExist bool
		wantErr   bool
		errSubstr string
		check     func(t *testing.T, value any)
	}{
		{
			name:      "root dollar",
			document:  mustParse(`{"a":1}`),
			path:      "$",
			wantExist: true,
			check: func(t *testing.T, value any) {
				m, ok := value.(map[string]any)
				if !ok {
					t.Fatalf("expected map, got %T", value)
				}
				if _, ok := m["a"]; !ok {
					t.Fatalf("expected key 'a'")
				}
			},
		},
		{
			name:      "dot notation",
			document:  mustParse(`{"a":{"b":2}}`),
			path:      "$.a.b",
			wantExist: true,
			check: func(t *testing.T, value any) {
				n, ok := value.(json.Number)
				if !ok || n.String() != "2" {
					t.Fatalf("expected 2, got %v", value)
				}
			},
		},
		{
			name:      "bracket double-quoted property",
			document:  mustParse(`{"a b":1}`),
			path:      `$["a b"]`,
			wantExist: true,
			check: func(t *testing.T, value any) {
				n, ok := value.(json.Number)
				if !ok || n.String() != "1" {
					t.Fatalf("expected 1, got %v", value)
				}
			},
		},
		{
			name:      "bracket double-quoted simple key",
			document:  mustParse(`{"key":"val"}`),
			path:      `$["key"]`,
			wantExist: true,
			check: func(t *testing.T, value any) {
				if value != "val" {
					t.Fatalf("expected val, got %v", value)
				}
			},
		},
		{
			name:      "single-quoted bracket is invalid for strconv.Unquote",
			document:  mustParse(`{"key":1}`),
			path:      `$['key']`,
			wantErr:   true,
			errSubstr: "invalid quoted property",
		},
		{
			name:      "bracket numeric index",
			document:  mustParse(`[10,20,30]`),
			path:      "$[1]",
			wantExist: true,
			check: func(t *testing.T, value any) {
				n, ok := value.(json.Number)
				if !ok || n.String() != "20" {
					t.Fatalf("expected 20, got %v", value)
				}
			},
		},
		{
			name:      "nested dot and bracket",
			document:  mustParse(`{"a":[{"b":3}]}`),
			path:      "$.a[0].b",
			wantExist: true,
			check: func(t *testing.T, value any) {
				n, ok := value.(json.Number)
				if !ok || n.String() != "3" {
					t.Fatalf("expected 3, got %v", value)
				}
			},
		},
		{
			name:      "missing key returns not exists",
			document:  mustParse(`{"a":1}`),
			path:      "$.b",
			wantExist: false,
		},
		{
			name:      "non-object traversal returns not exists",
			document:  mustParse(`{"a":"str"}`),
			path:      "$.a.b",
			wantExist: false,
		},
		{
			name:      "array index out of bounds",
			document:  mustParse(`[1]`),
			path:      "$[5]",
			wantExist: false,
		},
		{
			name:      "negative array index not supported",
			document:  mustParse(`[1,2,3]`),
			path:      "$[-1]",
			wantExist: false,
		},
		{
			name:      "deeply nested 5 levels",
			document:  mustParse(`{"a":{"b":{"c":{"d":{"e":"deep"}}}}}`),
			path:      "$.a.b.c.d.e",
			wantExist: true,
			check: func(t *testing.T, value any) {
				if value != "deep" {
					t.Fatalf("expected deep, got %v", value)
				}
			},
		},
		{
			name:      "empty path",
			document:  mustParse(`{}`),
			path:      "",
			wantErr:   true,
			errSubstr: "path is empty",
		},
		{
			name:      "missing dollar prefix",
			document:  mustParse(`{}`),
			path:      "foo",
			wantErr:   true,
			errSubstr: "must start with '$'",
		},
		{
			name:      "missing property after dot",
			document:  mustParse(`{}`),
			path:      "$.",
			wantErr:   true,
			errSubstr: "missing property name",
		},
		{
			name:      "unterminated bracket",
			document:  mustParse(`{}`),
			path:      "$[",
			wantErr:   true,
			errSubstr: "unterminated bracket",
		},
		{
			name:      "empty bracket",
			document:  mustParse(`{}`),
			path:      "$[]",
			wantErr:   true,
			errSubstr: "empty bracket",
		},
		{
			name:      "unexpected token",
			document:  mustParse(`{}`),
			path:      "$!",
			wantErr:   true,
			errSubstr: "unexpected token",
		},
		{
			name:      "access property on array returns not exists",
			document:  mustParse(`[1,2]`),
			path:      "$.foo",
			wantExist: false,
		},
		{
			name:      "access index on object returns not exists",
			document:  mustParse(`{"a":1}`),
			path:      "$[0]",
			wantExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, exists, err := extractJSONPathValue(tt.document, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if exists != tt.wantExist {
				t.Fatalf("exists = %v, want %v", exists, tt.wantExist)
			}
			if tt.check != nil {
				tt.check(t, value)
			}
		})
	}
}

func TestParseJSONPathBracket(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		start        int
		wantEnd      int
		wantProperty string
		wantIndex    *int
		wantErr      bool
		errSubstr    string
	}{
		{
			name:      "integer index",
			path:      "[0]",
			start:     0,
			wantEnd:   2,
			wantIndex: intPtr(0),
		},
		{
			name:      "single-quoted property is invalid",
			path:      "['key']",
			start:     0,
			wantErr:   true,
			errSubstr: "invalid quoted property",
		},
		{
			name:         "double-quoted property",
			path:         `["key"]`,
			start:        0,
			wantEnd:      6,
			wantProperty: "key",
		},
		{
			name:         "unquoted property fallback",
			path:         "[key]",
			start:        0,
			wantEnd:      4,
			wantProperty: "key",
		},
		{
			name:      "unterminated bracket",
			path:      "[abc",
			start:     0,
			wantErr:   true,
			errSubstr: "unterminated bracket",
		},
		{
			name:      "empty bracket content",
			path:      "[]",
			start:     0,
			wantErr:   true,
			errSubstr: "empty bracket",
		},
		{
			name:      "invalid quoted property",
			path:      "['bad]",
			start:     0,
			wantErr:   true,
			errSubstr: "invalid quoted property",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endIndex, token, err := parseJSONPathBracket(tt.path, tt.start)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if endIndex != tt.wantEnd {
				t.Fatalf("endIndex = %d, want %d", endIndex, tt.wantEnd)
			}
			if tt.wantIndex != nil {
				if token.index == nil {
					t.Fatalf("expected index %d, got nil", *tt.wantIndex)
				}
				if *token.index != *tt.wantIndex {
					t.Fatalf("index = %d, want %d", *token.index, *tt.wantIndex)
				}
			}
			if tt.wantProperty != "" {
				if token.property != tt.wantProperty {
					t.Fatalf("property = %q, want %q", token.property, tt.wantProperty)
				}
			}
		})
	}
}

func TestCompareJSONPathValue(t *testing.T) {
	tests := []struct {
		name       string
		actual     any
		exists     bool
		expectation jsonPathExpectation
		wantPass   bool
		wantReason string
		wantErr    bool
		errSubstr  string
	}{
		{
			name:        "exists comparator value present",
			actual:      "hello",
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorExists},
			wantPass:    true,
		},
		{
			name:        "exists comparator value missing",
			actual:      nil,
			exists:      false,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorExists},
			wantPass:    false,
			wantReason:  "did not resolve",
		},
		{
			name:        "equals matching",
			actual:      json.Number("42"),
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorEquals, Value: json.Number("42")},
			wantPass:    true,
		},
		{
			name:        "equals not matching",
			actual:      json.Number("42"),
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorEquals, Value: json.Number("99")},
			wantPass:    false,
			wantReason:  "did not equal",
		},
		{
			name:        "equals non-existent value",
			actual:      nil,
			exists:      false,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorEquals, Value: "abc"},
			wantPass:    false,
			wantReason:  "did not resolve",
		},
		{
			name:        "contains string match",
			actual:      "hello world",
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorContains, Value: "world"},
			wantPass:    true,
		},
		{
			name:        "contains string no match",
			actual:      "hello",
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorContains, Value: "xyz"},
			wantPass:    false,
			wantReason:  "did not contain",
		},
		{
			name:        "contains array match",
			actual:      []any{json.Number("1"), json.Number("2"), json.Number("3")},
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorContains, Value: json.Number("2")},
			wantPass:    true,
		},
		{
			name:        "contains array no match",
			actual:      []any{json.Number("1"), json.Number("2")},
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorContains, Value: json.Number("9")},
			wantPass:    false,
			wantReason:  "did not contain",
		},
		{
			name:        "greater_than pass",
			actual:      json.Number("10"),
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorGreater, Value: json.Number("5")},
			wantPass:    true,
		},
		{
			name:        "greater_than fail",
			actual:      json.Number("3"),
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorGreater, Value: json.Number("5")},
			wantPass:    false,
			wantReason:  "not greater than",
		},
		{
			name:        "greater_than non-numeric actual",
			actual:      "abc",
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorGreater, Value: json.Number("5")},
			wantErr:     true,
			errSubstr:   "actual value is not numeric",
		},
		{
			name:        "greater_than non-numeric expected",
			actual:      json.Number("10"),
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorGreater, Value: "abc"},
			wantErr:     true,
			errSubstr:   "expected value is not numeric",
		},
		{
			name:        "less_than pass",
			actual:      json.Number("3"),
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorLess, Value: json.Number("5")},
			wantPass:    true,
		},
		{
			name:        "less_than fail",
			actual:      json.Number("10"),
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorLess, Value: json.Number("5")},
			wantPass:    false,
			wantReason:  "not less than",
		},
		{
			name:        "less_than non-numeric actual",
			actual:      "abc",
			exists:      true,
			expectation: jsonPathExpectation{Path: "$.x", Comparator: jsonPathComparatorLess, Value: json.Number("5")},
			wantErr:     true,
			errSubstr:   "actual value is not numeric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pass, reason, err := compareJSONPathValue(tt.actual, tt.exists, tt.expectation)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pass != tt.wantPass {
				t.Fatalf("pass = %v, want %v", pass, tt.wantPass)
			}
			if tt.wantReason != "" && !strings.Contains(reason, tt.wantReason) {
				t.Fatalf("reason %q does not contain %q", reason, tt.wantReason)
			}
		})
	}
}

func TestJsonValueContains(t *testing.T) {
	tests := []struct {
		name       string
		actual     any
		expected   any
		wantPass   bool
		wantReason string
		wantErr    bool
		errSubstr  string
	}{
		{
			name:     "string contains substring",
			actual:   "hello world",
			expected: "world",
			wantPass: true,
		},
		{
			name:       "string does not contain",
			actual:     "hello",
			expected:   "xyz",
			wantPass:   false,
			wantReason: "did not contain",
		},
		{
			name:      "string contains non-string expected",
			actual:    "hello",
			expected:  42,
			wantErr:   true,
			errSubstr: "expected value must be a string",
		},
		{
			name:     "array contains element",
			actual:   []any{json.Number("1"), json.Number("2"), json.Number("3")},
			expected: json.Number("2"),
			wantPass: true,
		},
		{
			name:       "array does not contain",
			actual:     []any{json.Number("1"), json.Number("2")},
			expected:   json.Number("9"),
			wantPass:   false,
			wantReason: "did not contain",
		},
		{
			name:     "stringified fallback match",
			actual:   json.Number("42"),
			expected: json.Number("4"),
			wantPass: true,
		},
		{
			name:       "stringified fallback no match",
			actual:     json.Number("42"),
			expected:   json.Number("9"),
			wantPass:   false,
			wantReason: "did not contain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pass, reason, err := jsonValueContains(tt.actual, tt.expected)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Fatalf("error %q does not contain %q", err, tt.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pass != tt.wantPass {
				t.Fatalf("pass = %v, want %v", pass, tt.wantPass)
			}
			if tt.wantReason != "" && !strings.Contains(reason, tt.wantReason) {
				t.Fatalf("reason %q does not contain %q", reason, tt.wantReason)
			}
		})
	}
}

func TestJsonValuesEqual(t *testing.T) {
	tests := []struct {
		name  string
		left  any
		right any
		want  bool
	}{
		{
			name:  "identical strings",
			left:  "abc",
			right: "abc",
			want:  true,
		},
		{
			name:  "different strings",
			left:  "a",
			right: "b",
			want:  false,
		},
		{
			name:  "json.Number vs int64 after normalization",
			left:  json.Number("42"),
			right: json.Number("42"),
			want:  true,
		},
		{
			name:  "json.Number integer vs float",
			left:  json.Number("10"),
			right: json.Number("10.0"),
			want:  true,
		},
		{
			name:  "numeric precision edge case",
			left:  json.Number("96.12"),
			right: json.Number("96.124991"),
			want:  false,
		},
		{
			name:  "nil equality",
			left:  nil,
			right: nil,
			want:  true,
		},
		{
			name:  "nested objects equal",
			left:  map[string]any{"a": json.Number("1"), "b": "two"},
			right: map[string]any{"a": json.Number("1"), "b": "two"},
			want:  true,
		},
		{
			name:  "nested objects differ",
			left:  map[string]any{"a": json.Number("1")},
			right: map[string]any{"a": json.Number("2")},
			want:  false,
		},
		{
			name:  "number vs string",
			left:  json.Number("42"),
			right: "42",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonValuesEqual(tt.left, tt.right)
			if got != tt.want {
				t.Fatalf("jsonValuesEqual = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeJSONValue(t *testing.T) {
	tests := []struct {
		name  string
		input any
		check func(t *testing.T, value any)
	}{
		{
			name:  "json.Number integer becomes int64",
			input: json.Number("42"),
			check: func(t *testing.T, value any) {
				v, ok := value.(int64)
				if !ok {
					t.Fatalf("expected int64, got %T", value)
				}
				if v != 42 {
					t.Fatalf("expected 42, got %d", v)
				}
			},
		},
		{
			name:  "json.Number float becomes float64",
			input: json.Number("3.14"),
			check: func(t *testing.T, value any) {
				v, ok := value.(float64)
				if !ok {
					t.Fatalf("expected float64, got %T", value)
				}
				if v != 3.14 {
					t.Fatalf("expected 3.14, got %f", v)
				}
			},
		},
		{
			name:  "json.Number scientific notation",
			input: json.Number("1e2"),
			check: func(t *testing.T, value any) {
				v, ok := value.(float64)
				if !ok {
					t.Fatalf("expected float64, got %T", value)
				}
				if v != 100.0 {
					t.Fatalf("expected 100.0, got %f", v)
				}
			},
		},
		{
			name:  "array recursive normalization",
			input: []any{json.Number("1"), json.Number("2.5")},
			check: func(t *testing.T, value any) {
				arr, ok := value.([]any)
				if !ok || len(arr) != 2 {
					t.Fatalf("expected array of 2, got %v", value)
				}
				if _, ok := arr[0].(int64); !ok {
					t.Fatalf("arr[0] expected int64, got %T", arr[0])
				}
				if _, ok := arr[1].(float64); !ok {
					t.Fatalf("arr[1] expected float64, got %T", arr[1])
				}
			},
		},
		{
			name:  "map recursive normalization",
			input: map[string]any{"k": json.Number("7")},
			check: func(t *testing.T, value any) {
				m, ok := value.(map[string]any)
				if !ok {
					t.Fatalf("expected map, got %T", value)
				}
				if _, ok := m["k"].(int64); !ok {
					t.Fatalf("m[k] expected int64, got %T", m["k"])
				}
			},
		},
		{
			name:  "string passthrough",
			input: "hello",
			check: func(t *testing.T, value any) {
				if value != "hello" {
					t.Fatalf("expected hello, got %v", value)
				}
			},
		},
		{
			name:  "nil passthrough",
			input: nil,
			check: func(t *testing.T, value any) {
				if value != nil {
					t.Fatalf("expected nil, got %v", value)
				}
			},
		},
		{
			name:  "bool passthrough",
			input: true,
			check: func(t *testing.T, value any) {
				if value != true {
					t.Fatalf("expected true, got %v", value)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeJSONValue(tt.input)
			tt.check(t, result)
		})
	}
}

func TestStringifyJSONValue(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{name: "nil", input: nil, want: "null"},
		{name: "string", input: "hello", want: `"hello"`},
		{name: "number", input: 42, want: "42"},
		{name: "boolean", input: true, want: "true"},
		{name: "map", input: map[string]any{"a": 1}, want: `{"a":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringifyJSONValue(tt.input)
			if got != tt.want {
				t.Fatalf("stringifyJSONValue = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateJSONSchema(t *testing.T) {
	tests := []struct {
		name        string
		actual      string
		expected    string
		wantVerdict string
		wantScore   *float64
	}{
		{
			name:        "valid document passes schema",
			actual:      `{"answer":"done"}`,
			expected:    `{"type":"object","properties":{"answer":{"type":"string"}}}`,
			wantVerdict: "pass",
			wantScore:   floatPtr(1),
		},
		{
			name:        "invalid document fails schema",
			actual:      `{"answer":42}`,
			expected:    `{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","required":["answer","score"],"properties":{"answer":{"type":"string"},"score":{"type":"number"}}}`,
			wantVerdict: "fail",
			wantScore:   floatPtr(0),
		},
		{
			name:        "malformed actual JSON",
			actual:      `not json`,
			expected:    `{"type":"object"}`,
			wantVerdict: "error",
		},
		{
			name:        "malformed schema",
			actual:      `{}`,
			expected:    `not json`,
			wantVerdict: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := validateJSONSchema(tt.actual, tt.expected)
			if outcome.verdict != tt.wantVerdict {
				t.Fatalf("verdict = %q, want %q (reason: %s)", outcome.verdict, tt.wantVerdict, outcome.reason)
			}
			if tt.wantScore != nil {
				if outcome.normalizedScore == nil {
					t.Fatalf("expected score %f, got nil", *tt.wantScore)
				}
				if *outcome.normalizedScore != *tt.wantScore {
					t.Fatalf("score = %f, want %f", *outcome.normalizedScore, *tt.wantScore)
				}
			}
		})
	}
}

func TestValidateJSONPathMatch(t *testing.T) {
	tests := []struct {
		name        string
		actual      string
		expected    string
		wantVerdict string
		wantScore   *float64
	}{
		{
			name:        "matching path value",
			actual:      `{"status":"done","score":11}`,
			expected:    `{"path":"$.status","comparator":"equals","value":"done"}`,
			wantVerdict: "pass",
			wantScore:   floatPtr(1),
		},
		{
			name:        "non-matching path value",
			actual:      `{"status":"pending"}`,
			expected:    `{"path":"$.status","comparator":"equals","value":"done"}`,
			wantVerdict: "fail",
			wantScore:   floatPtr(0),
		},
		{
			name:        "exists check passes",
			actual:      `{"a":1}`,
			expected:    `$.a`,
			wantVerdict: "pass",
			wantScore:   floatPtr(1),
		},
		{
			name:        "exists check fails",
			actual:      `{"a":1}`,
			expected:    `$.b`,
			wantVerdict: "fail",
			wantScore:   floatPtr(0),
		},
		{
			name:        "malformed actual",
			actual:      `broken`,
			expected:    `$.a`,
			wantVerdict: "error",
		},
		{
			name:        "malformed expectation",
			actual:      `{}`,
			expected:    ``,
			wantVerdict: "error",
		},
		{
			name:        "numeric comparison greater_than",
			actual:      `{"score":95.5}`,
			expected:    `{"path":"$.score","comparator":"greater_than","value":90}`,
			wantVerdict: "pass",
			wantScore:   floatPtr(1),
		},
		{
			name:        "contains substring",
			actual:      `{"msg":"hello world"}`,
			expected:    `{"path":"$.msg","comparator":"contains","value":"world"}`,
			wantVerdict: "pass",
			wantScore:   floatPtr(1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := validateJSONPathMatch(tt.actual, tt.expected)
			if outcome.verdict != tt.wantVerdict {
				t.Fatalf("verdict = %q, want %q (reason: %s)", outcome.verdict, tt.wantVerdict, outcome.reason)
			}
			if tt.wantScore != nil {
				if outcome.normalizedScore == nil {
					t.Fatalf("expected score %f, got nil", *tt.wantScore)
				}
				if *outcome.normalizedScore != *tt.wantScore {
					t.Fatalf("score = %f, want %f", *outcome.normalizedScore, *tt.wantScore)
				}
			}
		})
	}
}

func TestMergeEvidence(t *testing.T) {
	tests := []struct {
		name  string
		base  map[string]any
		extra map[string]any
		check func(t *testing.T, result map[string]any)
	}{
		{
			name:  "empty extra returns base reference",
			base:  map[string]any{"a": 1},
			extra: map[string]any{},
			check: func(t *testing.T, result map[string]any) {
				if result["a"] != 1 {
					t.Fatalf("expected a=1, got %v", result["a"])
				}
			},
		},
		{
			name:  "no collision merges",
			base:  map[string]any{"a": 1},
			extra: map[string]any{"b": 2},
			check: func(t *testing.T, result map[string]any) {
				if result["a"] != 1 || result["b"] != 2 {
					t.Fatalf("expected merged map, got %v", result)
				}
			},
		},
		{
			name:  "collision adds evidence prefix",
			base:  map[string]any{"a": 1},
			extra: map[string]any{"a": 2},
			check: func(t *testing.T, result map[string]any) {
				if result["a"] != 1 {
					t.Fatalf("base key should be preserved, got %v", result["a"])
				}
				if result["evidence_a"] != 2 {
					t.Fatalf("collision key should be evidence_a=2, got %v", result["evidence_a"])
				}
			},
		},
		{
			name:  "both empty",
			base:  map[string]any{},
			extra: map[string]any{},
			check: func(t *testing.T, result map[string]any) {
				// empty extra returns base directly
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeEvidence(tt.base, tt.extra)
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func intPtr(v int) *int {
	return &v
}
