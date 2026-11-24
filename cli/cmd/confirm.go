package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// confirmReader gives the source of a confirmation answer. Tests replace it.
var confirmReader = func() io.Reader { return os.Stdin }

// confirmLargeEffect asks for confirmation before a command with a large
// effect, for example a delete or a stop of all work in a workspace. The
// command must declare its own --yes flag.
//
//   - --yes set: continue with no prompt.
//   - Non-interactive mode (--non-interactive, AGENTCLASH_NONINTERACTIVE, CI
//     or no TTY): stop with interactive_input_required (exit 64). The command
//     makes no request.
//   - Interactive terminal: write "<prompt> [y/N] " to stderr and read one
//     line. Only "y" or "yes" on a full line continues; any other answer, an
//     empty line or EOF (also after "y") stops with aborted (exit 64).
//
// The prompt goes to stderr even with --quiet, and stdout stays clean for
// --json output.
func confirmLargeEffect(cmd *cobra.Command, rc *RunContext, prompt string) error {
	if yes, _ := cmd.Flags().GetBool("yes"); yes {
		return nil
	}
	if nonInteractiveMode() {
		return &cliError{
			Code:     "interactive_input_required",
			Message:  fmt.Sprintf("%s needs a confirmation; pass --yes in non-interactive mode", cmd.CommandPath()),
			Details:  map[string]any{"flag": "--yes"},
			NextStep: "Check the effect of the command, then run it again with --yes.",
		}
	}

	fmt.Fprintf(rc.Output.ErrWriter(), "%s [y/N] ", prompt)
	line, err := bufio.NewReader(confirmReader()).ReadString('\n')
	// Accept only a full line. EOF before the end of the line or a read
	// failure is not a clear answer, also if the text is "y".
	if err == nil {
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
			return nil
		}
	}
	return &cliError{
		Code:    "aborted",
		Message: fmt.Sprintf("%s cancelled: no confirmation", cmd.CommandPath()),
	}
}
