package engine

import "strings"

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
// See issue #186 for the full threat model.
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
