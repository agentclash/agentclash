package cmd

import "fmt"

// ExitCodeError is returned from RunE when a command needs to set a specific
// process exit code (e.g. `compare gate` for CI integrations). Returning it
// from RunE avoids calling os.Exit directly, which would skip deferred cleanup.
type ExitCodeError struct {
	Code    int
	Message string
}

func (e *ExitCodeError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("exit code %d", e.Code)
	}
	return e.Message
}

// Silent reports whether main should suppress printing the error message
// before exiting. compareGate already prints a human- or JSON-readable summary
// to stdout/stderr, so it sets this to true.
func (e *ExitCodeError) Silent() bool {
	return e.Message == ""
}
