package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

const (
	defaultSandboxWorkingDirectory = "/workspace"
	defaultSandboxTTL              = 60 * time.Minute
	sandboxBootBuffer              = 20 * time.Second
	sandboxCleanupTimeout          = 15 * time.Second
)

func (e NativeExecutor) prepareSandbox(ctx context.Context, executionContext repository.RunAgentExecutionContext, request sandbox.CreateRequest) (sandbox.Session, error) {
	session, err := e.sandboxProvider.Create(ctx, request)
	if err != nil {
		return nil, NewFailure(StopReasonSandboxError, "create native sandbox", err)
	}

	if err := stageSandboxInputs(ctx, session, executionContext); err != nil {
		return nil, cleanupSandboxOnError(session, err)
	}
	return session, nil
}

func cleanupSandboxOnError(session sandbox.Session, originalErr error) error {
	if session == nil {
		return originalErr
	}
	if destroyErr := destroySandbox(session); destroyErr != nil {
		return errors.Join(originalErr, NewFailure(StopReasonSandboxError, "destroy native sandbox", destroyErr))
	}
	return originalErr
}

func nativeSandboxRequest(executionContext repository.RunAgentExecutionContext) (sandbox.CreateRequest, error) {
	policy := sandbox.ToolPolicy{
		AllowedToolKinds: allowedToolKinds(executionContext.ChallengePackVersion.Manifest),
		AllowShell:       false,
		AllowNetwork:     false,
		MaxToolCalls:     executionContext.Deployment.RuntimeProfile.MaxToolCalls,
	}
	filesystem := sandbox.FilesystemSpec{
		WorkingDirectory:  defaultSandboxWorkingDirectory,
		ReadableRoots:     []string{defaultSandboxWorkingDirectory},
		WritableRoots:     []string{defaultSandboxWorkingDirectory},
		MaxWorkspaceBytes: 0,
	}

	applyChallengeSandboxPolicy(&policy, &filesystem, executionContext.ChallengePackVersion.Manifest)
	applyRuntimeSandboxPolicy(&policy, &filesystem, executionContext.Deployment.RuntimeProfile.ProfileConfig)

	request := sandbox.CreateRequest{
		RunID:      executionContext.Run.ID,
		RunAgentID: executionContext.RunAgent.ID,
		Timeout:    sandboxTTL(executionContext),
		ToolPolicy: policy,
		Filesystem: filesystem,
		Labels:     sandboxLabels(executionContext),
	}

	if err := applySandboxConfig(&request, executionContext.ChallengePackVersion.Manifest); err != nil {
		return sandbox.CreateRequest{}, err
	}

	return request, nil
}

func (e NativeExecutor) loadWorkspaceSecrets(ctx context.Context, workspaceID uuid.UUID) (map[string]string, error) {
	if e.secretsLookup == nil {
		return map[string]string{}, nil
	}
	loaded, err := e.secretsLookup.LoadWorkspaceSecrets(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if loaded == nil {
		return map[string]string{}, nil
	}
	return loaded, nil
}

func sandboxTTL(executionContext repository.RunAgentExecutionContext) time.Duration {
	timeout := runTimeout(executionContext)
	if timeout <= 0 {
		return defaultSandboxTTL
	}
	return timeout + sandboxBootBuffer + sandboxCleanupTimeout
}

func sandboxLabels(executionContext repository.RunAgentExecutionContext) map[string]string {
	return map[string]string{
		"run_id":                    executionContext.Run.ID.String(),
		"run_agent_id":              executionContext.RunAgent.ID.String(),
		"challenge_pack_version_id": executionContext.ChallengePackVersion.ID.String(),
		"agent_build_version_id":    executionContext.Deployment.AgentBuildVersion.ID.String(),
	}
}

func destroySandbox(session sandbox.Session) error {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), sandboxCleanupTimeout)
	defer cancel()
	return session.Destroy(cleanupCtx)
}

