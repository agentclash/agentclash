package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/challengepack"
	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	"github.com/google/uuid"
)

func TestStageArtifactBackedAssetsUploadsManifestAndInputCaseAssets(t *testing.T) {
	ctx := context.Background()
	workspaceID := uuid.New()
	versionAssetID := uuid.New()
	challengeAssetID := uuid.New()
	caseAssetID := uuid.New()

	executionContext := repository.RunAgentExecutionContext{
		Run: domain.Run{WorkspaceID: workspaceID},
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			Manifest: []byte(fmt.Sprintf(`{
				"version": {
					"assets": [
						{"key": "global-data", "path": "/workspace/assets/global.csv", "artifact_id": %q}
					]
				},
				"challenges": [
					{
						"key": "forecast",
						"assets": [
							{"key": "rubric", "path": "/workspace/assets/rubric.md", "artifact_id": %q}
						]
					}
				]
			}`, versionAssetID, challengeAssetID)),
		},
		ChallengeInputSet: &repository.ChallengeInputSetExecutionContext{
			Cases: []repository.ChallengeCaseExecutionContext{
				{
					CaseKey: "forecast-case",
					Assets: []challengepack.AssetReference{
						{Key: "case-data", Path: "/workspace/cases/input.json", ArtifactID: &caseAssetID},
					},
				},
			},
		},
	}

	session := sandbox.NewFakeSession("asset-sandbox")
	loader := fakeAssetLoader{
		workspaceID: workspaceID,
		content: map[uuid.UUID][]byte{
			versionAssetID:   []byte("date,value\n2026-05-03,42\n"),
			challengeAssetID: []byte("# Rubric\nUse the provided data.\n"),
			caseAssetID:      []byte(`{"region":"apac"}`),
		},
	}

	executor := NativeExecutor{}.WithAssetLoader(loader)
	if err := stageArtifactBackedAssets(ctx, executor.assetLoader, session, executionContext); err != nil {
		t.Fatalf("stageArtifactBackedAssets returned error: %v", err)
	}

	files := session.Files()
	assertFileContent(t, files, "/workspace/assets/global.csv", "date,value\n2026-05-03,42\n")
	assertFileContent(t, files, "/workspace/assets/rubric.md", "# Rubric\nUse the provided data.\n")
	assertFileContent(t, files, "/workspace/cases/input.json", `{"region":"apac"}`)
}

func TestStageArtifactBackedAssetsSkipsInlineAssetsWithoutLoader(t *testing.T) {
	executionContext := repository.RunAgentExecutionContext{
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			Manifest: []byte(`{
				"version": {"assets": [{"key": "inline", "path": "/workspace/assets/inline.txt"}]},
				"challenges": [{"key": "forecast", "assets": [{"key": "guide", "path": "/workspace/assets/guide.md"}]}]
			}`),
		},
		ChallengeInputSet: &repository.ChallengeInputSetExecutionContext{
			Cases: []repository.ChallengeCaseExecutionContext{
				{
					CaseKey: "forecast-case",
					Assets: []challengepack.AssetReference{
						{Key: "case-inline", Path: "/workspace/cases/inline.json"},
					},
				},
			},
		},
	}

	session := sandbox.NewFakeSession("inline-assets")
	if err := stageArtifactBackedAssets(context.Background(), nil, session, executionContext); err != nil {
		t.Fatalf("stageArtifactBackedAssets returned error: %v", err)
	}
	if files := session.Files(); len(files) != 0 {
		t.Fatalf("uploaded files = %#v, want none", files)
	}
}

func TestStageArtifactBackedAssetsFailsClosedWithoutLoader(t *testing.T) {
	assetID := uuid.New()
	executionContext := repository.RunAgentExecutionContext{
		ChallengePackVersion: repository.ChallengePackVersionExecutionContext{
			Manifest: []byte(fmt.Sprintf(`{
				"version": {
					"assets": [
						{"key": "global-data", "path": "/workspace/assets/global.csv", "artifact_id": %q}
					]
				}
			}`, assetID)),
		},
	}

	session := sandbox.NewFakeSession("missing-loader")
	err := stageArtifactBackedAssets(context.Background(), nil, session, executionContext)
	if err == nil {
		t.Fatal("stageArtifactBackedAssets returned nil error")
	}
	failure, ok := AsFailure(err)
	if !ok {
		t.Fatalf("error type = %T, want engine Failure", err)
	}
	if failure.StopReason != StopReasonSandboxError {
		t.Fatalf("stop reason = %s, want sandbox_error", failure.StopReason)
	}
	if got := session.Files(); len(got) != 0 {
		t.Fatalf("uploaded files = %#v, want none", got)
	}
}

type fakeAssetLoader struct {
	workspaceID uuid.UUID
	content     map[uuid.UUID][]byte
}

func (l fakeAssetLoader) LoadAsset(_ context.Context, workspaceID uuid.UUID, artifactID uuid.UUID) (AssetContent, error) {
	if workspaceID != l.workspaceID {
		return AssetContent{}, fmt.Errorf("workspace = %s, want %s", workspaceID, l.workspaceID)
	}
	content, ok := l.content[artifactID]
	if !ok {
		return AssetContent{}, errors.New("asset not found")
	}
	return AssetContent{Content: append([]byte(nil), content...)}, nil
}

func assertFileContent(t *testing.T, files map[string][]byte, path string, want string) {
	t.Helper()

	got, ok := files[path]
	if !ok {
		t.Fatalf("file %s was not uploaded; files=%#v", path, files)
	}
	if string(got) != want {
		t.Fatalf("file %s = %q, want %q", path, string(got), want)
	}
}
