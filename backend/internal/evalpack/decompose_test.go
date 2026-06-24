package evalpack

import (
	"bytes"
	"testing"
)

// TestBundleToCompositionRoundTripsCatalog proves edit-in-builder is lossless:
// decompiling every catalog pack and recomposing it must reproduce the exact
// runnable manifest. This also makes "every catalog pack is editable in the
// builder" a tested invariant. The catalog packs collectively exercise the
// advanced passthrough: custom tools, tool_policy, sandbox, metrics,
// post_execution_checks, and runtime_limits.
func TestBundleToCompositionRoundTripsCatalog(t *testing.T) {
	entries, err := Catalog()
	if err != nil {
		t.Fatalf("Catalog(): %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no catalog entries")
	}

	for _, entry := range entries {
		t.Run(entry.Slug, func(t *testing.T) {
			bundle, err := ParseYAML([]byte(entry.YAML))
			if err != nil {
				t.Fatalf("ParseYAML: %v", err)
			}
			want, err := ManifestJSON(bundle)
			if err != nil {
				t.Fatalf("ManifestJSON(bundle): %v", err)
			}

			comp, err := BundleToComposition(bundle)
			if err != nil {
				t.Fatalf("BundleToComposition: %v", err)
			}
			recomposed, err := ComposeBundle(comp, nil)
			if err != nil {
				t.Fatalf("ComposeBundle: %v", err)
			}
			got, err := ManifestJSON(recomposed)
			if err != nil {
				t.Fatalf("ManifestJSON(recomposed): %v", err)
			}

			if !bytes.Equal(want, got) {
				t.Errorf("round-trip manifest mismatch for %s\nwant: %s\ngot:  %s", entry.Slug, want, got)
			}
		})
	}
}

// TestManifestToBundleRoundTripsCatalog proves the manifest reconstruction used
// to hydrate a builder draft from a published pack is loss-free: re-deriving a
// bundle from a stored manifest yields the identical manifest.
func TestManifestToBundleRoundTripsCatalog(t *testing.T) {
	entries, err := Catalog()
	if err != nil {
		t.Fatalf("Catalog(): %v", err)
	}
	for _, entry := range entries {
		t.Run(entry.Slug, func(t *testing.T) {
			bundle, err := ParseYAML([]byte(entry.YAML))
			if err != nil {
				t.Fatalf("ParseYAML: %v", err)
			}
			manifest, err := ManifestJSON(bundle)
			if err != nil {
				t.Fatalf("ManifestJSON: %v", err)
			}
			rebuilt, err := ManifestToBundle(manifest)
			if err != nil {
				t.Fatalf("ManifestToBundle: %v", err)
			}
			again, err := ManifestJSON(rebuilt)
			if err != nil {
				t.Fatalf("ManifestJSON(rebuilt): %v", err)
			}
			if !bytes.Equal(manifest, again) {
				t.Errorf("manifest round-trip mismatch for %s\nwant: %s\ngot:  %s", entry.Slug, manifest, again)
			}
		})
	}
}

// TestBundleToCompositionPreservesAdvancedFields guards the specific fields the
// builder UI cannot edit yet — they must survive decompile so a re-publish
// doesn't silently strip a pack's tools / runtime limits / capture checks.
func TestBundleToCompositionPreservesAdvancedFields(t *testing.T) {
	entry, ok, err := CatalogBySlug("text-to-sql")
	if err != nil || !ok {
		t.Fatalf("CatalogBySlug(text-to-sql): ok=%v err=%v", ok, err)
	}
	bundle, err := ParseYAML([]byte(entry.YAML))
	if err != nil {
		t.Fatalf("ParseYAML: %v", err)
	}

	comp, err := BundleToComposition(bundle)
	if err != nil {
		t.Fatalf("BundleToComposition: %v", err)
	}
	if comp.Advanced == nil {
		t.Fatal("expected Advanced to be populated for a pack with post_execution_checks + runtime_limits")
	}
	if len(comp.Advanced.PostExecutionChecks) == 0 {
		t.Error("post_execution_checks were dropped")
	}
	if comp.Advanced.RuntimeLimits.MaxDurationMS == nil {
		t.Error("runtime_limits were dropped")
	}
	if len(comp.Advanced.Metrics) == 0 {
		t.Error("metrics were dropped")
	}
}

// TestBundleToCompositionInlinesEveryPiece confirms pieces come back as inline
// definitions (no dangling library ref ids), so a hydrated draft composes
// without needing the original workspace pieces.
func TestBundleToCompositionInlinesEveryPiece(t *testing.T) {
	entry, ok, err := CatalogBySlug("tool-calling-accuracy")
	if err != nil || !ok {
		t.Fatalf("CatalogBySlug: ok=%v err=%v", ok, err)
	}
	bundle, err := ParseYAML([]byte(entry.YAML))
	if err != nil {
		t.Fatalf("ParseYAML: %v", err)
	}
	comp, err := BundleToComposition(bundle)
	if err != nil {
		t.Fatalf("BundleToComposition: %v", err)
	}

	groups := [][]PieceRef{comp.Challenges, comp.InputSets, comp.Validators, comp.Judges}
	for _, group := range groups {
		for _, ref := range group {
			if ref.RefID != nil {
				t.Errorf("piece unexpectedly references a library id: %v", ref.RefID)
			}
			if len(ref.Inline) == 0 {
				t.Error("piece has no inline definition")
			}
		}
	}
	if len(comp.Validators) == 0 {
		t.Error("expected inlined validators")
	}
}
