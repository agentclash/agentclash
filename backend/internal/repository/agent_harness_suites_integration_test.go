package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

func TestRepositoryAgentHarnessSuitePersistsVersionedTaskBank(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	suite, err := repo.CreateAgentHarnessSuite(ctx, repository.CreateAgentHarnessSuiteParams{
		OrganizationID:  fixture.organizationID,
		WorkspaceID:     fixture.workspaceID,
		CreatedByUserID: &fixture.userID,
		Name:            "Rust Private Bank",
		Slug:            "rust-private-bank",
		Description:     "Private Rust coding tasks",
		Metadata:        json.RawMessage(`{"suite_kind":"private_task_bank"}`),
		Tasks: []repository.CreateAgentHarnessSuiteTaskParams{{
			Title:          "Fix ownership bug",
			PublicPrompt:   "Fix a Rust compile failure.",
			TaskPrompt:     "Fix the hidden borrow-checker failure and open a PR.",
			SourceType:     "github_issue",
			SourceSnapshot: json.RawMessage(`{"repository":"acme/rusty","number":17,"labels":["rust"]}`),
			RepositoryURL:  stringPtr("https://github.com/acme/rusty"),
			BaseBranch:     stringPtr("main"),
			ExecutionConfig: json.RawMessage(`{
				"setup_commands": ["cargo fetch"],
				"timeout_seconds": 900
			}`),
			EvaluationConfig: json.RawMessage(`{
				"validators": [{"type":"command","command":"cargo test --all"}],
				"llm_judges": [{"key":"pr_quality"}],
				"redact_replay": true
			}`),
			Metadata: json.RawMessage(`{"difficulty":"medium"}`),
		}},
	})
	if err != nil {
		t.Fatalf("CreateAgentHarnessSuite returned error: %v", err)
	}

	if suite.CurrentVersionNumber != 1 || suite.CurrentVersionID == uuid.Nil || suite.TaskCount != 1 {
		t.Fatalf("suite version/task count = %d/%s/%d, want v1 with one task", suite.CurrentVersionNumber, suite.CurrentVersionID, suite.TaskCount)
	}
	listed, err := repo.ListAgentHarnessSuitesByWorkspaceID(ctx, fixture.workspaceID)
	if err != nil {
		t.Fatalf("ListAgentHarnessSuitesByWorkspaceID returned error: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != suite.ID || listed[0].TaskCount != 1 {
		t.Fatalf("listed suites = %#v, want persisted suite", listed)
	}
	tasks, err := repo.ListAgentHarnessSuiteTasksByVersionID(ctx, suite.CurrentVersionID)
	if err != nil {
		t.Fatalf("ListAgentHarnessSuiteTasksByVersionID returned error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.SourceType != "github_issue" || task.RepositoryURL == nil || *task.RepositoryURL != "https://github.com/acme/rusty" {
		t.Fatalf("task source/repo = %q/%#v, want github issue repo snapshot", task.SourceType, task.RepositoryURL)
	}
	if string(task.EvaluationConfig) == "{}" || string(task.SourceSnapshot) == "{}" {
		t.Fatalf("task hidden evaluation/source snapshot not preserved: eval=%s source=%s", task.EvaluationConfig, task.SourceSnapshot)
	}

	_, err = repo.CreateAgentHarnessSuite(ctx, repository.CreateAgentHarnessSuiteParams{
		OrganizationID:  fixture.organizationID,
		WorkspaceID:     fixture.workspaceID,
		CreatedByUserID: &fixture.userID,
		Name:            "Rust Private Bank Again",
		Slug:            "rust-private-bank",
		Tasks: []repository.CreateAgentHarnessSuiteTaskParams{{
			Title:      "Duplicate slug task",
			TaskPrompt: "Do the hidden task.",
			SourceType: "manual",
		}},
	})
	if !errors.Is(err, repository.ErrAgentHarnessSuiteSlugConflict) {
		t.Fatalf("duplicate slug error = %v, want ErrAgentHarnessSuiteSlugConflict", err)
	}
}
