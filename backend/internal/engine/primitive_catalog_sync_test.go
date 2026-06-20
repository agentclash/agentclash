package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/agentclash/agentclash/backend/internal/toolspec"
)

// TestPrimitiveCatalogMatchesNativePrimitives guards against drift between the
// engine's runtime primitive registry and the toolspec catalog the API exposes
// to the tool builder. If a primitive is added/changed in primitive_tools.go,
// toolspec.Primitives() must be updated to match (and vice versa).
func TestPrimitiveCatalogMatchesNativePrimitives(t *testing.T) {
	permissive := sandbox.ToolPolicy{
		AllowShell:       true,
		AllowedToolKinds: []string{toolKindFile, toolKindData, toolKindNetwork, toolKindBuild},
	}
	native := nativePrimitiveTools(permissive)

	catalog := map[string]toolspec.PrimitiveSpec{}
	for _, p := range toolspec.Primitives() {
		catalog[p.Name] = p
	}

	if len(native) != len(catalog) {
		t.Fatalf("primitive count mismatch: engine has %d, toolspec has %d", len(native), len(catalog))
	}

	for name, tool := range native {
		spec, ok := catalog[name]
		if !ok {
			t.Errorf("engine primitive %q missing from toolspec.Primitives()", name)
			continue
		}
		if spec.Description != tool.Description() {
			t.Errorf("primitive %q description drift:\n engine:   %q\n toolspec: %q", name, tool.Description(), spec.Description)
		}
		if !jsonEqual(t, spec.Parameters, tool.Parameters()) {
			t.Errorf("primitive %q parameters drift:\n engine:   %s\n toolspec: %s", name, tool.Parameters(), spec.Parameters)
		}
	}

	for name := range catalog {
		if _, ok := native[name]; !ok {
			t.Errorf("toolspec primitive %q not present in engine native primitives (under permissive policy)", name)
		}
	}
}

func jsonEqual(t *testing.T, a, b json.RawMessage) bool {
	t.Helper()
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		t.Fatalf("invalid JSON a: %v", err)
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		t.Fatalf("invalid JSON b: %v", err)
	}
	return reflect.DeepEqual(va, vb)
}
