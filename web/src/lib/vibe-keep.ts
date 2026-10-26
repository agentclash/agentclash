import type { SavedCheck } from "./vibe";

export function savedWorkURL(
  item: Pick<SavedCheck, "session_id" | "artifact_id" | "baseline_operation_id" | "workspace_id">,
) {
  const params = new URLSearchParams({
    session: item.session_id,
    agent: item.artifact_id,
    workspace: item.workspace_id,
    view: "agent",
  });
  if (
    item.baseline_operation_id &&
    item.baseline_operation_id !== "00000000-0000-0000-0000-000000000000"
  ) {
    params.set("view", "results");
    params.set("run", item.baseline_operation_id);
  }
  return `/vibe-evals?${params}`;
}

export function keepReturnURL(
  session: string,
  artifact: string,
  workspace: string,
  baseline?: string,
  returnPath?: string,
  advanced = false,
) {
  const safe = safeVibeReturn(returnPath);
  const params = new URL(safe || "/vibe-evals", "https://local.invalid").searchParams;
  params.set("session", session);
  params.set("agent", artifact);
  params.set("keep", "1");
  if (advanced) params.set("keep_advanced", "1");
  if (workspace) params.set("workspace", workspace);
  if (baseline) params.set("keep_run", baseline);
  return `/vibe-evals?${params}`;
}

export function safeVibeReturn(path?: string) {
  if (!path || !path.startsWith("/vibe-evals") || path.startsWith("//")) return undefined;
  try {
    const url = new URL(path, "https://local.invalid");
    if (url.origin !== "https://local.invalid" || url.pathname !== "/vibe-evals") return undefined;
    return url.pathname + url.search;
  } catch {
    return undefined;
  }
}
export function advancedURL(workspace: string, draft: string, returnPath?: string, title?: string) {
  const safe = safeVibeReturn(returnPath);
  return `/workspaces/${encodeURIComponent(workspace)}/challenge-packs/builder/${encodeURIComponent(draft)}${safe ? `?return_to=${encodeURIComponent(safe)}${title ? `&return_title=${encodeURIComponent(title)}` : ""}` : ""}`;
}

// Return links name the frozen version even when the original URL selected it implicitly.
export function pinVibeReturn(path: string, session: string, artifact: string, run?: string) {
  const url = new URL(safeVibeReturn(path) || "/vibe-evals", "https://local.invalid");
  url.searchParams.set("session", session);
  url.searchParams.set("agent", artifact);
  if (run) url.searchParams.set("run", run);
  else url.searchParams.delete("run");
  for (const key of ["keep", "keep_run", "keep_advanced"]) url.searchParams.delete(key);
  return url.pathname + url.search;
}
