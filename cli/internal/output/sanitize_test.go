package output

import "testing"

func TestSanitizeLine(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain", "plain"},
		{"", ""},
		{"line1\nline2", "line1 line2"},
		{"col1\tcol2", "col1 col2"},
		{"before\rafter", "before after"},
		{"\x1b[31mred\x1b[0m", "[31mred[0m"},
		{"a\x07b", "ab"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			if got := SanitizeLine(tc.in); got != tc.want {
				t.Fatalf("SanitizeLine(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizeControl(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"plain", "plain"},
		{"", ""},
		{"line1\nline2", "line1\nline2"},
		{"col1\tcol2", "col1\tcol2"},
		{"\x1b[31mred\x1b[0m", "[31mred[0m"},
		{"hello\x07world", "helloworld"},
		{"\x7fbackspace\x08", "backspace"},
		{"utf-8 💡 ok", "utf-8 💡 ok"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			if got := SanitizeControl(tc.in); got != tc.want {
				t.Fatalf("SanitizeControl(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
