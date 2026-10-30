package vibe

import (
	"strings"
	"testing"
)

func TestSessionTitle(t *testing.T) {
	for _, tt := range []struct {
		name, input, want string
		invalid           bool
	}{
		{name: "trim", input: "  Returns agent  ", want: "Returns agent"},
		{name: "empty"},
		{name: "spaces", input: "   "},
		{name: "Unicode spaces", input: "\u2003Agent\u2003", want: "Agent"},
		{name: "80 Unicode characters", input: strings.Repeat("界", 80), want: strings.Repeat("界", 80)},
		{name: "81 characters", input: strings.Repeat("a", 81), invalid: true},
		{name: "81 Unicode characters", input: strings.Repeat("界", 81), invalid: true},
		{name: "newline", input: "bad\nname", invalid: true},
		{name: "leading tab", input: "\tname", invalid: true},
		{name: "trailing newline", input: "name\n", invalid: true},
		{name: "NUL", input: "bad\x00name", invalid: true},
		{name: "DEL", input: "bad\u007fname", invalid: true},
		{name: "C1 control", input: "bad\u0085name", invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SessionTitle(tt.input)
			if tt.invalid {
				requireFault(t, err, "invalid_request")
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("title=%q err=%v, want %q", got, err, tt.want)
			}
		})
	}
}
