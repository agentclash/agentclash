package challengepack

import "testing"

// TestCatalogLoadsAndIsRunnable is the runnability gate for the curated library:
// every embedded pack must parse, validate, compose a manifest (the exact gate
// the instantiate path hits via PublishChallengePackBundle), and produce a
// usable spec card. A content bug fails CI here instead of at instantiate time.
func TestCatalogLoadsAndIsRunnable(t *testing.T) {
	entries, err := Catalog()
	if err != nil {
		t.Fatalf("Catalog() error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("Catalog() returned no entries")
	}

	for _, entry := range entries {
		t.Run(entry.Slug, func(t *testing.T) {
			if entry.Slug == "" {
				t.Fatal("entry has empty slug")
			}
			if entry.Name == "" {
				t.Error("entry has empty name")
			}
			if entry.Family == "" {
				t.Error("entry has empty family")
			}
			if entry.Category == "" {
				t.Error("entry has empty category (missing catalogMetadata?)")
			}
			if entry.ExecutionMode == "" {
				t.Error("entry has empty execution_mode")
			}
			if entry.YAML == "" {
				t.Fatal("entry has empty YAML")
			}

			// Re-parse + manifest: this is exactly what PublishBundle does, so a
			// pass here proves the pack is instantiable.
			bundle, err := ParseYAML([]byte(entry.YAML))
			if err != nil {
				t.Fatalf("ParseYAML: %v", err)
			}
			if _, err := ManifestJSON(bundle); err != nil {
				t.Fatalf("ManifestJSON: %v", err)
			}
			if len(entry.SpecCard.Dimensions) == 0 {
				t.Error("spec card has no scoring dimensions")
			}
		})
	}
}

// TestCatalogMetadataMatchesFiles enforces the 1:1 relationship between embedded
// pack files and the editorial catalogMetadata registry, so neither can drift.
func TestCatalogMetadataMatchesFiles(t *testing.T) {
	entries, err := Catalog()
	if err != nil {
		t.Fatalf("Catalog() error: %v", err)
	}

	slugs := map[string]struct{}{}
	for _, entry := range entries {
		slugs[entry.Slug] = struct{}{}
		if _, ok := catalogMetadata[entry.Slug]; !ok {
			t.Errorf("pack %q has no catalogMetadata entry", entry.Slug)
		}
	}
	for slug := range catalogMetadata {
		if _, ok := slugs[slug]; !ok {
			t.Errorf("catalogMetadata has entry %q with no matching catalog file", slug)
		}
	}
}

// TestCatalogPacksAreAssetFree guards the v1 constraint that catalog packs use
// only inline fixtures. Artifact-backed assets cannot be instantiated into a
// workspace (validateStoredAssetReferences requires the artifact to already
// exist and belong to that workspace), so a catalog pack that references one
// would fail at instantiate time.
func TestCatalogPacksAreAssetFree(t *testing.T) {
	entries, err := Catalog()
	if err != nil {
		t.Fatalf("Catalog() error: %v", err)
	}
	for _, entry := range entries {
		bundle, err := ParseYAML([]byte(entry.YAML))
		if err != nil {
			t.Fatalf("%s: ParseYAML: %v", entry.Slug, err)
		}
		if len(bundle.Version.Assets) != 0 {
			t.Errorf("%s: declares version assets; catalog packs must be asset-free", entry.Slug)
		}
		for _, ch := range bundle.Challenges {
			if len(ch.Assets) != 0 || len(ch.ArtifactRefs) != 0 {
				t.Errorf("%s: challenge %q declares assets/artifact_refs", entry.Slug, ch.Key)
			}
		}
		for _, set := range bundle.InputSets {
			for _, c := range set.Cases {
				if len(c.Assets) != 0 || len(c.Artifacts) != 0 {
					t.Errorf("%s: case %q declares assets/artifacts", entry.Slug, c.EffectiveKey())
				}
			}
		}
	}
}

func TestCatalogBySlug(t *testing.T) {
	entry, ok, err := CatalogBySlug("json-output-conformance")
	if err != nil {
		t.Fatalf("CatalogBySlug error: %v", err)
	}
	if !ok {
		t.Fatal("expected json-output-conformance to be in the catalog")
	}
	if entry.YAML == "" {
		t.Error("CatalogBySlug should return the full entry including YAML")
	}

	if _, ok, _ := CatalogBySlug("does-not-exist"); ok {
		t.Error("unexpected hit for unknown slug")
	}
}
