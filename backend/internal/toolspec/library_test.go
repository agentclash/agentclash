package toolspec

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// TestLibraryEntriesValidate is the safety net for the catalog: every shipped
// entry must be a valid tool definition so an "Add from library" never fails at
// the user's save. It also enforces basic catalog hygiene.
func TestLibraryEntriesValidate(t *testing.T) {
	cats := map[string]struct{}{}
	for _, c := range LibraryCategories() {
		cats[c] = struct{}{}
	}

	slugs := map[string]struct{}{}
	for _, e := range Library() {
		if e.Slug == "" {
			t.Errorf("entry %q has an empty slug", e.Name)
			continue
		}
		if _, dup := slugs[e.Slug]; dup {
			t.Errorf("duplicate library slug %q", e.Slug)
		}
		slugs[e.Slug] = struct{}{}

		if e.Name == "" {
			t.Errorf("%s: empty name", e.Slug)
		}
		if e.Description == "" {
			t.Errorf("%s: empty description", e.Slug)
		}
		if _, ok := cats[e.Category]; !ok {
			t.Errorf("%s: unknown category %q", e.Slug, e.Category)
		}
		if e.ToolKind != ToolTypePrimitive {
			t.Errorf("%s: tool_kind = %q, want %q", e.Slug, e.ToolKind, ToolTypePrimitive)
		}
		if e.Delivery != DeliveryLive && e.Delivery != DeliveryMock {
			t.Errorf("%s: delivery = %q, want live or mock", e.Slug, e.Delivery)
		}

		if errs := ValidateDefinition(e.ToolKind, e.Definition, ValidateOptions{}); len(errs) > 0 {
			t.Errorf("%s: definition invalid: %v", e.Slug, errs)
		}

		// buildLibrary injects the entry description into the stored definition,
		// so an added tool carries a human description into the builder.
		var d struct {
			Description string `json:"description"`
		}
		_ = json.Unmarshal(e.Definition, &d)
		if d.Description != e.Description {
			t.Errorf("%s: definition.description = %q, want %q", e.Slug, d.Description, e.Description)
		}

		if e.HasLive() {
			if errs := ValidateDefinition(e.ToolKind, e.Live, ValidateOptions{}); len(errs) > 0 {
				t.Errorf("%s: live definition invalid: %v", e.Slug, errs)
			}
			if e.RequiresSecret == "" {
				t.Errorf("%s: bundles a live variant but declares no RequiresSecret", e.Slug)
			}
		}
	}

	if len(slugs) < 50 {
		t.Errorf("library has %d entries; expected at least 50", len(slugs))
	}
}

func TestLibraryBySlug(t *testing.T) {
	if _, ok := LibraryBySlug("web-search"); !ok {
		t.Fatal("expected web-search in the library")
	}
	if _, ok := LibraryBySlug("definitely-not-a-tool"); ok {
		t.Fatal("expected a miss for an unknown slug")
	}
}

func TestLibraryDelegateDefinitionsOnlyReferenceRequiredParams(t *testing.T) {
	for _, entry := range Library() {
		definitions := map[string]json.RawMessage{"default": entry.Definition}
		if entry.HasLive() {
			definitions["live"] = entry.Live
		}
		for variant, raw := range definitions {
			var definition struct {
				Parameters struct {
					Required []string `json:"required"`
				} `json:"parameters"`
				Implementation struct {
					Mode string          `json:"mode"`
					Args json.RawMessage `json:"args"`
				} `json:"implementation"`
			}
			if err := json.Unmarshal(raw, &definition); err != nil {
				t.Fatalf("%s/%s: decode definition: %v", entry.Slug, variant, err)
			}
			if definition.Implementation.Mode != ModeDelegate {
				continue
			}
			required := make(map[string]struct{}, len(definition.Parameters.Required))
			for _, name := range definition.Parameters.Required {
				required[name] = struct{}{}
			}
			for _, placeholder := range placeholdersIn(definition.Implementation.Args) {
				reference := unwrapTemplateEncoding(strings.TrimSpace(placeholder))
				if reference == "parameters" || strings.HasPrefix(reference, "secrets.") {
					continue
				}
				if _, ok := required[reference]; !ok {
					t.Errorf("%s/%s: delegate arg references optional parameter %q; omitted optional inputs fail runtime resolution", entry.Slug, variant, reference)
				}
			}
		}
	}
}

func TestLibraryLiveDefinitionsKeepSecretsOutOfURLs(t *testing.T) {
	for _, entry := range Library() {
		if !entry.HasLive() {
			continue
		}
		var definition struct {
			Implementation struct {
				Args struct {
					URL string `json:"url"`
				} `json:"args"`
			} `json:"implementation"`
		}
		if err := json.Unmarshal(entry.Live, &definition); err != nil {
			t.Fatalf("%s: decode live definition: %v", entry.Slug, err)
		}
		if strings.Contains(definition.Implementation.Args.URL, "${secrets.") {
			t.Errorf("%s: live URL contains a secret placeholder that the HTTP response URL can expose", entry.Slug)
		}
	}
}

func TestLibraryLiveRequestBodiesDeclareContentType(t *testing.T) {
	for _, entry := range Library() {
		if !entry.HasLive() {
			continue
		}
		var definition struct {
			Implementation struct {
				Args struct {
					Body    string            `json:"body"`
					Headers map[string]string `json:"headers"`
				} `json:"args"`
			} `json:"implementation"`
		}
		if err := json.Unmarshal(entry.Live, &definition); err != nil {
			t.Fatalf("%s: decode live definition: %v", entry.Slug, err)
		}
		if definition.Implementation.Args.Body != "" && definition.Implementation.Args.Headers["Content-Type"] == "" {
			t.Errorf("%s: live request has a body but no Content-Type header", entry.Slug)
		}
	}
}

func TestSafeCalcScript(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}

	for _, tc := range []struct {
		name       string
		expression string
		want       string
		wantError  bool
	}{
		{name: "arithmetic", expression: "(2 + 3) * 4", want: "20"},
		{name: "code execution", expression: `__import__("os").getcwd()`, wantError: true},
		{name: "boolean", expression: "True", wantError: true},
		{name: "oversized power", expression: "9 ** 999999999", wantError: true},
		{name: "non finite", expression: "1e309", wantError: true},
		{name: "too long", expression: strings.Repeat("1+", 130) + "1", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := exec.Command("python3", "-c", safeCalcScript, tc.expression).CombinedOutput()
			if tc.wantError {
				if err == nil {
					t.Fatalf("expression unexpectedly succeeded: %s", output)
				}
				return
			}
			if err != nil {
				t.Fatalf("expression failed: %v: %s", err, output)
			}
			if strings.TrimSpace(string(output)) != tc.want {
				t.Fatalf("output = %q, want %q", output, tc.want)
			}
		})
	}
}

func TestGenerateUUIDUsesInstalledSandboxRuntime(t *testing.T) {
	entry, ok := LibraryBySlug("generate-uuid")
	if !ok {
		t.Fatal("generate-uuid entry is missing")
	}
	var definition struct {
		Implementation struct {
			Args struct {
				Command []string `json:"command"`
			} `json:"args"`
		} `json:"implementation"`
	}
	if err := json.Unmarshal(entry.Definition, &definition); err != nil {
		t.Fatal(err)
	}
	command := definition.Implementation.Args.Command
	if len(command) < 3 || command[0] != "python3" || !strings.Contains(command[2], "uuid.uuid4") {
		t.Fatalf("command = %#v, want Python stdlib UUID generation", command)
	}
}