func allowedToolKinds(manifest json.RawMessage) []string {
	type toolPolicy struct {
		AllowedToolKinds []string `json:"allowed_tool_kinds"`
	}
	type challengeManifest struct {
		ToolPolicy *toolPolicy `json:"tool_policy"`
	}

	var decoded challengeManifest
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		slog.Warn("allowedToolKinds: failed to parse challenge manifest", "error", err)
		return nil
	}
	if decoded.ToolPolicy == nil {
		return nil
	}
	return normalizeStrings(decoded.ToolPolicy.AllowedToolKinds)
}

func applyChallengeSandboxPolicy(policy *sandbox.ToolPolicy, filesystem *sandbox.FilesystemSpec, manifest json.RawMessage) {
	type toolPolicy struct {
		AllowShell   *bool `json:"allow_shell"`
		AllowNetwork *bool `json:"allow_network"`
	}
	type filesystemPolicy struct {
		WorkingDirectory  string   `json:"working_directory"`
		ReadableRoots     []string `json:"readable_roots"`
		WritableRoots     []string `json:"writable_roots"`
		MaxWorkspaceBytes int64    `json:"max_workspace_bytes"`
	}
	type challengeManifest struct {
		ToolPolicy *toolPolicy       `json:"tool_policy"`
		Filesystem *filesystemPolicy `json:"filesystem"`
	}

	var decoded challengeManifest
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		slog.Warn("applyChallengeSandboxPolicy: failed to parse challenge manifest", "error", err)
		return
	}

	if decoded.ToolPolicy != nil {
		if decoded.ToolPolicy.AllowShell != nil {
			policy.AllowShell = *decoded.ToolPolicy.AllowShell
		}
		if decoded.ToolPolicy.AllowNetwork != nil {
			policy.AllowNetwork = *decoded.ToolPolicy.AllowNetwork
		}
	}

	if decoded.Filesystem != nil {
		mergeFilesystem(filesystem, decoded.Filesystem.WorkingDirectory, decoded.Filesystem.ReadableRoots, decoded.Filesystem.WritableRoots, decoded.Filesystem.MaxWorkspaceBytes)
	}
}

func applyRuntimeSandboxPolicy(policy *sandbox.ToolPolicy, filesystem *sandbox.FilesystemSpec, profileConfig json.RawMessage) {
	type sandboxConfig struct {
		WorkingDirectory  string   `json:"working_directory"`
		ReadableRoots     []string `json:"readable_roots"`
		WritableRoots     []string `json:"writable_roots"`
		MaxWorkspaceBytes int64    `json:"max_workspace_bytes"`
		AllowShell        *bool    `json:"allow_shell"`
		AllowNetwork      *bool    `json:"allow_network"`
	}
	type runtimeProfileConfig struct {
		Sandbox *sandboxConfig `json:"sandbox"`
	}

	var decoded runtimeProfileConfig
	if err := json.Unmarshal(profileConfig, &decoded); err != nil {
		slog.Warn("applyRuntimeSandboxPolicy: failed to parse runtime profile config", "error", err)
		return
	}
	if decoded.Sandbox == nil {
		return
	}

	if decoded.Sandbox.AllowShell != nil {
		policy.AllowShell = *decoded.Sandbox.AllowShell
	}
	if decoded.Sandbox.AllowNetwork != nil {
		policy.AllowNetwork = *decoded.Sandbox.AllowNetwork
	}

	mergeFilesystem(
		filesystem,
		decoded.Sandbox.WorkingDirectory,
		decoded.Sandbox.ReadableRoots,
		decoded.Sandbox.WritableRoots,
		decoded.Sandbox.MaxWorkspaceBytes,
	)
}

