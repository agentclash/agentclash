package api

import (
	"runtime"
	"strings"
	"testing"
)

func TestBuildUserAgent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		apiURL      string
		version     string
		commandPath string
		wantPrefix  string
		wantHasMeta bool
	}{
		{
			name:        "hosted with command emits full meta",
			apiURL:      "https://api.agentclash.dev",
			version:     "0.5.2",
			commandPath: "agentclash run create",
			wantPrefix:  "agentclash-cli/0.5.2 (cmd=run.create;",
			wantHasMeta: true,
		},
		{
			name:        "hosted root command emits neutral",
			apiURL:      "https://api.agentclash.dev",
			version:     "0.5.2",
			commandPath: "agentclash",
			wantPrefix:  "agentclash-cli/0.5.2",
			wantHasMeta: false,
		},
		{
			name:        "localhost emits neutral even with command",
			apiURL:      "http://localhost:8080",
			version:     "0.5.2",
			commandPath: "agentclash run create",
			wantPrefix:  "agentclash-cli/0.5.2",
			wantHasMeta: false,
		},
		{
			name:        "self-hosted custom host emits neutral",
			apiURL:      "https://acme-corp.internal/agentclash",
			version:     "0.5.2",
			commandPath: "agentclash workspace list",
			wantPrefix:  "agentclash-cli/0.5.2",
			wantHasMeta: false,
		},
		{
			name:        "case-insensitive host match",
			apiURL:      "HTTPS://API.AgentClash.dev/",
			version:     "0.5.2",
			commandPath: "agentclash auth login",
			wantPrefix:  "agentclash-cli/0.5.2 (cmd=auth.login;",
			wantHasMeta: true,
		},
		{
			name:        "malformed url falls back to neutral",
			apiURL:      "not a url at all",
			version:     "0.5.2",
			commandPath: "agentclash run create",
			wantPrefix:  "agentclash-cli/0.5.2",
			wantHasMeta: false,
		},
		{
			name:        "empty version becomes dev",
			apiURL:      "https://api.agentclash.dev",
			version:     "",
			commandPath: "agentclash run create",
			wantPrefix:  "agentclash-cli/dev (cmd=run.create;",
			wantHasMeta: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := BuildUserAgent(tc.apiURL, tc.version, tc.commandPath)
			if !strings.HasPrefix(got, tc.wantPrefix) {
				t.Fatalf("BuildUserAgent = %q; want prefix %q", got, tc.wantPrefix)
			}
			hasMeta := strings.Contains(got, "(cmd=")
			if hasMeta != tc.wantHasMeta {
				t.Fatalf("BuildUserAgent = %q; meta present? got=%v want=%v", got, hasMeta, tc.wantHasMeta)
			}
			if tc.wantHasMeta {
				if !strings.Contains(got, "os="+runtime.GOOS) {
					t.Fatalf("BuildUserAgent = %q; missing os=%s", got, runtime.GOOS)
				}
				if !strings.Contains(got, "arch="+runtime.GOARCH) {
					t.Fatalf("BuildUserAgent = %q; missing arch=%s", got, runtime.GOARCH)
				}
			}
		})
	}
}

func TestNormalizeCommandPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"agentclash run create", "run.create"},
		{"agentclash", ""},
		{"agentclash auth login", "auth.login"},
		{"", ""},
		{"agentclash challenge-pack publish", "challenge-pack.publish"},
	}
	for _, c := range cases {
		got := normalizeCommandPath(c.in)
		if got != c.want {
			t.Fatalf("normalizeCommandPath(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}
