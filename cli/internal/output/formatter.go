package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/itchyny/gojq"
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
	query  *gojq.Code
}

// SetQuery installs a compiled jq expression that every structured success
// document is filtered through (gh --jq semantics). Error envelopes are
// rendered outside the Formatter and stay unfiltered by design — agents need
// a stable error shape regardless of the projection they asked for.
func (f *Formatter) SetQuery(code *gojq.Code) {
	f.query = code
}

// printQueried runs the compiled jq expression over data and emits results
// jq-style: strings print raw (no quotes), everything else as one compact
// JSON document per line. data is round-tripped through encoding/json first
// because gojq operates on plain JSON values, not Go structs.
func (f *Formatter) printQueried(data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	// Decode with UseNumber so integers larger than 2^53 survive: the default
	// any-decode turns every JSON number into a float64, silently corrupting
	// big int64 fields (e.g. event sequence numbers). gojq accepts json.Number
	// as a first-class input value, so the projection is exact end-to-end.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var plain any
	if err := dec.Decode(&plain); err != nil {
		return err
	}
	iter := f.query.Run(plain)
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if iterErr, isErr := v.(error); isErr {
			return fmt.Errorf("--query: %w", iterErr)
		}
		if s, isString := v.(string); isString {
			if _, err := fmt.Fprintln(f.writer, s); err != nil {
				return err
			}
			continue
		}
		line, err := json.Marshal(v)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(f.writer, string(line)); err != nil {
			return err
		}
	}
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

// IsStructured returns true if the output format is machine-parsable
// (JSON or YAML). Commands that previously gated on IsJSON should use this
// so --output yaml works end-to-end.
func (f *Formatter) IsStructured() bool {
	return f.format == FormatJSON || f.format == FormatYAML
}

// IsYAML returns true if the active format is YAML. Streaming commands that
// need per-event encoding (e.g. `run events`) use this to emit a proper
// YAML document stream rather than NDJSON.
func (f *Formatter) IsYAML() bool {
	return f.format == FormatYAML
}

// Writer returns the primary output writer.
func (f *Formatter) Writer() io.Writer {
	return f.writer
}

// ErrWriter returns the secondary (stderr) writer. Commands route human
// progress/diagnostics here when structured output (--json/--output) is active
// so stdout stays a clean machine-readable stream.
func (f *Formatter) ErrWriter() io.Writer {
	return f.errw
}

// SetWriters overrides output streams. It is primarily useful for focused
// command rendering tests that should not mutate process-wide stdout/stderr.
func (f *Formatter) SetWriters(writer, errw io.Writer) {
	if writer != nil {
		f.writer = writer
	}
	if errw != nil {
		f.errw = errw
	}
}

// PrintTable renders a table to stdout.
func (f *Formatter) PrintTable(columns []Column, rows [][]string) {
	if len(rows) == 0 {
		f.PrintWarning("No results found.")
		return
	}
	RenderTable(f.writer, columns, rows)
}

// PrintJSON renders data as indented JSON to stdout, or jq-filtered output
// when a --query expression is installed.
func (f *Formatter) PrintJSON(data any) error {
	if f.query != nil {
		return f.printQueried(data)
	}
	enc := json.NewEncoder(f.writer)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// PrintYAML renders data as YAML to stdout. A --query expression takes
// precedence and emits jq-style output (raw strings / compact JSON lines) —
// the projection defines the shape, not the container format.
func (f *Formatter) PrintYAML(data any) error {
	if f.query != nil {
		return f.printQueried(data)
	}
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

// PrintRaw writes raw structured data (for JSON/YAML) or falls back to JSON
// when no structured format is selected (table mode prefers JSON over a YAML
// dump of a map). Returns any encoder error.
func (f *Formatter) PrintRaw(data any) error {
	switch f.format {
	case FormatYAML:
		return f.PrintYAML(data)
	case FormatJSON:
		return f.PrintJSON(data)
	default:
		return f.PrintJSON(data)
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
