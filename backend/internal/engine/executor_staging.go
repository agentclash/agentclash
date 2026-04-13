package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/challengepack"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func marshalSandboxRunContext(executionContext repository.RunAgentExecutionContext) ([]byte, error) {
	return json.Marshal(map[string]any{
		"run_id":                 executionContext.Run.ID.String(),
		"run_agent_id":           executionContext.RunAgent.ID.String(),
		"agent_spec":             cloneJSON(executionContext.Deployment.AgentBuildVersion.AgentSpec),
		"challenge_pack_version": cloneJSON(executionContext.ChallengePackVersion.Manifest),
		"challenge_input_set":    cloneChallengeInputSet(executionContext.ChallengeInputSet),
		"deployment_config":      cloneJSON(executionContext.Deployment.SnapshotConfig),
		"runtime_profile_config": cloneJSON(executionContext.Deployment.RuntimeProfile.ProfileConfig),
	})
}

func stageSandboxInputs(ctx context.Context, session sandbox.Session, executionContext repository.RunAgentExecutionContext) error {
	runContextPayload, err := marshalSandboxRunContext(executionContext)
	if err != nil {
		return NewFailure(StopReasonSandboxError, "marshal native sandbox context", err)
	}
	if err := session.UploadFile(ctx, "/workspace/agentclash/run-context.json", runContextPayload); err != nil {
		return NewFailure(StopReasonSandboxError, "upload native sandbox context", err)
	}
	if err := session.UploadFile(ctx, "/workspace/agentclash/challenge-pack-manifest.json", cloneJSON(executionContext.ChallengePackVersion.Manifest)); err != nil {
		return NewFailure(StopReasonSandboxError, "upload challenge pack manifest", err)
	}
	challengesPayload, err := json.Marshal(executionContext.ChallengePackVersion.Challenges)
	if err != nil {
		return NewFailure(StopReasonSandboxError, "marshal challenge definitions", err)
	}
	if err := session.UploadFile(ctx, "/workspace/agentclash/challenges.json", challengesPayload); err != nil {
		return NewFailure(StopReasonSandboxError, "upload challenge definitions", err)
	}
	if executionContext.ChallengeInputSet != nil {
		inputSetPayload, err := json.Marshal(executionContext.ChallengeInputSet)
		if err != nil {
			return NewFailure(StopReasonSandboxError, "marshal challenge input set", err)
		}
		if err := session.UploadFile(ctx, "/workspace/agentclash/challenge-input-set.json", inputSetPayload); err != nil {
			return NewFailure(StopReasonSandboxError, "upload challenge input set", err)
		}
		if err := stageWorkspaceFixtureFiles(ctx, session, executionContext.ChallengeInputSet.Cases); err != nil {
			return err
		}
	}
	return nil
}

type workspaceFixtureFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func stageWorkspaceFixtureFiles(ctx context.Context, session sandbox.Session, cases []repository.ChallengeCaseExecutionContext) error {
	for _, item := range cases {
		files, err := extractWorkspaceFixtureFiles(item)
		if err != nil {
			return NewFailure(StopReasonSandboxError, "decode workspace fixture files", err)
		}
		for _, file := range files {
			if strings.TrimSpace(file.Path) == "" {
				continue
			}
			if err := session.UploadFile(ctx, file.Path, []byte(file.Content)); err != nil {
				return NewFailure(StopReasonSandboxError, "upload workspace fixture file", err)
			}
		}
	}
	return nil
}

func extractWorkspaceFixtureFiles(item repository.ChallengeCaseExecutionContext) ([]workspaceFixtureFile, error) {
	files := make([]workspaceFixtureFile, 0)
	if len(bytes.TrimSpace(item.Payload)) > 0 {
		var decoded struct {
			WorkspaceFiles []workspaceFixtureFile `json:"workspace_files"`
		}
		if err := json.Unmarshal(item.Payload, &decoded); err != nil {
			return nil, err
		}
		files = append(files, decoded.WorkspaceFiles...)
	}
	for _, input := range item.Inputs {
		if input.Kind != "workspace" {
			continue
		}
		inputFiles, err := decodeWorkspaceInputFiles(item, input)
		if err != nil {
			return nil, err
		}
		files = append(files, inputFiles...)
	}
	return files, nil
}

func decodeWorkspaceInputFiles(item repository.ChallengeCaseExecutionContext, input challengepack.CaseInput) ([]workspaceFixtureFile, error) {
	value, ok := input.Value.([]any)
	if !ok {
		return nil, fmt.Errorf(
			"workspace input %q for case %q must be an array of file objects",
			input.Key,
			item.CaseKey,
		)
	}

	files := make([]workspaceFixtureFile, 0, len(value))
	for index, rawFile := range value {
		fileMap, ok := rawFile.(map[string]any)
		if !ok {
			return nil, fmt.Errorf(
				"workspace input %q for case %q file[%d] must be an object",
				input.Key,
				item.CaseKey,
				index,
			)
		}

		path, ok := fileMap["path"].(string)
		if !ok {
			return nil, fmt.Errorf(
				"workspace input %q for case %q file[%d].path must be a string",
				input.Key,
				item.CaseKey,
				index,
			)
		}
		content, ok := fileMap["content"].(string)
		if !ok {
			return nil, fmt.Errorf(
				"workspace input %q for case %q file[%d].content must be a string",
				input.Key,
				item.CaseKey,
				index,
			)
		}

		files = append(files, workspaceFixtureFile{Path: path, Content: content})
	}

	return files, nil
}
