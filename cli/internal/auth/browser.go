package auth

import (
	"os"
	"os/exec"
	"runtime"
)

// OpenBrowser opens a URL in the user's default browser.
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
