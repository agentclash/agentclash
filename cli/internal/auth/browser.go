package auth

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
)

// browserRun executes a platform-specific opener command. Unit tests swap
// this out with a stub that records the name/args and returns nil, which
// keeps the test cross-platform — callers don't need a startable dummy
// command to exist on the host.
var browserRun = func(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

// OpenBrowser opens a URL in the user's default browser. The URL is parsed
// and its scheme validated before exec so that neither the shell nor cmd.exe
// can interpret stray metacharacters from a compromised backend response.
//
// On Windows, we use rundll32 + url.dll,FileProtocolHandler instead of
// `cmd /c start <url>`. `start` re-parses its argument through cmd.exe, so
// `&`, `|`, `^`, and `"` in a hostile URL could spawn arbitrary commands.
// rundll32 takes the URL as a direct argument and does no shell parsing.
func OpenBrowser(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	switch parsed.Scheme {
	case "http", "https":
		// ok
	default:
		return fmt.Errorf("refusing to open URL with scheme %q", parsed.Scheme)
	}

	switch runtime.GOOS {
	case "darwin":
		return browserRun("open", raw)
	case "windows":
		return browserRun("rundll32", "url.dll,FileProtocolHandler", raw)
	default:
		return browserRun("xdg-open", raw)
	}
}

// CanOpenBrowser returns false for environments that are very likely headless.
func CanOpenBrowser() bool {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
		return false
	}
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return false
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}
