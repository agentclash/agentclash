package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/agentclash/agentclash/cli/cmd"
)

// Set via ldflags at build time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	err := cmd.Execute()
	if err == nil {
		return
	}

	code, rendered := cmd.RenderError(err, os.Stderr)
	if rendered {
		os.Exit(code)
	}

	var exitErr *cmd.ExitCodeError
	if errors.As(err, &exitErr) {
		if !exitErr.Silent() {
			fmt.Fprintln(os.Stderr, exitErr)
		}
		os.Exit(exitErr.Code)
	}

	fmt.Fprintln(os.Stderr, err)
	// Same failure-class exit band as JSON mode (RenderError computes it even
	// when it does not render), so scripts can branch identically either way.
	os.Exit(code)
}
