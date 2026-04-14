package output

import (
	"os"

	"github.com/fatih/color"
)

// Status color functions.
var (
	Green   = color.New(color.FgGreen).SprintFunc()
	Red     = color.New(color.FgRed).SprintFunc()
	Yellow  = color.New(color.FgYellow).SprintFunc()
	Cyan    = color.New(color.FgCyan).SprintFunc()
	Bold    = color.New(color.Bold).SprintFunc()
	Faint   = color.New(color.Faint).SprintFunc()
	Magenta = color.New(color.FgMagenta).SprintFunc()
)

// DisableColors turns off all color output.
func DisableColors() {
	color.NoColor = true
}

// InitColors configures color output based on flags and environment.
func InitColors(noColor bool) {
	if noColor || os.Getenv("NO_COLOR") != "" {
		DisableColors()
	}
}

// StatusColor returns a colorized status string.
func StatusColor(status string) string {
	switch status {
	case "completed", "active", "ready", "passed", "pass":
		return Green(status)
	case "failed", "error", "archived":
		return Red(status)
	case "running", "executing", "evaluating", "provisioning", "building":
		return Cyan(status)
	case "queued", "pending", "draft", "invited":
		return Yellow(status)
	case "scoring", "paused":
		return Magenta(status)
	case "cancelled", "suspended":
		return Faint(status)
	default:
		return status
	}
}
