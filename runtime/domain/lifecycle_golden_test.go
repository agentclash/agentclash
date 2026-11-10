package domain

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateWebLifecycles = flag.Bool(
	"update",
	false,
	"rewrite the web copy of the status lifecycles (web/src/lib/generated/status-lifecycles.json)",
)

// webLifecyclesRegenerateCommand is the command that rewrites the web copy.
// Keep it in step with the `_generated` note and docs/state-machines.md.
const webLifecyclesRegenerateCommand = "cd runtime && go test ./domain -run TestWebStatusLifecyclesMatchDomain -update"

// webLifecyclesPath is relative to this package directory (runtime/domain).
var webLifecyclesPath = filepath.Join("..", "..", "web", "src", "lib", "generated", "status-lifecycles.json")

type webLifecycleFile struct {
	Generated  string                  `json:"_generated"`
	Lifecycles map[string]webLifecycle `json:"lifecycles"`
}

type webLifecycle struct {
	States      []string            `json:"states"`
	Terminal    []string            `json:"terminal"`
	Transitions map[string][]string `json:"transitions"`
}

func webLifecyclesJSON(t *testing.T) []byte {
	t.Helper()
	file := webLifecycleFile{
		Generated: "Generated from domain.Lifecycles() in runtime/domain/lifecycle.go. " +
			"Do not edit. Regenerate with: " + webLifecyclesRegenerateCommand,
		Lifecycles: map[string]webLifecycle{},
	}
	for _, lifecycle := range Lifecycles() {
		file.Lifecycles[lifecycle.Resource] = webLifecycle{
			States:      lifecycle.States,
			Terminal:    lifecycle.Terminal,
			Transitions: lifecycle.Transitions,
		}
	}
	// encoding/json sorts map keys, so the output is stable. Encode ends the
	// output with a newline.
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(file); err != nil {
		t.Fatalf("marshal web lifecycles: %v", err)
	}
	return out.Bytes()
}

// TestWebStatusLifecyclesMatchDomain keeps the web copy of the status
// lifecycles equal to the transition tables. The web app reads the copy
// through web/src/lib/status-lifecycles.ts.
func TestWebStatusLifecyclesMatchDomain(t *testing.T) {
	got := webLifecyclesJSON(t)

	if *updateWebLifecycles {
		if err := os.MkdirAll(filepath.Dir(webLifecyclesPath), 0o755); err != nil {
			t.Fatalf("create %s: %v", filepath.Dir(webLifecyclesPath), err)
		}
		if err := os.WriteFile(webLifecyclesPath, got, 0o644); err != nil {
			t.Fatalf("write %s: %v", webLifecyclesPath, err)
		}
		t.Logf("updated %s", webLifecyclesPath)
		return
	}

	want, err := os.ReadFile(webLifecyclesPath)
	if err != nil {
		t.Fatalf("read %s: %v\nRegenerate it with:\n  %s", webLifecyclesPath, err, webLifecyclesRegenerateCommand)
	}
	if string(got) != string(want) {
		t.Errorf("web/src/lib/generated/status-lifecycles.json does not match domain.Lifecycles().\n"+
			"If the lifecycle change is intentional, regenerate the file and update the web app:\n  %s",
			webLifecyclesRegenerateCommand)
	}
}
