package api

import (
	"fmt"
	"net/url"
	"runtime"
	"strings"
)

// HostedAPIHost is the only host the CLI is allowed to send command-level
// telemetry to. For every other host (localhost, self-hosted production
// backends, custom domains) the CLI emits the neutral User-Agent form so
// nothing about the command being run leaks into someone else's network.
const HostedAPIHost = "api.agentclash.dev"

// BuildUserAgent composes the User-Agent header value the CLI sends.
//
//	  Hosted (api.agentclash.dev):
//	    agentclash-cli/<version> (cmd=<dotted-cmd>; os=<goos>; arch=<goarch>; go=<goversion>)
//	  Self-hosted / localhost / unknown:
//	    agentclash-cli/<version>
//
// Gating is by URL host only — the resolved base URL flows from the same
// resolution order documented in the project README (--api-url >
// AGENTCLASH_API_URL > saved config > release default). If the URL fails to
// parse, the neutral form is returned (safe default — never leak command
// metadata when we're uncertain about the destination).
func BuildUserAgent(apiURL, version, commandPath string) string {
	neutral := neutralUserAgent(version)

	parsed, err := url.Parse(apiURL)
	if err != nil || parsed == nil || parsed.Host == "" {
		return neutral
	}
	host := strings.ToLower(parsed.Hostname())
	if host != HostedAPIHost {
		return neutral
	}

	command := normalizeCommandPath(commandPath)
	if command == "" {
		return neutral
	}
	return fmt.Sprintf(
		"agentclash-cli/%s (cmd=%s; os=%s; arch=%s; go=%s)",
		safeVersion(version), command, runtime.GOOS, runtime.GOARCH, runtime.Version(),
	)
}

func neutralUserAgent(version string) string {
	return "agentclash-cli/" + safeVersion(version)
}

func safeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "dev"
	}
	return v
}

// normalizeCommandPath converts cobra's "agentclash run create" form into the
// dotted form used as the PostHog "command" property ("run.create").
// Returns "" when the path is the root command alone — there's no specific
// command to record.
func normalizeCommandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	parts := strings.Fields(path)
	if len(parts) == 0 {
		return ""
	}
	// Drop the leading "agentclash" binary name.
	if parts[0] == "agentclash" {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ".")
}
