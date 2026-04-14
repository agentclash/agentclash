package output

import (
	"fmt"
	"os"
	"time"

	"github.com/briandowns/spinner"
)

// Spinner wraps a terminal spinner for long operations.
type Spinner struct {
	s       *spinner.Spinner
	enabled bool
}

// NewSpinner creates a spinner. It is disabled if output is not a TTY or quiet mode is on.
func NewSpinner(msg string, quiet bool) *Spinner {
	enabled := !quiet && isTerminal(os.Stderr)
	sp := &Spinner{enabled: enabled}
	if enabled {
		sp.s = spinner.New(spinner.CharSets[14], 80*time.Millisecond, spinner.WithWriter(os.Stderr))
		sp.s.Suffix = " " + msg
		sp.s.Start()
	}
	return sp
}

// Update changes the spinner message.
func (sp *Spinner) Update(msg string) {
	if sp.enabled && sp.s != nil {
		sp.s.Suffix = " " + msg
	}
}

// Stop stops the spinner with a final message.
func (sp *Spinner) Stop() {
	if sp.enabled && sp.s != nil {
		sp.s.Stop()
	}
}

// StopWithSuccess stops the spinner and prints a success message.
func (sp *Spinner) StopWithSuccess(msg string) {
	sp.Stop()
	if sp.enabled {
		fmt.Fprintf(os.Stderr, "%s %s\n", Green("✓"), msg)
	}
}

// StopWithError stops the spinner and prints an error message.
func (sp *Spinner) StopWithError(msg string) {
	sp.Stop()
	if sp.enabled {
		fmt.Fprintf(os.Stderr, "%s %s\n", Red("✗"), msg)
	}
}

func isTerminal(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
