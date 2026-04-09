package engine

import (
	"encoding/json"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func TestQuerySQLTool_UsesSQLiteDayOne(t *testing.T) {
	session := sandbox.NewFakeSession("query-sqlite")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 0, Stdout: `[{"id":1,"name":"Ada"}]`})

	result, err := executeQuerySQLTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"engine":"sqlite","query":"select * from users","database_path":"/workspace/app.db"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindData}},
	})
	if err != nil {
		t.Fatalf("executeQuerySQLTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got %#v", result)
	}
}

func TestQuerySQLTool_RejectsUnsupportedEngine(t *testing.T) {
	session := sandbox.NewFakeSession("query-postgres")
	result, err := executeQuerySQLTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"engine":"postgres","query":"select 1"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindData}},
	})
	if err != nil {
		t.Fatalf("executeQuerySQLTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected unsupported engine error, got %#v", result)
	}
}

func TestQuerySQLTool_ReturnsToolErrorForSQLiteFailure(t *testing.T) {
	session := sandbox.NewFakeSession("query-sql-fail")
	session.SetExecResult(sandbox.ExecResult{ExitCode: 1, Stderr: "no such table: users"})

	result, err := executeQuerySQLTool(t.Context(), ToolExecutionRequest{
		Args:       json.RawMessage(`{"engine":"sqlite","query":"select * from users","database_path":"/workspace/app.db"}`),
		Session:    session,
		ToolPolicy: sandbox.ToolPolicy{AllowedToolKinds: []string{toolKindData}},
	})
	if err != nil {
		t.Fatalf("executeQuerySQLTool returned error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected sqlite tool error, got %#v", result)
	}
}
