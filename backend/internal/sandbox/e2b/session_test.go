package e2b

import "testing"

func TestMergeEnvironmentCarriesSandboxEnvIntoCommandEnv(t *testing.T) {
	merged := mergeEnvironment(
		map[string]string{
			"CODEX_API_KEY":  "sk-workspace",
			"OPENAI_API_KEY": "sk-workspace",
			"PATH":           "/usr/bin",
		},
		map[string]string{
			"PATH": "custom-path",
			"HOME": "/home/user",
		},
	)

	if merged["CODEX_API_KEY"] != "sk-workspace" || merged["OPENAI_API_KEY"] != "sk-workspace" {
		t.Fatalf("merged env = %#v, want Codex and OpenAI keys from sandbox env", merged)
	}
	if merged["PATH"] != "custom-path" {
		t.Fatalf("PATH = %q, want command override", merged["PATH"])
	}
	if merged["HOME"] != "/home/user" {
		t.Fatalf("HOME = %q, want command env", merged["HOME"])
	}
}

func TestMergeEnvironmentReturnsNilWhenEmpty(t *testing.T) {
	if got := mergeEnvironment(nil, nil); got != nil {
		t.Fatalf("empty merge = %#v, want nil", got)
	}
}
