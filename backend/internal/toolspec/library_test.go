package toolspec

import (
	"encoding/json"
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
