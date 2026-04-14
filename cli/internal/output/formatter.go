package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Format constants.
const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
)

// Formatter handles all output rendering.
type Formatter struct {
	format string
	quiet  bool
	writer io.Writer
	errw   io.Writer
}

// NewFormatter creates a formatter with the given settings.
func NewFormatter(format string, jsonFlag bool, quiet bool) *Formatter {
	f := &Formatter{
		format: format,
		quiet:  quiet,
		writer: os.Stdout,
		errw:   os.Stderr,
	}
	if jsonFlag {
		f.format = FormatJSON
	}
	return f
}

// Format returns the current output format.
func (f *Formatter) Format() string {
	return f.format
}

// IsJSON returns true if output format is JSON.
func (f *Formatter) IsJSON() bool {
	return f.format == FormatJSON
}

// Writer returns the primary output writer.
func (f *Formatter) Writer() io.Writer {
	return f.writer
}

// PrintTable renders a table to stdout.
func (f *Formatter) PrintTable(columns []Column, rows [][]string) {
	if len(rows) == 0 {
		f.PrintWarning("No results found.")
		return
	}
	RenderTable(f.writer, columns, rows)
}

// PrintJSON renders data as indented JSON to stdout.
func (f *Formatter) PrintJSON(data any) error {
	enc := json.NewEncoder(f.writer)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// PrintYAML renders data as YAML to stdout.
func (f *Formatter) PrintYAML(data any) error {
	enc := yaml.NewEncoder(f.writer)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(data)
}

// Print dispatches to the appropriate format renderer.
func (f *Formatter) Print(data any, columns []Column, rows [][]string) {
	switch f.format {
	case FormatJSON:
		f.PrintJSON(data)
	case FormatYAML:
		f.PrintYAML(data)
	default:
		f.PrintTable(columns, rows)
	}
}

// PrintRaw writes raw structured data (for JSON/YAML) or falls back to table.
func (f *Formatter) PrintRaw(data any) {
	switch f.format {
	case FormatJSON:
		f.PrintJSON(data)
	case FormatYAML:
		f.PrintYAML(data)
	default:
		f.PrintJSON(data)
	}
}

// PrintSuccess prints a success message to stderr.
func (f *Formatter) PrintSuccess(msg string) {
	if !f.quiet {
		fmt.Fprintf(f.errw, "%s %s\n", Green("✓"), msg)
	}
}

// PrintWarning prints a warning message to stderr.
func (f *Formatter) PrintWarning(msg string) {
	if !f.quiet {
		fmt.Fprintf(f.errw, "%s %s\n", Yellow("!"), msg)
	}
}

// PrintError prints an error message to stderr.
func (f *Formatter) PrintError(msg string) {
	fmt.Fprintf(f.errw, "%s %s\n", Red("error:"), msg)
}

// PrintDetail prints a key-value detail line to stdout (for `get` commands in table mode).
func (f *Formatter) PrintDetail(key, value string) {
	fmt.Fprintf(f.writer, "%-20s %s\n", Bold(key+":"), value)
}
