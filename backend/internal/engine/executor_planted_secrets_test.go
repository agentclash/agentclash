package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/sandbox"
)

// recordingSession is a slightly richer fakeSandboxSession that
// captures every upload and exec for assertion. The verification
// tests use a simpler version; we keep this local so it doesn't get
// entangled with their assumptions.
type recordingSession struct {
	mu        sync.Mutex
	uploaded  map[string][]byte
	execCalls []sandbox.ExecRequest
	// execResultFor maps a substring of the joined Command to a
	// canned ExecResult; the first match wins. Anything unmatched
	// returns ExitCode 0.
	execResultFor map[string]sandbox.ExecResult
}

func newRecordingSession() *recordingSession {
	return &recordingSession{
		uploaded:      map[string][]byte{},
		execResultFor: map[string]sandbox.ExecResult{},
	}
}

func (s *recordingSession) ID() string { return "recording" }
func (s *recordingSession) UploadFile(ctx context.Context, path string, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.uploaded[path] = append([]byte(nil), content...)
	return nil
}
func (s *recordingSession) ReadFile(ctx context.Context, path string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.uploaded[path]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("not found: %s", path)
}
func (s *recordingSession) WriteFile(ctx context.Context, path string, content []byte) error {
	return s.UploadFile(ctx, path, content)
}
func (s *recordingSession) ListFiles(ctx context.Context, prefix string) ([]sandbox.FileInfo, error) {
	return nil, nil
}
func (s *recordingSession) Exec(ctx context.Context, request sandbox.ExecRequest) (sandbox.ExecResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.execCalls = append(s.execCalls, request)
	joined := strings.Join(request.Command, " ")
	for needle, result := range s.execResultFor {
		if strings.Contains(joined, needle) {
			return result, nil
		}
	}
	return sandbox.ExecResult{ExitCode: 0}, nil
}
func (s *recordingSession) DownloadFile(ctx context.Context, path string) ([]byte, error) {
	return s.ReadFile(ctx, path)
}
func (s *recordingSession) Destroy(ctx context.Context) error { return nil }

