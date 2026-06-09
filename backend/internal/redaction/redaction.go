// Package redaction provides shared, high-precision secret-shape scrubbing for text that
// flows back to an LLM or is persisted. It was extracted from internal/engine (the issue
// #186 stderr scrubbing) so the engine and future dataset / Vibe Eval guide-agent consumers
// can share one implementation rather than each rolling its own. Only the engine is wired in
// today; the others land with their own consumers. See docs/vibe-eval-backend-design.md
// §7 / §11.6.
//
// Scope today: header-shaped auth secrets (Authorization:, Cookie:, bearer/basic tokens,
// x-api-key:, api_key:). Bare provider-key token shapes (e.g. sk-..., AKIA..., e2b_...) and
// signed-URL scrubbing for the Vibe Eval chat transcript will be added here alongside their
// consumer in the guide-agent work, with their own tests — they are intentionally NOT added
// yet to avoid changing a security-sensitive path that currently has no such caller.
package redaction

import "regexp"

// Marker replaces a matched secret fragment.
const Marker = "[redacted]"

// HeaderSecretPatterns match auth/header-shaped secrets. They deliberately over-match
// (greedy until end-of-line): legitimate text containing these tokens is rare, and a
// false-positive scrub loses a little debuggability but never leaks a secret.
var HeaderSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)authorization\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)proxy-authorization\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)set-cookie\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)cookie\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)x-(?:api|auth|access|secret|session|csrf|xsrf)[-_](?:key|token|id)\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)api[-_]?key\s*:\s*[^\r\n]*`),
	regexp.MustCompile(`(?i)bearer\s+[^\s\r\n]+`),
	regexp.MustCompile(`(?i)basic\s+[A-Za-z0-9+/=]{8,}`),
}

// ScrubHeaderSecrets replaces any HeaderSecretPatterns match in s with Marker. It is
// behavior-identical to the engine's former scrubStderrSecrets.
func ScrubHeaderSecrets(s string) string {
	scrubbed := s
	for _, pattern := range HeaderSecretPatterns {
		scrubbed = pattern.ReplaceAllString(scrubbed, Marker)
	}
	return scrubbed
}
