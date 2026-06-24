package engine

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/sandbox"
)

// Wiring for `security.planted_secrets[]` declared on a challenge pack
// manifest. There are three supported `location` values:
//
//   env             — value exported as $Name in the sandbox shell
//   file            — value written to FilePath inside the sandbox
//   infisical-mock  — value served by an in-sandbox mock Agent Vault
//                     sidecar; the agent's http_request tool routes
//                     through it via HTTPS_PROXY
//
// The `tool-output` location is reserved in the schema for a future
// mock-tool surface but is not handled here yet.
//
// Two integration points:
//
//   applyPlantedSecretsToRequest  — pre-create, mutates the
//     sandbox.CreateRequest.EnvVars so that env-located canaries and
//     the mock-vault HTTPS_PROXY / AGENT_VAULT_TOKEN handles are
//     present in the session's default environment.
//
//   applyPlantedSecretsToSession  — post-create, runs against the
//     live session: uploads file-located canaries and launches the
//     mock Agent Vault sidecar in a background process.
//
// Both functions are no-ops when the manifest carries no security
// policy or no qualifying planted secrets.

const (
	plantedSecretLocationEnv           = "env"
	plantedSecretLocationFile          = "file"
	plantedSecretLocationInfisicalMock = "infisical-mock"

	mockAgentVaultScriptPath = "/workspace/agentclash/mock_agent_vault.py"
	mockAgentVaultLogPath    = "/workspace/agentclash/mock_agent_vault.log"
	mockAgentVaultHost       = "127.0.0.1"
	mockAgentVaultPort       = 8888

	mockAgentVaultLaunchTimeout = 15 * time.Second
)

// mockAgentVaultScript is the bundled Python sidecar — see
// assets/mock_agent_vault.py for the source. We embed at build time
// so the worker has no runtime dependency on the filesystem layout
// of the source tree once it ships.
//
//go:embed assets/mock_agent_vault.py
var mockAgentVaultScript []byte

// plantedSecret is the subset of challengepack.PlantedSecret we need
// at runtime. We decode from the manifest JSON directly rather than
// importing the bundle package so the executor stays decoupled from
// the publish-time validation surface (it would already have rejected
// invalid manifests).
type plantedSecret struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Location string `json:"location"`
	FilePath string `json:"file_path,omitempty"`
}

func decodePlantedSecrets(manifest json.RawMessage) []plantedSecret {
	if len(manifest) == 0 {
		return nil
	}
	var shape struct {
		Security *struct {
			PlantedSecrets []plantedSecret `json:"planted_secrets"`
		} `json:"security"`
	}
	if err := json.Unmarshal(manifest, &shape); err != nil {
		slog.Warn("decodePlantedSecrets: manifest unmarshal", "error", err)
		return nil
	}
	if shape.Security == nil {
		return nil
	}
	return shape.Security.PlantedSecrets
}

// applyPlantedSecretsToRequest pushes env-located and infisical-mock-
// located canaries into request.EnvVars. Manifest-declared
// sandbox.env_vars win on collision — pack authors who want a
// non-default name for the proxy URL can override it explicitly.
func applyPlantedSecretsToRequest(request *sandbox.CreateRequest, manifest json.RawMessage) {
	secrets := decodePlantedSecrets(manifest)
	if len(secrets) == 0 {
		return
	}
	if request.EnvVars == nil {
		request.EnvVars = map[string]string{}
	}

	addIfAbsent := func(key, value string) {
		if _, exists := request.EnvVars[key]; exists {
			return
		}
		request.EnvVars[key] = value
	}

	mockNeeded := false
	for _, secret := range secrets {
		switch strings.ToLower(strings.TrimSpace(secret.Location)) {
		case plantedSecretLocationEnv:
			name := strings.TrimSpace(secret.Name)
			if name == "" {
				continue
			}
			addIfAbsent(name, secret.Value)
		case plantedSecretLocationInfisicalMock:
			mockNeeded = true
			name := strings.TrimSpace(secret.Name)
			if name != "" {
				addIfAbsent(name, secret.Value)
			}
		}
	}
	if mockNeeded {
		proxyURL := fmt.Sprintf("http://%s:%d", mockAgentVaultHost, mockAgentVaultPort)
		addIfAbsent("HTTPS_PROXY", proxyURL)
		addIfAbsent("HTTP_PROXY", proxyURL)
		addIfAbsent("AGENT_VAULT_URL", proxyURL)
		addIfAbsent("AGENT_VAULT_MOCK_LOG", mockAgentVaultLogPath)
	}
}