func plantedSecretsManifest(t *testing.T, secrets ...plantedSecret) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"security": map[string]any{
			"planted_secrets": secrets,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestApplyPlantedSecretsToRequest_EnvLocationAdds(t *testing.T) {
	req := &sandbox.CreateRequest{}
	manifest := plantedSecretsManifest(t,
		plantedSecret{Name: "STRIPE_KEY", Value: "canary-stripe", Location: "env"},
		plantedSecret{Name: "DB_PASSWORD", Value: "canary-db", Location: "env"},
	)
	applyPlantedSecretsToRequest(req, manifest)
	if got := req.EnvVars["STRIPE_KEY"]; got != "canary-stripe" {
		t.Errorf("STRIPE_KEY = %q; want canary-stripe", got)
	}
	if got := req.EnvVars["DB_PASSWORD"]; got != "canary-db" {
		t.Errorf("DB_PASSWORD = %q; want canary-db", got)
	}
}

func TestApplyPlantedSecretsToRequest_ManifestEnvWinsOnCollision(t *testing.T) {
	// Manifest sandbox.env_vars are populated by applySandboxConfig
	// BEFORE applyPlantedSecretsToRequest runs. Simulate that here.
	req := &sandbox.CreateRequest{EnvVars: map[string]string{"STRIPE_KEY": "manifest-override"}}
	manifest := plantedSecretsManifest(t,
		plantedSecret{Name: "STRIPE_KEY", Value: "canary-stripe", Location: "env"},
	)
	applyPlantedSecretsToRequest(req, manifest)
	if got := req.EnvVars["STRIPE_KEY"]; got != "manifest-override" {
		t.Errorf("manifest override must win; got %q", got)
	}
}

func TestApplyPlantedSecretsToRequest_InfisicalMockInjectsProxyHandles(t *testing.T) {
	req := &sandbox.CreateRequest{}
	manifest := plantedSecretsManifest(t,
		plantedSecret{Name: "AGENT_VAULT_TOKEN", Value: "av_agt_canary_xyz", Location: "infisical-mock"},
	)
	applyPlantedSecretsToRequest(req, manifest)
	wantProxy := fmt.Sprintf("http://%s:%d", mockAgentVaultHost, mockAgentVaultPort)
	for _, key := range []string{"HTTPS_PROXY", "HTTP_PROXY", "AGENT_VAULT_URL"} {
		if got := req.EnvVars[key]; got != wantProxy {
			t.Errorf("%s = %q; want %q", key, got, wantProxy)
		}
	}
	if got := req.EnvVars["AGENT_VAULT_TOKEN"]; got != "av_agt_canary_xyz" {
		t.Errorf("AGENT_VAULT_TOKEN = %q; want canary value", got)
	}
	if got := req.EnvVars["AGENT_VAULT_MOCK_LOG"]; got != mockAgentVaultLogPath {
		t.Errorf("AGENT_VAULT_MOCK_LOG = %q; want %q", got, mockAgentVaultLogPath)
	}
}

func TestApplyPlantedSecretsToRequest_InfisicalMockDoesNotOverrideExplicitProxy(t *testing.T) {
	req := &sandbox.CreateRequest{EnvVars: map[string]string{"HTTPS_PROXY": "http://operator-set:9999"}}
	manifest := plantedSecretsManifest(t,
		plantedSecret{Name: "AGENT_VAULT_TOKEN", Value: "av_agt_canary_xyz", Location: "infisical-mock"},
	)
	applyPlantedSecretsToRequest(req, manifest)
	if got := req.EnvVars["HTTPS_PROXY"]; got != "http://operator-set:9999" {
		t.Errorf("operator HTTPS_PROXY must win; got %q", got)
	}
}

func TestApplyPlantedSecretsToRequest_FileLocationIgnored(t *testing.T) {
	req := &sandbox.CreateRequest{}
	manifest := plantedSecretsManifest(t,
		plantedSecret{Name: "API_KEY", Value: "canary", Location: "file", FilePath: "/workspace/secret"},
	)
	applyPlantedSecretsToRequest(req, manifest)
	if got := req.EnvVars["API_KEY"]; got != "" {
		t.Errorf("file-located secret must not be planted as env var; got %q", got)
	}
}

func TestApplyPlantedSecretsToRequest_NoSecurityIsNoop(t *testing.T) {
	req := &sandbox.CreateRequest{}
	applyPlantedSecretsToRequest(req, json.RawMessage(`{"version":{"number":1}}`))
	if len(req.EnvVars) != 0 {
		t.Errorf("no-security manifest should leave EnvVars unset; got %v", req.EnvVars)
	}
}

func TestApplyPlantedSecretsToSession_FileLocationUploadsValue(t *testing.T) {
	sess := newRecordingSession()
	manifest := plantedSecretsManifest(t,
		plantedSecret{Name: "API_KEY", Value: "canary-file-value", Location: "file", FilePath: "/workspace/secret.txt"},
	)
	if err := applyPlantedSecretsToSession(context.Background(), sess, manifest); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := string(sess.uploaded["/workspace/secret.txt"]); got != "canary-file-value" {
		t.Errorf("uploaded /workspace/secret.txt = %q; want canary-file-value", got)
	}
	for _, call := range sess.execCalls {
		if strings.Contains(strings.Join(call.Command, " "), "mock_agent_vault.py") {
			t.Errorf("file-only manifest must not launch the mock sidecar; got call %v", call.Command)
		}
	}
}

func TestApplyPlantedSecretsToSession_FileLocationMissingFilePathErrors(t *testing.T) {
	sess := newRecordingSession()
	manifest := plantedSecretsManifest(t,
		plantedSecret{Name: "API_KEY", Value: "v", Location: "file"}, // no FilePath
	)
	if err := applyPlantedSecretsToSession(context.Background(), sess, manifest); err == nil {
		t.Fatal("expected error for file-located secret missing FilePath; got nil")
	}
}

func TestApplyPlantedSecretsToSession_InfisicalMockUploadsAndLaunches(t *testing.T) {
	sess := newRecordingSession()
	manifest := plantedSecretsManifest(t,
		plantedSecret{Name: "AGENT_VAULT_TOKEN", Value: "canary", Location: "infisical-mock"},
	)
	if err := applyPlantedSecretsToSession(context.Background(), sess, manifest); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if _, ok := sess.uploaded[mockAgentVaultScriptPath]; !ok {
		t.Errorf("mock script must be uploaded to %s; uploaded paths = %v", mockAgentVaultScriptPath, mapKeys(sess.uploaded))
	}
	body := sess.uploaded[mockAgentVaultScriptPath]
	if !strings.Contains(string(body), "mock-agent-vault") {
		t.Errorf("uploaded body does not look like the bundled script (no 'mock-agent-vault' marker)")
	}

	// Two execs: one to start the sidecar, one for the healthcheck.
	if got := len(sess.execCalls); got < 2 {
		t.Fatalf("expected at least 2 exec calls (launch + healthcheck); got %d: %v", got, sess.execCalls)
	}
	launch := strings.Join(sess.execCalls[0].Command, " ")
	if !strings.Contains(launch, "setsid python3") {
		t.Errorf("launch command should use `setsid python3`; got %q", launch)
	}
	if !strings.Contains(launch, mockAgentVaultScriptPath) {
		t.Errorf("launch command should reference %s; got %q", mockAgentVaultScriptPath, launch)
	}
	healthcheck := strings.Join(sess.execCalls[1].Command, " ")
	if !strings.Contains(healthcheck, "__healthz") {
		t.Errorf("second exec should be the healthcheck poll; got %q", healthcheck)
	}
}

func TestApplyPlantedSecretsToSession_HealthcheckFailureErrors(t *testing.T) {
	sess := newRecordingSession()
	// Force the healthcheck Exec to come back with exit=1.
	sess.execResultFor["__healthz"] = sandbox.ExecResult{ExitCode: 1, Stderr: "curl: (7) Connection refused"}
	manifest := plantedSecretsManifest(t,
		plantedSecret{Name: "AGENT_VAULT_TOKEN", Value: "canary", Location: "infisical-mock"},
	)
	err := applyPlantedSecretsToSession(context.Background(), sess, manifest)
	if err == nil {
		t.Fatal("expected error when healthcheck fails; got nil")
	}
	if !strings.Contains(err.Error(), "healthcheck") {
		t.Errorf("error should mention healthcheck; got %v", err)
	}
}

func TestApplyPlantedSecretsToSession_NoSecurityIsNoop(t *testing.T) {
	sess := newRecordingSession()
	if err := applyPlantedSecretsToSession(context.Background(), sess, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(sess.uploaded) != 0 || len(sess.execCalls) != 0 {
		t.Errorf("empty manifest must produce no side effects; uploads=%v execs=%v",
			mapKeys(sess.uploaded), sess.execCalls)
	}
}

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/safe/path", "/safe/path"},
		{"plain", "plain"},
		{"with space", "'with space'"},
		{"has'quote", `'has'\''quote'`},
		{"a$b", "'a$b'"},
	}
	for _, tc := range cases {
		if got := shellQuote(tc.in); got != tc.want {
			t.Errorf("shellQuote(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func mapKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
