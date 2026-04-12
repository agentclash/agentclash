package workflow

import "testing"

func TestExecutionModeFromManifest(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		want     string
	}{
		{"empty", "", ""},
		{"native default", `{"version":{"number":1}}`, ""},
		{"prompt_eval", `{"version":{"execution_mode":"prompt_eval"}}`, "prompt_eval"},
		{"native explicit", `{"version":{"execution_mode":"native"}}`, "native"},
		{"invalid json", `not json`, ""},
		{"trimmed", `{"version":{"execution_mode":"  prompt_eval  "}}`, "prompt_eval"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := executionModeFromManifest([]byte(tc.manifest))
			if got != tc.want {
				t.Fatalf("executionModeFromManifest(%q) = %q, want %q", tc.manifest, got, tc.want)
			}
		})
	}
}
