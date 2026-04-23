package repository

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func TestDecodeAgentBuildVersionExecutionContext_DecodesFrozenToolBindings(t *testing.T) {
	buildVersionID := uuid.New()
	toolID := uuid.New()

	payload := []byte(fmt.Sprintf(`{
		"agent_kind":"llm_agent",
		"policy_spec":{"instructions":"Use tools"},
		"tools":[
			{
				"tool_id":"%s",
				"binding_config":{"tool_name":"github_create_issue"}
			}
		]
	}`, toolID))

	executionContext, err := decodeAgentBuildVersionExecutionContext(buildVersionID, payload)
	if err != nil {
		t.Fatalf("decodeAgentBuildVersionExecutionContext returned error: %v", err)
	}
	if executionContext.ID != buildVersionID {
		t.Fatalf("build version id = %s, want %s", executionContext.ID, buildVersionID)
	}
	if len(executionContext.Tools) != 1 {
		t.Fatalf("tool binding count = %d, want 1", len(executionContext.Tools))
	}
	if executionContext.Tools[0].ToolID != toolID {
		t.Fatalf("tool id = %s, want %s", executionContext.Tools[0].ToolID, toolID)
	}
	if executionContext.Tools[0].BindingRole != "default" {
		t.Fatalf("binding role = %q, want default", executionContext.Tools[0].BindingRole)
	}

	var bindingConfig map[string]any
	if err := json.Unmarshal(executionContext.Tools[0].BindingConfig, &bindingConfig); err != nil {
		t.Fatalf("decode binding config: %v", err)
	}
	if bindingConfig["tool_name"] != "github_create_issue" {
		t.Fatalf("binding_config.tool_name = %#v, want github_create_issue", bindingConfig["tool_name"])
	}
}
