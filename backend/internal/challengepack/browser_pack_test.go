package challengepack

import (
	"os"
	"testing"
)

func TestParseBrowserNavigationSmokePack(t *testing.T) {
	content, err := os.ReadFile("../../../examples/challenge-packs/browser-navigation-smoke.yaml")
	if err != nil {
		t.Fatalf("read browser smoke pack: %v", err)
	}
	bundle, err := ParseYAML(content)
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
	if bundle.Pack.Slug != "browser-navigation-smoke" {
		t.Fatalf("slug = %q, want browser-navigation-smoke", bundle.Pack.Slug)
	}
	kinds, ok := bundle.Version.ToolPolicy["allowed_tool_kinds"].([]any)
	if !ok || len(kinds) != 1 || kinds[0] != "browser" {
		t.Fatalf("allowed_tool_kinds = %#v, want [browser]", bundle.Version.ToolPolicy["allowed_tool_kinds"])
	}
}
