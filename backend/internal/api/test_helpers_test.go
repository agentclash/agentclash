package api

import (
	"context"
	"errors"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/hostedruns"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type stubHostedRunIngestionService struct{}

func (stubHostedRunIngestionService) IngestEvent(_ context.Context, _ uuid.UUID, _ string, _ hostedruns.Event) error {
	return nil
}

type stubAgentBuildService struct{}

func (stubAgentBuildService) CreateBuild(_ context.Context, _ Caller, _ uuid.UUID, _ CreateAgentBuildInput) (repository.AgentBuild, error) {
	return repository.AgentBuild{}, errors.New("not implemented")
}
func (stubAgentBuildService) GetBuild(_ context.Context, _ uuid.UUID) (repository.AgentBuild, error) {
	return repository.AgentBuild{}, errors.New("not implemented")
}
func (stubAgentBuildService) ListBuilds(_ context.Context, _ uuid.UUID) ([]repository.AgentBuild, error) {
	return nil, errors.New("not implemented")
}
func (stubAgentBuildService) ListVersions(_ context.Context, _ uuid.UUID) ([]repository.AgentBuildVersion, error) {
	return nil, errors.New("not implemented")
}
func (stubAgentBuildService) CreateVersion(_ context.Context, _ Caller, _ uuid.UUID, _ CreateAgentBuildVersionInput) (repository.AgentBuildVersion, error) {
	return repository.AgentBuildVersion{}, errors.New("not implemented")
}
func (stubAgentBuildService) GetVersion(_ context.Context, _ uuid.UUID) (repository.AgentBuildVersion, error) {
	return repository.AgentBuildVersion{}, errors.New("not implemented")
}
func (stubAgentBuildService) UpdateVersion(_ context.Context, _ uuid.UUID, _ UpdateAgentBuildVersionInput) (repository.AgentBuildVersion, error) {
	return repository.AgentBuildVersion{}, errors.New("not implemented")
}
func (stubAgentBuildService) ValidateVersion(_ context.Context, _ uuid.UUID) (ValidateBuildVersionResult, error) {
	return ValidateBuildVersionResult{}, errors.New("not implemented")
}
func (stubAgentBuildService) MarkVersionReady(_ context.Context, _ uuid.UUID) error {
	return errors.New("not implemented")
}
func (stubAgentBuildService) CreateDeployment(_ context.Context, _ Caller, _ uuid.UUID, _ CreateAgentDeploymentInput) (repository.AgentDeploymentRow, error) {
	return repository.AgentDeploymentRow{}, errors.New("not implemented")
}
