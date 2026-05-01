package repository

import "testing"

func TestEscapePostgresLikePattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "agent-app",
			want:  "agent-app",
		},
		{
			name:  "escapes wildcard characters",
			input: "agent_app%",
			want:  `agent\_app\%`,
		},
		{
			name:  "escapes escape character first",
			input: `owner\repo_100%`,
			want:  `owner\\repo\_100\%`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapePostgresLikePattern(tc.input); got != tc.want {
				t.Fatalf("escapePostgresLikePattern(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