func applySandboxConfig(request *sandbox.CreateRequest, manifest json.RawMessage) error {
	type sandboxBlock struct {
		NetworkAccess      bool              `json:"network_access"`
		NetworkAllowlist   []string          `json:"network_allowlist"`
		EnvVars            map[string]string `json:"env_vars"`
		AdditionalPackages []string          `json:"additional_packages"`
		SandboxTemplateID  string            `json:"sandbox_template_id"`
	}
	type versionBlock struct {
		SandboxTemplateID string `json:"sandbox_template_id"`
	}
	type manifestShape struct {
		Sandbox *sandboxBlock `json:"sandbox"`
		Version *versionBlock `json:"version"`
	}

	var decoded manifestShape
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		// Preserve historical behavior: a malformed manifest is a no-op
		// here, not a hard error. Validation catches broken manifests at
		// publish time. Log the failure so operators can see it.
		slog.Warn("applySandboxConfig: failed to parse manifest", "error", err)
		return nil
	}

	if decoded.Sandbox != nil {
		if decoded.Sandbox.NetworkAccess {
			request.ToolPolicy.AllowNetwork = true
		}
		if len(decoded.Sandbox.NetworkAllowlist) > 0 {
			request.NetworkAllowlist = decoded.Sandbox.NetworkAllowlist
		}
		if len(decoded.Sandbox.EnvVars) > 0 {
			if err := validateEnvVarLiterals(decoded.Sandbox.EnvVars); err != nil {
				return err
			}
			request.EnvVars = decoded.Sandbox.EnvVars
		}
		if len(decoded.Sandbox.AdditionalPackages) > 0 {
			request.AdditionalPackages = decoded.Sandbox.AdditionalPackages
		}
		if decoded.Sandbox.SandboxTemplateID != "" {
			request.TemplateID = decoded.Sandbox.SandboxTemplateID
		}
	}

	// Template ID pinned in version block takes precedence.
	if decoded.Version != nil && decoded.Version.SandboxTemplateID != "" {
		request.TemplateID = decoded.Version.SandboxTemplateID
	}

	return nil
}

// validateEnvVarLiterals rejects any env_var value that contains a
// ${...} placeholder. Sandbox env_vars are intentionally literals
// only:
//
//  1. Per-call exec in E2B does not inherit sandbox-level env (see
//     e2b/session.go:176-184), so secrets injected here would be
//     invisible to agent-spawned processes anyway.
//  2. Any process that DOES see them (boot-time shell) runs as root
//     in the sandbox and shares a uid with the agent, so /proc
//     inspection could leak them.
//
// Pack authors who need to authenticate a remote API should use the
// http_request primitive with ${secrets.*} in headers — that's the
// one hardened path. See issue #186.
func validateEnvVarLiterals(envVars map[string]string) error {
	for key, value := range envVars {
		if idx := strings.Index(value, "${"); idx >= 0 {
			after := value[idx+2:]
			end := strings.Index(after, "}")
			var placeholder string
			if end >= 0 {
				placeholder = "${" + after[:end] + "}"
			} else {
				placeholder = "${" + after + "..."
			}
			if strings.HasPrefix(after, "secrets.") {
				return fmt.Errorf("env_vars[%q] references %s; sandbox env_vars cannot carry secrets — use http_request headers instead (issue #186)", key, placeholder)
			}
			return fmt.Errorf("env_vars[%q] contains placeholder %s; sandbox env_vars must be literal strings", key, placeholder)
		}
	}
	return nil
}

func mergeFilesystem(filesystem *sandbox.FilesystemSpec, workingDirectory string, readableRoots []string, writableRoots []string, maxWorkspaceBytes int64) {
	if trimmed := strings.TrimSpace(workingDirectory); trimmed != "" {
		filesystem.WorkingDirectory = trimmed
	}
	if normalized := normalizeStrings(readableRoots); len(normalized) > 0 {
		filesystem.ReadableRoots = normalized
	}
	if normalized := normalizeStrings(writableRoots); len(normalized) > 0 {
		filesystem.WritableRoots = normalized
	}
	if maxWorkspaceBytes > 0 {
		filesystem.MaxWorkspaceBytes = maxWorkspaceBytes
	}
}
