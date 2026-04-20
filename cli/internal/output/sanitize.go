package output

import "strings"

// SanitizeControl strips ANSI escape sequences and C0 control bytes from s,
// preserving only tab (\t) and newline (\n). Use it before printing
// server-controlled strings in non-JSON modes so a malicious payload can't
// move the cursor, clear the screen, or smuggle OSC hyperlinks into the
// user's terminal.
//
// JSON / YAML output remains byte-for-byte — automation pipelines rely on
// seeing raw bytes, and terminal interpretation doesn't apply there.
func SanitizeControl(s string) string {
	if s == "" {
		return s
	}
	if !needsSanitize(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			// Drop ESC (0x1b) and other C0 / DEL control bytes.
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func needsSanitize(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\t' || c == '\n' {
			continue
		}
		if c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

// SanitizeLine is like SanitizeControl but also strips tabs and newlines,
// replacing them with a single space. Use it for server-controlled strings
// that get embedded inline in a single-line terminal message — e.g.
// `compare gate`'s summary — so a buggy or hostile API can't inject extra
// lines or forge prefixes like `error:` on a line of its own.
func SanitizeLine(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	changed := false
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			b.WriteByte(' ')
			changed = true
		case r < 0x20 || r == 0x7f:
			changed = true
			continue
		default:
			b.WriteRune(r)
		}
	}
	if !changed {
		return s
	}
	return b.String()
}
