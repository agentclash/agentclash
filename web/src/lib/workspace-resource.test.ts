import { describe, expect, it } from "vitest";
import { workspaceMutationKeys, workspaceResourceKeys } from "./workspace-resource";

describe("workspaceResourceKeys", () => {
  it("builds stable keys for paginated run data", () => {
    expect(workspaceResourceKeys.runs("ws-1", 40)).toEqual([
      "/v1/workspaces/ws-1/runs",
      { limit: 20, offset: 40 },
    ]);
  });

  it("builds stable keys for eval sessions", () => {
    expect(workspaceResourceKeys.evalSessions("ws-1", 0)).toEqual([
      "/v1/eval-sessions",
      { workspace_id: "ws-1", limit: 20, offset: 0 },
    ]);
  });

  it("includes dependent resource keys for deployment dialogs", () => {
    expect(workspaceMutationKeys.createDeploymentDialog("ws-1")).toEqual([
      ["/v1/workspaces/ws-1/agent-builds"],
      ["/v1/workspaces/ws-1/runtime-profiles"],
      ["/v1/workspaces/ws-1/provider-accounts"],
      ["/v1/workspaces/ws-1/model-aliases"],
    ]);
  });
});
