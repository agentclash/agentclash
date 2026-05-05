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

	if code, rendered := cmd.RenderError(err, os.Stderr); rendered {
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
	os.Exit(1)
}
