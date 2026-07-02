package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/sandbox"
)

type sandboxAssetLocation struct {
	Field string
	Asset challengepack.AssetReference
}

func stageArtifactBackedAssets(ctx context.Context, loader AssetLoader, session sandbox.Session, executionContext repository.RunAgentExecutionContext) error {
	assets, err := collectSandboxAssetLocations(executionContext)
	if err != nil {
		return NewFailure(StopReasonSandboxError, "decode challenge pack asset references", err)
	}
	if len(assets) == 0 {
		return nil
	}
	if loader == nil {
		return NewFailure(StopReasonSandboxError, "artifact-backed challenge pack assets require an artifact loader", nil)
	}

	workspaceID := executionContext.Run.WorkspaceID
	for _, item := range assets {
		if item.Asset.ArtifactID == nil {
			continue
		}
		if strings.TrimSpace(item.Asset.Path) == "" {
			return NewFailure(StopReasonSandboxError, fmt.Sprintf("%s.path is required", item.Field), nil)
		}

		content, err := loader.LoadAsset(ctx, workspaceID, *item.Asset.ArtifactID)
		if err != nil {
			return NewFailure(StopReasonSandboxError, fmt.Sprintf("load challenge pack asset %s", item.Field), err)
		}
		if err := session.UploadFile(ctx, item.Asset.Path, content.Content); err != nil {
			return NewFailure(StopReasonSandboxError, fmt.Sprintf("upload challenge pack asset %s", item.Field), err)
		}
	}
	return nil
}

func collectSandboxAssetLocations(executionContext repository.RunAgentExecutionContext) ([]sandboxAssetLocation, error) {
	locations := make([]sandboxAssetLocation, 0)

	manifestLocations, err := collectManifestAssetLocations(executionContext.ChallengePackVersion.Manifest)
	if err != nil {
		return nil, err
	}
	locations = append(locations, manifestLocations...)

	if executionContext.ChallengeInputSet != nil {
		for i, item := range executionContext.ChallengeInputSet.Cases {
			for j, asset := range item.Assets {
				if asset.ArtifactID == nil {
					continue
				}
				locations = append(locations, sandboxAssetLocation{
					Field: fmt.Sprintf("challenge_input_set.cases[%d].assets[%d]", i, j),
					Asset: asset,
				})
			}
		}
	}

	return locations, nil
}

func collectManifestAssetLocations(manifest json.RawMessage) ([]sandboxAssetLocation, error) {
	if len(manifest) == 0 {
		return nil, nil
	}

	type versionBlock struct {
		Assets []challengepack.AssetReference `json:"assets"`
	}
	type challengeBlock struct {
		Key    string                         `json:"key"`
		Assets []challengepack.AssetReference `json:"assets"`
	}
	type manifestShape struct {
		Version    versionBlock     `json:"version"`
		Challenges []challengeBlock `json:"challenges"`
	}

	var decoded manifestShape
	if err := json.Unmarshal(manifest, &decoded); err != nil {
		return nil, err
	}

	locations := make([]sandboxAssetLocation, 0, len(decoded.Version.Assets))
	for i, asset := range decoded.Version.Assets {
		if asset.ArtifactID == nil {
			continue
		}
		locations = append(locations, sandboxAssetLocation{
			Field: fmt.Sprintf("version.assets[%d]", i),
			Asset: asset,
		})
	}
	for i, challenge := range decoded.Challenges {
		for j, asset := range challenge.Assets {
			if asset.ArtifactID == nil {
				continue
			}
			locations = append(locations, sandboxAssetLocation{
				Field: fmt.Sprintf("challenges[%d].assets[%d]", i, j),
				Asset: asset,
			})
		}
	}
	return locations, nil
}
