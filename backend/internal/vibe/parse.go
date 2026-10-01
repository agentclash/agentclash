package vibe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ValidateJSON walks tokens before typed decoding. No allocation proportional to
// an attacker-declared array length, no duplicate fields, no recursive refs.
func ValidateJSON(raw []byte, l Limits) error {
	if len(raw) > l.FileBytes || !utf8.Valid(raw) {
		return fault("invalid_import", "Content is too large or is not valid UTF-8.")
	}
	if err := validSurrogates(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	nodes := 0
	var value func(int) error
	value = func(depth int) error {
		nodes++
		if depth > l.Depth || nodes > l.Nodes {
			return fault("import_limit", "Structured content is too deep or contains too many values.")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch v := t.(type) {
		case json.Delim:
			count := 0
			seen := map[string]bool{}
			for d.More() {
				count++
				if v == '{' {
					key, e := d.Token()
					if e != nil {
						return e
					}
					k, ok := key.(string)
					if !ok || len(k) > MaxKeyBytes || seen[k] || count > l.Keys {
						return fault("import_limit", "Object keys must be unique and bounded.")
					}
					seen[k] = true
					if k == "$ref" || k == "$dynamicRef" || k == "$recursiveRef" {
						return fault("unsupported_schema", "Schema references are not supported by Vibe imports; inline the schema.")
					}
				} else if v != '[' || count > l.Array {
					return fault("import_limit", "Array is too large.")
				}
				if err := value(depth + 1); err != nil {
					return err
				}
			}
			end, e := d.Token()
			if e != nil {
				return e
			}
			if (v == '{' && end != json.Delim('}')) || (v == '[' && end != json.Delim(']')) {
				return fmt.Errorf("invalid container")
			}
		case string:
			if len(v) > l.StringBytes {
				return fault("import_limit", "A structured field is too long.")
			}
		case json.Number:
			if len(v) > MaxNumberBytes {
				return fault("import_limit", "A number is too long.")
			}
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fault("invalid_import", "Expected exactly one JSON document.")
	}
	return nil
}

// encoding/json replaces lone UTF-16 surrogates. Reject them instead, preserving
// adversarial strings byte-for-byte through accepted structured imports.
func validSurrogates(raw []byte) error {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			break
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return fmt.Errorf("invalid unicode escape")
		}
		n, e := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if e != nil {
			return e
		}
		i += 4
		if n >= 0xDC00 && n <= 0xDFFF {
			return fault("invalid_encoding", "Unpaired unicode surrogate.")
		}
		if n >= 0xD800 && n <= 0xDBFF {
			if i+6 >= len(raw) || string(raw[i+1:i+3]) != "\\u" {
				return fault("invalid_encoding", "Unpaired unicode surrogate.")
			}
			lo, e := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if e != nil || lo < 0xDC00 || lo > 0xDFFF {
				return fault("invalid_encoding", "Unpaired unicode surrogate.")
			}
			i += 6
		}
	}
	return nil
}

func Decode(raw []byte, l Limits, out any) error {
	if err := ValidateJSON(raw, l); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	d.UseNumber()
	return d.Decode(out)
}

// YAML is parsed into an unexpanded node tree, never straight into a map. Aliases,
// custom tags and merge keys are rejected before conversion. The raw input byte
// limit bounds the parser's work; callers never accept compressed uploads.
func ImportJSON(raw []byte, l Limits) ([]byte, error) {
	if len(raw) > l.FileBytes || !utf8.Valid(raw) {
		return nil, fault("import_limit", "Import is too large or has invalid encoding.")
	}
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "{") || strings.HasPrefix(strings.TrimSpace(string(raw)), "[") {
		return raw, ValidateJSON(raw, l)
	}
	if err := preflightYAML(raw, l); err != nil {
		return nil, err
	}
	// Reject YAML features lexically before parsing as well as in the node walk.
	// Plain strings containing these characters remain supported via JSON imports.
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "&") || strings.Contains(line, "*") || strings.Contains(line, "!") {
			return nil, fault("unsupported_yaml", "YAML anchors, aliases and tags are disabled. Use JSON for literal text containing &, * or !.")
		}
	}
	var root yaml.Node
	d := yaml.NewDecoder(bytes.NewReader(raw))
	if err := d.Decode(&root); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := d.Decode(&extra); err != io.EOF {
		return nil, fault("invalid_import", "Only one YAML document is allowed.")
	}
	nodes := 0
	var walk func(*yaml.Node, int) (any, error)
	walk = func(n *yaml.Node, depth int) (any, error) {
		nodes++
		if depth > l.Depth || nodes > l.Nodes || len(n.Value) > l.StringBytes {
			return nil, fault("import_limit", "YAML content exceeds structural limits.")
		}
		if n.Anchor != "" || n.Kind == yaml.AliasNode {
			return nil, fault("unsupported_yaml", "YAML aliases are disabled.")
		}
		switch n.Kind {
		case yaml.DocumentNode:
			if len(n.Content) != 1 {
				return nil, fmt.Errorf("empty YAML")
			}
			return walk(n.Content[0], depth)
		case yaml.MappingNode:
			if len(n.Content)/2 > l.Keys {
				return nil, fault("import_limit", "Too many object keys.")
			}
			m := map[string]any{}
			for i := 0; i < len(n.Content); i += 2 {
				k := n.Content[i]
				if k.Kind != yaml.ScalarNode || k.Tag != "!!str" || k.Value == "<<" || len(k.Value) > MaxKeyBytes {
					return nil, fmt.Errorf("invalid YAML key")
				}
				if _, ok := m[k.Value]; ok {
					return nil, fmt.Errorf("duplicate YAML key")
				}
				v, e := walk(n.Content[i+1], depth+1)
				if e != nil {
					return nil, e
				}
				m[k.Value] = v
			}
			return m, nil
		case yaml.SequenceNode:
			if len(n.Content) > l.Array {
				return nil, fault("import_limit", "Too many array entries.")
			}
			a := make([]any, 0, len(n.Content))
			for _, c := range n.Content {
				v, e := walk(c, depth+1)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			return a, nil
		case yaml.ScalarNode:
			switch n.Tag {
			case "!!str":
				return n.Value, nil
			case "!!null":
				return nil, nil
			case "!!bool":
				return strconv.ParseBool(n.Value)
			case "!!int", "!!float":
				if len(n.Value) > MaxNumberBytes || !json.Valid([]byte(n.Value)) {
					return nil, fmt.Errorf("use JSON-compatible numbers")
				}
				return json.Number(n.Value), nil
			}
		}
		return nil, fault("unsupported_yaml", "Unsupported YAML value.")
	}
	v, err := walk(&root, 0)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, ValidateJSON(b, l)
}

// Bound parser work BEFORE yaml.v3 builds nodes. This intentionally accepts a
// conservative YAML subset; JSON is the lossless escape hatch for literal YAML
// punctuation. No alias expansion, unlimited indentation or huge flow graphs
// can reach the YAML parser, even if the eventual typed value would be small.
func preflightYAML(raw []byte, l Limits) error {
	flow, nodes := 0, 0
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		indent := len(line) - len(bytes.TrimLeft(line, " "))
		if indent > l.Depth*2 || bytes.Contains(line, []byte{'\t'}) {
			return fault("import_limit", "YAML indentation exceeds the supported nesting limit. Use JSON for complex documents.")
		}
		nodes++
		for _, c := range line {
			switch c {
			case '[', '{':
				flow++
				nodes++
			case ']', '}':
				flow--
			case ',', ':':
				nodes++
			}
			if flow > l.Depth || nodes > l.Nodes {
				return fault("import_limit", "YAML flow nesting or node count exceeds its parser limit.")
			}
		}
	}
	return nil
}