// applyPlantedSecretsToSession runs after the sandbox session is up.
// It writes file-located canaries and, if any planted secret declares
// the infisical-mock location, uploads the bundled mock sidecar and
// launches it as a background process.
func applyPlantedSecretsToSession(ctx context.Context, session sandbox.Session, manifest json.RawMessage) error {
	secrets := decodePlantedSecrets(manifest)
	if len(secrets) == 0 {
		return nil
	}

	mockNeeded := false
	for _, secret := range secrets {
		switch strings.ToLower(strings.TrimSpace(secret.Location)) {
		case plantedSecretLocationFile:
			target := strings.TrimSpace(secret.FilePath)
			if target == "" {
				return NewFailure(StopReasonSandboxError,
					"planted secret with location=file is missing file_path",
					fmt.Errorf("planted secret %q missing file_path", secret.Name))
			}
			if err := session.UploadFile(ctx, target, []byte(secret.Value)); err != nil {
				return NewFailure(StopReasonSandboxError,
					"upload file-located planted secret",
					fmt.Errorf("planted secret %q to %s: %w", secret.Name, target, err))
			}
		case plantedSecretLocationInfisicalMock:
			mockNeeded = true
		}
	}

	if !mockNeeded {
		return nil
	}
	if err := launchMockAgentVault(ctx, session); err != nil {
		return err
	}
	return nil
}

// launchMockAgentVault uploads the Python mock sidecar and starts it
// as a detached background process. The launch shell uses `setsid`
// + `&` + redirected stdio so the Exec returns immediately while the
// daemon keeps running. A short healthcheck polls /__healthz so we
// fail fast if the bind didn't work.
func launchMockAgentVault(ctx context.Context, session sandbox.Session) error {
	if err := session.UploadFile(ctx, mockAgentVaultScriptPath, mockAgentVaultScript); err != nil {
		return NewFailure(StopReasonSandboxError,
			"upload mock agent vault script",
			fmt.Errorf("upload %s: %w", mockAgentVaultScriptPath, err))
	}
	// Make sure the log directory exists; the script's log-dir
	// best-effort mkdir is inside the script, but if writing to
	// /workspace/agentclash fails the script silently logs nothing.
	if err := session.UploadFile(ctx, filepath.Join(filepath.Dir(mockAgentVaultLogPath), ".keep"), []byte{}); err != nil {
		// Non-fatal — the script will try mkdir on its own.
		slog.Debug("launchMockAgentVault: parent .keep upload failed (non-fatal)", "error", err)
	}

	startCmd := fmt.Sprintf(
		"setsid python3 %s --host %s --port %d --log %s "+
			">/tmp/mock_agent_vault.boot.log 2>&1 </dev/null &",
		shellQuote(mockAgentVaultScriptPath),
		mockAgentVaultHost,
		mockAgentVaultPort,
		shellQuote(mockAgentVaultLogPath),
	)
	execCtx, cancel := context.WithTimeout(ctx, mockAgentVaultLaunchTimeout)
	defer cancel()
	if _, err := session.Exec(execCtx, sandbox.ExecRequest{
		Command: []string{"sh", "-c", startCmd},
		Timeout: 5 * time.Second,
	}); err != nil {
		return NewFailure(StopReasonSandboxError,
			"start mock agent vault sidecar",
			fmt.Errorf("setsid python3: %w", err))
	}

	// Healthcheck. The mock binds on localhost; if curl can reach
	// /__healthz we know the proxy is live before the agent's first
	// tool call. A small retry loop covers startup latency.
	healthCmd := fmt.Sprintf(
		`for i in 1 2 3 4 5 6 7 8 9 10; do `+
			`curl -fsS http://%s:%d/__healthz >/dev/null 2>&1 && exit 0; sleep 0.2; `+
			`done; exit 1`,
		mockAgentVaultHost, mockAgentVaultPort,
	)
	if result, err := session.Exec(execCtx, sandbox.ExecRequest{
		Command: []string{"sh", "-c", healthCmd},
		Timeout: 5 * time.Second,
	}); err != nil || result.ExitCode != 0 {
		return NewFailure(StopReasonSandboxError,
			"mock agent vault healthcheck did not respond",
			fmt.Errorf("healthcheck exit=%d err=%v boot-log at /tmp/mock_agent_vault.boot.log inside sandbox", result.ExitCode, err))
	}
	return nil
}

// shellQuote is a minimal single-quote shell escaper for paths we
// control. Paths come from constants here, not from the manifest, so
// the surface for shell injection is closed — this is belt-and-braces.
func shellQuote(s string) string {
	if !strings.ContainsAny(s, " \t\n\"'`$&|;<>(){}[]*?#~!") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
