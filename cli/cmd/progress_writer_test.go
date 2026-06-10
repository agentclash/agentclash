package cmd

import (
	"bytes"
	"testing"

	"github.com/agentclash/agentclash/cli/internal/output"
)

// progressWriter is the routing decision behind the security commands' --json
// support: in structured mode human progress must go to stderr so stdout stays
// a clean machine-readable stream. Assert that split directly.
func TestProgressWriterRoutesByFormat(t *testing.T) {
	t.Run("structured routes to stderr, stdout stays clean", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		f := output.NewFormatter(output.FormatJSON, false, false)
		f.SetWriters(&out, &errBuf)

		w, structured := progressWriter(&RunContext{Output: f})
		if !structured {
			t.Fatal("structured should be true for JSON output")
		}
		if _, err := w.Write([]byte("progress")); err != nil {
			t.Fatalf("write: %v", err)
		}
		if errBuf.String() != "progress" {
			t.Fatalf("progress = %q on stderr, want \"progress\"", errBuf.String())
		}
		if out.Len() != 0 {
			t.Fatalf("stdout must stay clean in structured mode, got %q", out.String())
		}
	})

	t.Run("table routes to stdout", func(t *testing.T) {
		var out, errBuf bytes.Buffer
		f := output.NewFormatter(output.FormatTable, false, false)
		f.SetWriters(&out, &errBuf)

		w, structured := progressWriter(&RunContext{Output: f})
		if structured {
			t.Fatal("structured should be false for table output")
		}
		if _, err := w.Write([]byte("progress")); err != nil {
			t.Fatalf("write: %v", err)
		}
		if out.String() != "progress" {
			t.Fatalf("progress = %q, want it on stdout", out.String())
		}
	})

	t.Run("nil RunContext defaults to stdout without panicking", func(t *testing.T) {
		w, structured := progressWriter(nil)
		if structured {
			t.Fatal("nil rc must report not-structured")
		}
		if w == nil {
			t.Fatal("nil rc must still return a usable writer")
		}
	})
}
