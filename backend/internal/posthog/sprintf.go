package posthog

import "fmt"

// fmtSprintf wraps fmt.Sprintf so client.go can stay import-light at the top.
func fmtSprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
