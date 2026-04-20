package auth

import (
	"runtime"
	"strings"
	"testing"
)

func TestOpenBrowserRejectsNonHTTPSchemes(t *testing.T) {
	saved := browserRun
	t.Cleanup(func() { browserRun = saved })
	// Detect any regression where a rejected URL still reaches the opener.
	browserRun = func(name string, args ...string) error {
		t.Fatalf("browserRun should not be invoked for rejected URLs, got %s %v", name, args)
		return nil
	}

	cases := []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"data:text/html,<script>",
		"ftp://example.com",
	}
	for _, raw := range cases {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			if err := OpenBrowser(raw); err == nil {
				t.Fatalf("expected error for %q", raw)
			}
		})
	}
}

func TestOpenBrowserAcceptsHTTPAndHTTPS(t *testing.T) {
	var gotName string
	var gotArgs []string
	saved := browserRun
	t.Cleanup(func() { browserRun = saved })
	browserRun = func(name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	}

	wantName := "xdg-open"
	switch runtime.GOOS {
	case "darwin":
		wantName = "open"
	case "windows":
		wantName = "rundll32"
	}

	for _, raw := range []string{"http://localhost:8080/verify", "https://agentclash.dev/device?user_code=A"} {
		gotName, gotArgs = "", nil
		if err := OpenBrowser(raw); err != nil {
			t.Fatalf("OpenBrowser(%q) unexpected error: %v", raw, err)
		}
		if gotName != wantName {
			t.Fatalf("OpenBrowser(%q) invoked %q, want %q", raw, gotName, wantName)
		}
		// The URL must be the last positional argument on every platform so
		// a shell/cmd.exe never gets a chance to reinterpret it.
		if len(gotArgs) == 0 || gotArgs[len(gotArgs)-1] != raw {
			t.Fatalf("OpenBrowser(%q): URL not final arg, got args=%v", raw, gotArgs)
		}
		if runtime.GOOS == "windows" {
			joined := strings.Join(gotArgs, " ")
			if !strings.Contains(joined, "FileProtocolHandler") {
				t.Fatalf("Windows opener args missing FileProtocolHandler: %q", joined)
			}
		}
	}
}
