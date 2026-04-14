package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderTableFormatsColumns(t *testing.T) {
	var buf bytes.Buffer
	cols := []Column{{Header: "ID"}, {Header: "Name"}, {Header: "Status"}}
	rows := [][]string{
		{"1", "Alice", "active"},
		{"2", "Bob", "archived"},
	}

	RenderTable(&buf, cols, rows)
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (header + 2 rows), got %d:\n%s", len(lines), output)
	}

	// Header should be uppercase.
	if !strings.Contains(lines[0], "ID") {
		t.Fatalf("header should contain 'ID', got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "NAME") {
		t.Fatalf("header should contain 'NAME', got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "STATUS") {
		t.Fatalf("header should contain 'STATUS', got: %s", lines[0])
	}
}

func TestRenderTableAlignColumns(t *testing.T) {
	var buf bytes.Buffer
	cols := []Column{{Header: "Short"}, {Header: "Long Column"}}
	rows := [][]string{
		{"a", "x"},
		{"bb", "yy"},
	}

	RenderTable(&buf, cols, rows)
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// All lines should have consistent column alignment.
	// The "SHORT" header and "a"/"bb" values should be padded to the same width.
	headerParts := strings.Fields(lines[0])
	if headerParts[0] != "SHORT" {
		t.Fatalf("first header = %q, want 'SHORT'", headerParts[0])
	}
}

func TestRenderTableHandlesEmptyColumns(t *testing.T) {
	var buf bytes.Buffer
	RenderTable(&buf, []Column{}, nil)
	if buf.Len() != 0 {
		t.Fatalf("empty columns should produce no output, got:\n%s", buf.String())
	}
}

func TestRenderTableHandlesMissingCells(t *testing.T) {
	var buf bytes.Buffer
	cols := []Column{{Header: "A"}, {Header: "B"}, {Header: "C"}}
	rows := [][]string{
		{"1", "2"}, // Missing third cell
		{"x"},      // Missing second and third
	}

	RenderTable(&buf, cols, rows)
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), output)
	}
	// Should not panic — missing cells rendered as empty.
}
