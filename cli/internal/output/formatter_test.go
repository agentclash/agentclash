package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFormatterPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter("json", false, false)
	f.writer = &buf

	data := map[string]string{"id": "123", "name": "test"}
	if err := f.PrintJSON(data); err != nil {
		t.Fatalf("PrintJSON error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if result["id"] != "123" {
		t.Fatalf("id = %q, want %q", result["id"], "123")
	}
}

func TestFormatterPrintYAML(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter("yaml", false, false)
	f.writer = &buf

	data := map[string]string{"id": "456", "name": "yaml-test"}
	if err := f.PrintYAML(data); err != nil {
		t.Fatalf("PrintYAML error: %v", err)
	}

	var result map[string]string
	if err := yaml.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid YAML: %v", err)
	}
	if result["id"] != "456" {
		t.Fatalf("id = %q, want %q", result["id"], "456")
	}
}

func TestFormatterPrintTableRenderRows(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter("table", false, false)
	f.writer = &buf

	cols := []Column{{Header: "ID"}, {Header: "Name"}}
	rows := [][]string{
		{"abc", "Alice"},
		{"def", "Bob"},
	}

	f.PrintTable(cols, rows)

	output := buf.String()
	if !strings.Contains(output, "ID") {
		t.Fatalf("table should contain header 'ID', got:\n%s", output)
	}
	if !strings.Contains(output, "NAME") {
		t.Fatalf("table should contain header 'NAME', got:\n%s", output)
	}
	if !strings.Contains(output, "Alice") {
		t.Fatalf("table should contain 'Alice', got:\n%s", output)
	}
	if !strings.Contains(output, "Bob") {
		t.Fatalf("table should contain 'Bob', got:\n%s", output)
	}
}

func TestFormatterPrintTableEmptyShowsWarning(t *testing.T) {
	var buf bytes.Buffer
	var errBuf bytes.Buffer
	f := NewFormatter("table", false, false)
	f.writer = &buf
	f.errw = &errBuf

	f.PrintTable([]Column{{Header: "ID"}}, nil)

	if buf.Len() != 0 {
		t.Fatalf("stdout should be empty for empty results, got:\n%s", buf.String())
	}
	if !strings.Contains(errBuf.String(), "No results") {
		t.Fatalf("stderr should show warning, got:\n%s", errBuf.String())
	}
}

func TestFormatterJSONFlagOverridesFormat(t *testing.T) {
	f := NewFormatter("table", true, false)
	if !f.IsJSON() {
		t.Fatal("IsJSON() should return true when jsonFlag is set")
	}
	if f.Format() != "json" {
		t.Fatalf("Format() = %q, want %q", f.Format(), "json")
	}
}

func TestFormatterPrintDispatchesCorrectly(t *testing.T) {
	// JSON format
	var jsonBuf bytes.Buffer
	jf := NewFormatter("json", false, false)
	jf.writer = &jsonBuf
	jf.Print(map[string]string{"x": "1"}, nil, nil)
	if !strings.Contains(jsonBuf.String(), `"x"`) {
		t.Fatalf("Print with json format should output JSON, got:\n%s", jsonBuf.String())
	}

	// Table format
	var tableBuf bytes.Buffer
	tf := NewFormatter("table", false, false)
	tf.writer = &tableBuf
	cols := []Column{{Header: "X"}}
	rows := [][]string{{"1"}}
	tf.Print(nil, cols, rows)
	if !strings.Contains(tableBuf.String(), "X") {
		t.Fatalf("Print with table format should output table, got:\n%s", tableBuf.String())
	}
}

func TestFormatterPrintSuccess(t *testing.T) {
	var errBuf bytes.Buffer
	f := NewFormatter("table", false, false)
	f.errw = &errBuf

	f.PrintSuccess("done!")
	if !strings.Contains(errBuf.String(), "done!") {
		t.Fatalf("PrintSuccess output = %q, should contain 'done!'", errBuf.String())
	}
}

func TestFormatterQuietSuppressesMessages(t *testing.T) {
	var errBuf bytes.Buffer
	f := NewFormatter("table", false, true)
	f.errw = &errBuf

	f.PrintSuccess("should not appear")
	f.PrintWarning("should not appear")
	if errBuf.Len() != 0 {
		t.Fatalf("quiet mode should suppress messages, got:\n%s", errBuf.String())
	}
}

func TestFormatterPrintErrorAlwaysShows(t *testing.T) {
	var errBuf bytes.Buffer
	f := NewFormatter("table", false, true) // quiet mode
	f.errw = &errBuf

	f.PrintError("critical failure")
	if !strings.Contains(errBuf.String(), "critical failure") {
		t.Fatalf("PrintError should show even in quiet mode, got:\n%s", errBuf.String())
	}
}
