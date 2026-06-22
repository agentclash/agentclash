package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type templateAgentBuildRepository struct {
	build        repository.AgentBuild
	latest       int32
	created      repository.CreateAgentBuildVersionParams
	createdValid bool
}

func (r *templateAgentBuildRepository) GetOrganizationIDByWorkspaceID(context.Context, uuid.UUID) (uuid.UUID, error) {
	return r.build.OrganizationID, nil
}

func (r *templateAgentBuildRepository) CreateAgentBuild(context.Context, repository.CreateAgentBuildParams) (repository.AgentBuild, error) {
	return repository.AgentBuild{}, nil
}

func (r *templateAgentBuildRepository) GetAgentBuildByID(_ context.Context, id uuid.UUID) (repository.AgentBuild, error) {
	if id != r.build.ID {
		return repository.AgentBuild{}, repository.ErrAgentBuildNotFound
	}
	return r.build, nil
}

func (r *templateAgentBuildRepository) ListAgentBuildsByWorkspaceID(context.Context, uuid.UUID) ([]repository.AgentBuild, error) {
	return nil, nil
}

func (r *templateAgentBuildRepository) CreateAgentBuildVersion(_ context.Context, params repository.CreateAgentBuildVersionParams) (repository.AgentBuildVersion, error) {
	r.created = params
	r.createdValid = true
	return repository.AgentBuildVersion{
		ID:               uuid.New(),
		AgentBuildID:     params.AgentBuildID,
		VersionNumber:    params.VersionNumber,
		VersionStatus:    "draft",
		AgentKind:        params.AgentKind,
		InterfaceSpec:    params.InterfaceSpec,
		PolicySpec:       params.PolicySpec,
		ReasoningSpec:    params.ReasoningSpec,
		MemorySpec:       params.MemorySpec,
		WorkflowSpec:     params.WorkflowSpec,
		GuardrailSpec:    params.GuardrailSpec,
		ModelSpec:        params.ModelSpec,
		OutputSchema:     params.OutputSchema,
		TraceContract:    params.TraceContract,
		PublicationSpec:  params.PublicationSpec,
		CreatedByUserID:  params.CreatedByUserID,
		CreatedAt:        time.Now().UTC(),
		Tools:            params.Tools,
		KnowledgeSources: params.KnowledgeSources,
	}, nil
}

func (r *templateAgentBuildRepository) GetAgentBuildVersionByID(context.Context, uuid.UUID) (repository.AgentBuildVersion, error) {
	return repository.AgentBuildVersion{}, repository.ErrAgentBuildVersionNotFound
}

func (r *templateAgentBuildRepository) GetLatestVersionNumberForBuild(context.Context, uuid.UUID) (int32, error) {
	return r.latest, nil
}

func (r *templateAgentBuildRepository) ListAgentBuildVersionsByBuildID(context.Context, uuid.UUID) ([]repository.AgentBuildVersion, error) {
	return nil, nil
}

func (r *templateAgentBuildRepository) UpdateAgentBuildVersionDraft(context.Context, repository.UpdateAgentBuildVersionDraftParams) error {
	return nil
}

func (r *templateAgentBuildRepository) MarkAgentBuildVersionReady(context.Context, uuid.UUID) error {
	return nil
}

func (r *templateAgentBuildRepository) CreateAgentDeployment(context.Context, repository.CreateAgentDeploymentParams) (repository.AgentDeploymentRow, error) {
	return repository.AgentDeploymentRow{}, nil
}

func (r *templateAgentBuildRepository) GetProviderAccountByID(context.Context, uuid.UUID) (repository.ProviderAccountRow, error) {
	return repository.ProviderAccountRow{}, nil
}

func TestCreateVersionAppliesTemplateDefaultsAndExplicitOverrides(t *testing.T) {
	buildID := uuid.New()
	userID := uuid.New()
	repo := &templateAgentBuildRepository{
		build: repository.AgentBuild{
			ID:             buildID,
			OrganizationID: uuid.New(),
			WorkspaceID:    uuid.New(),
		},
		latest: 3,
	}
	manager := NewAgentBuildManager(repo)

	version, err := manager.CreateVersion(context.Background(), Caller{UserID: userID}, buildID, CreateAgentBuildVersionInput{
		Template:     "honest-agent",
		PolicySpec:   json.RawMessage(`null`),
		ModelSpec:    json.RawMessage(`{"temperature":0}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"custom":{"type":"string"}}}`),
	})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}
	if !repo.createdValid {
		t.Fatal("CreateAgentBuildVersion was not called")
	}
	if version.VersionNumber != 4 || repo.created.VersionNumber != 4 {
		t.Fatalf("version number = %d/%d, want 4", version.VersionNumber, repo.created.VersionNumber)
	}
	if repo.created.AgentKind != "llm_agent" {
		t.Fatalf("agent kind = %q, want llm_agent", repo.created.AgentKind)
	}
	if !strings.Contains(string(repo.created.PolicySpec), "Answer honestly and directly") {
		t.Fatalf("policy spec did not come from honest-agent template: %s", repo.created.PolicySpec)
	}
	if string(repo.created.ModelSpec) != `{"temperature":0}` {
		t.Fatalf("model spec = %s, want explicit override", repo.created.ModelSpec)
	}
	if !strings.Contains(string(repo.created.OutputSchema), `"custom"`) {
		t.Fatalf("output schema = %s, want explicit override", repo.created.OutputSchema)
	}
}

func TestCreateVersionRejectsUnknownTemplate(t *testing.T) {
	buildID := uuid.New()
	repo := &templateAgentBuildRepository{
		build: repository.AgentBuild{
			ID:             buildID,
			OrganizationID: uuid.New(),
			WorkspaceID:    uuid.New(),
		},
	}
	manager := NewAgentBuildManager(repo)

	_, err := manager.CreateVersion(context.Background(), Caller{UserID: uuid.New()}, buildID, CreateAgentBuildVersionInput{
		Template: "missing-template",
	})
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
	var validationErr AgentBuildValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want AgentBuildValidationError", err)
	}
	if validationErr.Code != "invalid_template" || !strings.Contains(validationErr.Message, "code-reviewer") || !strings.Contains(validationErr.Message, "honest-agent") {
		t.Fatalf("validation error = %#v", validationErr)
	}
}

func TestListAgentBuildVersionTemplateResponsesIncludesBuiltIns(t *testing.T) {
	items := listAgentBuildVersionTemplateResponses()
	got := make(map[string]agentBuildVersionTemplateResponse, len(items))
	for _, item := range items {
		got[item.Key] = item
	}

	for _, key := range []string{"code-reviewer", "honest-agent"} {
		item, ok := got[key]
		if !ok {
			t.Fatalf("template %q missing from catalog: %+v", key, got)
		}
		if item.AgentKind != "llm_agent" || len(item.PolicySpec) == 0 {
			t.Fatalf("template %q response incomplete: %+v", key, item)
		}
	}
}
