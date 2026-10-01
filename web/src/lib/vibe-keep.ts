import type { SavedCheck } from "./vibe";

export function savedWorkURL(item: Pick<SavedCheck, "session_id" | "artifact_id" | "baseline_operation_id" | "workspace_id">) {
  const params = new URLSearchParams({ session: item.session_id, agent: item.artifact_id, workspace: item.workspace_id });
  if (item.baseline_operation_id && item.baseline_operation_id !== "00000000-0000-0000-0000-000000000000") {
    params.set("view", "checks");
    params.set("run", item.baseline_operation_id);
  }
  return `/vibe-evals?${params}`;
}

export function keepReturnURL(session: string, artifact: string, workspace: string, baseline?: string) {
  const params = new URLSearchParams({ session, agent: artifact, keep: "1" });
  if (workspace) params.set("workspace", workspace);
  if (baseline) params.set("keep_run", baseline);
  return `/vibe-evals?${params}`;
}
