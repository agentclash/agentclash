package challengepack

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
)

// catalogFS embeds the curated, ready-to-run challenge packs shipped as the
// product "library". Each file is a complete, runnable bundle (same schema as
// examples/challenge-packs/*.yaml) with small inline fixtures so it runs
// out-of-the-box and doubles as a template users clone and tweak. The catalog
// is the global, in-code analogue of StarterPieceLibrary (see library.go) but
// for full packs rather than individual pieces.
//
//go:embed catalog/*.yaml
var catalogFS embed.FS

const catalogDir = "catalog"

// CatalogCategory groups packs for the library gallery. Editorial, not
// load-bearing for execution.
const (
	CatalogCategoryEnterprise      = "enterprise"
	CatalogCategoryAgentCapability = "agent_capability"
	CatalogCategorySafety          = "safety"
)

// CatalogEntry is one curated pack in the library. Runnable fields (Slug, Name,
// Family, Description, ExecutionMode, Difficulty, SpecCard) are derived from the
// parsed Bundle so they can never drift from what actually runs; editorial
// fields (Category, Tags, EstimatedCostUSD, EstimatedRuntimeMS) come from the
// catalogMetadata registry below, since the bundle schema has no home for them.
type CatalogEntry struct {
	Slug               string   `json:"slug"`
	Name               string   `json:"name"`
	Family             string   `json:"family"`
	Category           string   `json:"category,omitempty"`
	Tags               []string `json:"tags,omitempty"`
	Description        string   `json:"description,omitempty"`
	Difficulty         string   `json:"difficulty,omitempty"`
	ExecutionMode      string   `json:"execution_mode"`
	EstimatedCostUSD   *float64 `json:"estimated_cost_usd,omitempty"`
	EstimatedRuntimeMS *int64   `json:"estimated_runtime_ms,omitempty"`
	SpecCard           SpecCard `json:"spec_card"`
	// YAML is the raw, runnable bundle source. Returned on the detail endpoint
	// and used verbatim by the instantiate path; omitted from list responses.
	YAML string `json:"yaml,omitempty"`
}

// Summary returns a copy of the entry without the heavy YAML body, for list
// responses.
func (e CatalogEntry) Summary() CatalogEntry {
	e.YAML = ""
	return e
}

// catalogMeta holds the editorial fields that have no representation in the
// runnable bundle schema.
type catalogMeta struct {
	Category     string
	Tags         []string
	EstCostUSD   *float64
	EstRuntimeMS *int64
}

// catalogMetadata is keyed by pack slug. Every embedded catalog file must have
// an entry here and vice versa — enforced by TestCatalog* in catalog_test.go.
var catalogMetadata = map[string]catalogMeta{
	"tool-calling-accuracy": {
		Category:     CatalogCategoryAgentCapability,
		Tags:         []string{"tool-use", "function-calling", "agents"},
		EstCostUSD:   catalogFloatPtr(0.02),
		EstRuntimeMS: catalogIntPtr(30000),
	},
	"text-to-sql": {
		Category:     CatalogCategoryEnterprise,
		Tags:         []string{"sql", "data-analysis", "code-execution"},
		EstCostUSD:   catalogFloatPtr(0.03),
		EstRuntimeMS: catalogIntPtr(45000),
	},
	"document-extraction": {
		Category:     CatalogCategoryEnterprise,
		Tags:         []string{"extraction", "json", "documents"},
		EstCostUSD:   catalogFloatPtr(0.01),
		EstRuntimeMS: catalogIntPtr(15000),
	},
	"json-output-conformance": {
		Category:     CatalogCategoryAgentCapability,
		Tags:         []string{"structured-output", "json", "regression"},
		EstCostUSD:   catalogFloatPtr(0.005),
		EstRuntimeMS: catalogIntPtr(10000),
	},
	"customer-support-policy": {
		Category:     CatalogCategoryEnterprise,
		Tags:         []string{"support", "multi-turn", "policy", "pii"},
		EstCostUSD:   catalogFloatPtr(0.08),
		EstRuntimeMS: catalogIntPtr(90000),
	},
	"swe-bug-fix": {
		Category:     CatalogCategoryEnterprise,
		Tags:         []string{"coding", "swe", "code-execution"},
		EstCostUSD:   catalogFloatPtr(0.05),
		EstRuntimeMS: catalogIntPtr(60000),
	},
	"rag-faithfulness": {
		Category:     CatalogCategoryEnterprise,
		Tags:         []string{"rag", "faithfulness", "citations"},
		EstCostUSD:   catalogFloatPtr(0.03),
		EstRuntimeMS: catalogIntPtr(25000),
	},
	"summarization-faithfulness": {
		Category:     CatalogCategoryEnterprise,
		Tags:         []string{"summarization", "faithfulness", "content"},
		EstCostUSD:   catalogFloatPtr(0.02),
		EstRuntimeMS: catalogIntPtr(20000),
	},
	"it-helpdesk-triage": {
		Category:     CatalogCategoryEnterprise,
		Tags:         []string{"it-ops", "tools", "safety"},
		EstCostUSD:   catalogFloatPtr(0.04),
		EstRuntimeMS: catalogIntPtr(40000),
	},
	"prompt-injection-defense": {
		Category:     CatalogCategorySafety,
		Tags:         []string{"security", "prompt-injection", "exfiltration"},
		EstCostUSD:   catalogFloatPtr(0.03),
		EstRuntimeMS: catalogIntPtr(45000),
	},
	"jailbreak-refusal": {
		Category:     CatalogCategorySafety,
		Tags:         []string{"security", "jailbreak", "refusal"},
		EstCostUSD:   catalogFloatPtr(0.03),
		EstRuntimeMS: catalogIntPtr(45000),
	},
	"knowledge-reasoning-regression": {
		Category:     CatalogCategoryAgentCapability,
		Tags:         []string{"reasoning", "knowledge", "regression"},
		EstCostUSD:   catalogFloatPtr(0.01),
		EstRuntimeMS: catalogIntPtr(15000),
	},
}

