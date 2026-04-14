package auth

import (
	"os"
	"os/exec"
	"runtime"
)

// OpenBrowser opens the given URL in the user's default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// CanOpenBrowser returns true if we can likely open a browser.
// Returns false in SSH sessions, containers, or headless environments.
func CanOpenBrowser() bool {
	// SSH session — no local browser.
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != "" {
		return false
	}

	// Linux without display — headless.
	if runtime.GOOS == "linux" {
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return false
		}
	}

	// Docker/container indicators.
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return false
	}

	// Dumb terminal.
	if os.Getenv("TERM") == "dumb" {
		return false
	}

	return true
}
