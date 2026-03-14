package worker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

const defaultSandboxWorkingDirectory = "/workspace"

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

	metadata, err := sandboxMetadata(executionContext)
	if err != nil {
		return sandbox.CreateRequest{}, err
	}

	return sandbox.CreateRequest{
		RunID:      executionContext.Run.ID,
		RunAgentID: executionContext.RunAgent.ID,
		ToolPolicy: policy,
		Filesystem: filesystem,
		Metadata:   metadata,
	}, nil
}

func allowedToolKinds(manifest json.RawMessage) []string {
	type toolPolicy struct {
		AllowedToolKinds []string `json:"allowed_tool_kinds"`
	}
	type challengeManifest struct {
		ToolPolicy *toolPolicy `json:"tool_policy"`
	}

	var decoded challengeManifest
	if err := json.Unmarshal(manifest, &decoded); err != nil || decoded.ToolPolicy == nil {
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
	if err := json.Unmarshal(profileConfig, &decoded); err != nil || decoded.Sandbox == nil {
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

func sandboxMetadata(executionContext repository.RunAgentExecutionContext) (json.RawMessage, error) {
	type metadata struct {
		ChallengePackVersion json.RawMessage `json:"challenge_pack_version,omitempty"`
		RuntimeProfileConfig json.RawMessage `json:"runtime_profile_config,omitempty"`
		DeploymentConfig     json.RawMessage `json:"deployment_config,omitempty"`
	}

	payload, err := json.Marshal(metadata{
		ChallengePackVersion: cloneJSON(executionContext.ChallengePackVersion.Manifest),
		RuntimeProfileConfig: cloneJSON(executionContext.Deployment.RuntimeProfile.ProfileConfig),
		DeploymentConfig:     cloneJSON(executionContext.Deployment.SnapshotConfig),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal sandbox metadata: %w", err)
	}
	return payload, nil
}

func marshalSandboxRunContext(executionContext repository.RunAgentExecutionContext) ([]byte, error) {
	type challengeInputSet struct {
		InputKey      string  `json:"input_key"`
		Name          string  `json:"name"`
		Description   *string `json:"description,omitempty"`
		InputChecksum string  `json:"input_checksum"`
	}
	type runContext struct {
		RunID                string             `json:"run_id"`
		RunAgentID           string             `json:"run_agent_id"`
		ChallengePackVersion json.RawMessage    `json:"challenge_pack_version"`
		ChallengeInputSet    *challengeInputSet `json:"challenge_input_set,omitempty"`
		DeploymentConfig     json.RawMessage    `json:"deployment_config"`
		RuntimeProfileConfig json.RawMessage    `json:"runtime_profile_config"`
	}

	var inputSet *challengeInputSet
	if executionContext.ChallengeInputSet != nil {
		inputSet = &challengeInputSet{
			InputKey:      executionContext.ChallengeInputSet.InputKey,
			Name:          executionContext.ChallengeInputSet.Name,
			Description:   executionContext.ChallengeInputSet.Description,
			InputChecksum: executionContext.ChallengeInputSet.InputChecksum,
		}
	}

	return json.Marshal(runContext{
		RunID:                executionContext.Run.ID.String(),
		RunAgentID:           executionContext.RunAgent.ID.String(),
		ChallengePackVersion: cloneJSON(executionContext.ChallengePackVersion.Manifest),
		ChallengeInputSet:    inputSet,
		DeploymentConfig:     cloneJSON(executionContext.Deployment.SnapshotConfig),
		RuntimeProfileConfig: cloneJSON(executionContext.Deployment.RuntimeProfile.ProfileConfig),
	})
}

func cloneJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func normalizeStrings(values []string) []string {
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cloned = append(cloned, trimmed)
	}
	return cloned
}
