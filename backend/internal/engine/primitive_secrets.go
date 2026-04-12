package engine

import (
	"regexp"
	"strings"
)

// secretSafePrimitives enumerates every primitive that is hardened to
// handle plaintext ${secrets.*} values inside the sandbox without
// exposing them to the evaluated agent. Adding a primitive to this set
// is a security-relevant change and should be reviewed carefully:
//
//   - the primitive must NOT write its resolved args to a sandbox
//     filesystem path the agent can read (use stdin pipes or private
//     filesystem roots not in ReadableRoots).
//   - the primitive must NOT place the resolved secret in argv — argv
//     is observable via /proc/[pid]/cmdline by any process in the
//     sandbox.
//   - the primitive must strip sensitive response/output fields
//     (Authorization / Cookie / X-API-Key headers, etc.) before
//     returning a ToolExecutionResult to the LLM.
//   - the primitive must never include a resolved secret in an error
//     message.
//
// v1 includes only http_request. INTENTIONALLY EXCLUDED:
//
//   - exec / query_sql / query_json: the command or query text lands
//     in argv (observable via /proc/[pid]/cmdline) with no stdin
//     alternative implemented today.
//   - submit: the argument is the agent's final answer; anything
//     substituted into it is echoed back through the LLM context and
//     persisted in run_events as the run output.
//   - build / run_tests: pass env_vars to a subprocess whose stderr
//     is returned verbatim to the LLM as the tool result; no
//     scrubbing.
//
// Expanding this list to add a new primitive requires completing
// each bullet above for that primitive AND the scrubbing / error-
// sanitization work #186 step 3-5 did for http_request. See the
// issue for the full threat model.
var secretSafePrimitives = map[string]struct{}{
	httpRequestToolName: {},
}

func primitiveAcceptsSecrets(primitiveName string) bool {
	_, ok := secretSafePrimitives[primitiveName]
	return ok
}

// templateReferencesSecrets walks any template value (map / slice /
// string) and reports whether at least one string element references
// a ${secrets.*} placeholder. Used at composed-tool build time to
// gate secret-bearing args to primitives that can handle them safely.
func templateReferencesSecrets(value any) bool {
	switch v := value.(type) {
	case string:
		return stringReferencesSecrets(v)
	case map[string]any:
		for _, inner := range v {
			if templateReferencesSecrets(inner) {
				return true
			}
		}
	case []any:
		for _, inner := range v {
			if templateReferencesSecrets(inner) {
				return true
			}
		}
	}
	return false
}

// argsTemplateHasOutputPath checks whether an http_request argsTemplate
// declares a non-empty output_path. Composed tools that carry ${secrets.*}
// must not also specify output_path because the response body (which could
// echo credentials from the request) would persist as a file the agent can
// read_file on, bypassing the response header scrubber.
func argsTemplateHasOutputPath(args map[string]any) bool {
	v, ok := args["output_path"]
	if !ok {
		return false
	}
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) != ""
}

func stringReferencesSecrets(s string) bool {
	remaining := s
	for {
		idx := strings.Index(remaining, "${")
		if idx == -1 {
			return false
		}
		after := remaining[idx+2:]
		closeIdx := strings.Index(after, "}")
		if closeIdx == -1 {
			return false
		}
		if strings.HasPrefix(after[:closeIdx], "secrets.") {
			return true
		}
		remaining = after[closeIdx+1:]
	}
}

// sensitiveResponseHeaders is the case-insensitive denylist of HTTP
// response header names that may carry authentication material echoed
// back by a remote API. When http_request returns its parsed response
// to the LLM, these headers are replaced with a redacted marker so a
// server that mirrors the request Authorization header (for debug or
// by accident) cannot leak a ${secrets.X}-substituted value back into
// the agent's context.
//
// The list is intentionally curated rather than heuristic ("strip any
// header containing 'auth'"): a fixed allowlist gives the security
// reviewer a single place to audit, and a heuristic would
// false-positive on legitimate headers like X-Auth-Request-Redirect.
// Expand with care — a missed entry is a leak, but a false add breaks
// legitimate response inspection. Entries must be lower-case; the
// lookup lower-cases at the call site (HTTP header names are
// case-insensitive per RFC 7230).
var sensitiveResponseHeaders = map[string]struct{}{
	// RFC 7235 challenge / credential headers.
	"authorization":       {},
	"proxy-authorization": {},
	"www-authenticate":    {},
	"proxy-authenticate":  {},
	// RFC 6265 cookies.
	"cookie":     {},
	"set-cookie": {},
	// Common vendor / custom auth headers.
	"x-api-key":            {},
	"x-apikey":             {},
	"api-key":              {},
	"apikey":               {},
	"x-auth-token":         {},
	"x-access-token":       {},
	"x-access-key":         {},
	"x-secret-key":         {},
	"x-session-token":      {},
	"x-session-id":         {},
	"x-csrf-token":         {},
	"x-xsrf-token":         {},
	// AWS SigV4 / STS.
	"x-amz-security-token": {},
	// Google Cloud user credential headers.
	"x-goog-api-key":         {},
	"x-goog-iam-authorization-token": {},
	// Bare token-style names some APIs use.
	"token": {},
}

const redactedHeaderMarker = "[redacted]"

// scrubSensitiveResponseHeaders walks a decoded http_request response
// payload and replaces any sensitive header value with a redacted
// marker. Safe to call on any shape — a nil, a non-map, or a response
// without headers is a no-op.
func scrubSensitiveResponseHeaders(payload any) {
	m, ok := payload.(map[string]any)
	if !ok {
		return
	}
	headers, ok := m["headers"].(map[string]any)
	if !ok {
		return
	}
	for key := range headers {
		if _, sensitive := sensitiveResponseHeaders[strings.ToLower(strings.TrimSpace(key))]; sensitive {
			headers[key] = redactedHeaderMarker
		}
	}
}

// stderrSecretPatterns is the set of regex patterns scrubbed from
// primitive stderr before it flows back to the LLM as a tool error
// message. This is defense-in-depth for two scenarios:
//
//  1. An older E2B template version is pinned to a pack that still
//     ships http_request.py WITHOUT the try/except wrapper #186 step 5
//     added. In that case Python would print a raw traceback (which
//     may contain request headers in the exception repr) directly to
//     stderr, and without Go-side scrubbing the raw trace would flow
//     back to the LLM.
//  2. A future refactor introduces another primitive whose error
//     path goes through executeInternalCommand's stderr return but
//     gets missed in a security review.
//
// The patterns deliberately over-match (greedy until end-of-line).
// Legitimate error messages with these tokens are rare, and a
// false-positive scrub loses a bit of debuggability but never leaks.
var stderrSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)authorization\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)proxy-authorization\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)cookie\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)set-cookie\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)x-(?:api|auth|access|secret|session|csrf|xsrf)[-_](?:key|token|id)\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)api[-_]?key\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)bearer\s+[^\s\r\n]+`),
	regexp.MustCompile(`(?i)basic\s+[A-Za-z0-9+/=]{8,}`),
}

// scrubStderrSecrets replaces any fragment of stderr that matches a
// well-known auth pattern with a fixed marker. Called on the stderr
// returned from primitive executions before it becomes a tool error
// message, so a raw python traceback (or any future primitive that
// misbehaves) cannot dump a resolved secret back to the LLM.
func scrubStderrSecrets(stderr string) string {
	scrubbed := stderr
	for _, pattern := range stderrSecretPatterns {
		scrubbed = pattern.ReplaceAllString(scrubbed, redactedHeaderMarker)
	}
	return scrubbed
}