var (
	catalogOnce    sync.Once
	catalogEntries []CatalogEntry
	catalogErr     error
)

// Catalog returns the curated library packs, sorted by category then name.
// Parsed and validated once, then cached.
func Catalog() ([]CatalogEntry, error) {
	catalogOnce.Do(func() {
		catalogEntries, catalogErr = loadCatalog()
	})
	if catalogErr != nil {
		return nil, catalogErr
	}
	out := make([]CatalogEntry, len(catalogEntries))
	copy(out, catalogEntries)
	return out, nil
}

// CatalogBySlug returns the full entry (including YAML) for a slug.
func CatalogBySlug(slug string) (CatalogEntry, bool, error) {
	entries, err := Catalog()
	if err != nil {
		return CatalogEntry{}, false, err
	}
	slug = strings.TrimSpace(slug)
	for _, entry := range entries {
		if entry.Slug == slug {
			return entry, true, nil
		}
	}
	return CatalogEntry{}, false, nil
}

func loadCatalog() ([]CatalogEntry, error) {
	dirEntries, err := fs.ReadDir(catalogFS, catalogDir)
	if err != nil {
		return nil, fmt.Errorf("read catalog dir: %w", err)
	}

	entries := make([]CatalogEntry, 0, len(dirEntries))
	seen := map[string]struct{}{}
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() || !strings.HasSuffix(dirEntry.Name(), ".yaml") {
			continue
		}
		data, err := catalogFS.ReadFile(catalogDir + "/" + dirEntry.Name())
		if err != nil {
			return nil, fmt.Errorf("read catalog file %s: %w", dirEntry.Name(), err)
		}
		// ParseYAML runs ValidateBundle, so an invalid catalog pack fails loudly
		// here (and in TestCatalog*) rather than at instantiate time.
		bundle, err := ParseYAML(data)
		if err != nil {
			return nil, fmt.Errorf("catalog file %s: %w", dirEntry.Name(), err)
		}
		if _, exists := seen[bundle.Pack.Slug]; exists {
			return nil, fmt.Errorf("catalog file %s: duplicate pack slug %q", dirEntry.Name(), bundle.Pack.Slug)
		}
		seen[bundle.Pack.Slug] = struct{}{}

		description := ""
		if bundle.Pack.Description != nil {
			description = strings.TrimSpace(*bundle.Pack.Description)
		}
		meta := catalogMetadata[bundle.Pack.Slug]

		entries = append(entries, CatalogEntry{
			Slug:               bundle.Pack.Slug,
			Name:               bundle.Pack.Name,
			Family:             bundle.Pack.Family,
			Category:           meta.Category,
			Tags:               meta.Tags,
			Description:        description,
			Difficulty:         maxDifficulty(bundle.Challenges),
			ExecutionMode:      effectiveExecutionMode(bundle.Version.ExecutionMode),
			EstimatedCostUSD:   meta.EstCostUSD,
			EstimatedRuntimeMS: meta.EstRuntimeMS,
			SpecCard:           SpecCardSummary(bundle),
			YAML:               string(data),
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		ri, rj := categoryRank(entries[i].Category), categoryRank(entries[j].Category)
		if ri != rj {
			return ri < rj
		}
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func effectiveExecutionMode(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return ExecutionModeNative
	}
	return mode
}

var difficultyRank = map[string]int{"easy": 1, "medium": 2, "hard": 3, "expert": 4}

func maxDifficulty(challenges []ChallengeDefinition) string {
	best := ""
	bestRank := 0
	for _, c := range challenges {
		if r := difficultyRank[c.Difficulty]; r > bestRank {
			bestRank = r
			best = c.Difficulty
		}
	}
	return best
}

var categoryOrder = map[string]int{
	CatalogCategoryEnterprise:      0,
	CatalogCategoryAgentCapability: 1,
	CatalogCategorySafety:          2,
}

func categoryRank(category string) int {
	if rank, ok := categoryOrder[category]; ok {
		return rank
	}
	return len(categoryOrder)
}

func catalogFloatPtr(v float64) *float64 { return &v }
func catalogIntPtr(v int64) *int64       { return &v }
