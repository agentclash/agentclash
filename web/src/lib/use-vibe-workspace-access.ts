"use client";
import { useEffect, useRef, useState } from "react";
import { createApiClient } from "./api/client";
import type { UserMeResponse } from "./api/types";
import { useVibeEvidenceOwner } from "./use-vibe-evidence-owner";
export type WorkspaceEvidenceAccess = { status: "loading" | "ready" | "signed_out" | "unavailable"; canRead: boolean; canWrite: boolean; canAdmin: boolean };
const denied = (status: WorkspaceEvidenceAccess["status"]): WorkspaceEvidenceAccess => ({ status, canRead: false, canWrite: false, canAdmin: false });
export function workspaceEvidenceAccess(me: UserMeResponse, workspaceId: string): WorkspaceEvidenceAccess {
  for (const org of me.organizations) {
    const workspace = org.workspaces.find(w => w.id === workspaceId);
    if (!workspace) continue;
    const admin = org.role === "org_admin" || workspace.role === "workspace_admin";
    const write = admin || workspace.role === "workspace_member";
    return { status: "ready", canRead: write || workspace.role === "workspace_viewer", canWrite: write, canAdmin: admin };
  }
  return denied("ready");
}
// One controller-owned lookup; forms consume the result instead of discovering
// workspaces or subscribing to the project themselves.
export function useVibeWorkspaceAccess(actorScope: string, workspaceId: string | undefined, token: () => Promise<string | null | undefined>, authLoading = false) {
  const [attempt, setAttempt] = useState(0);
  const life = useVibeEvidenceOwner(actorScope, workspaceId, String(authLoading), String(attempt));
  const tokenRef = useRef(token);
  useEffect(() => { tokenRef.current = token; }, [token]);
  const [state, setState] = useState<{ owner: object; access: WorkspaceEvidenceAccess }>();
  useEffect(() => {
    let live = true; const captured = life.owner, current = life.capture();
    const publish = (access: WorkspaceEvidenceAccess) => { if (live && current()) setState({ owner: captured, access }); };
    if (!authLoading) void tokenRef.current().then(async auth => {
      if (!live || !current()) return;
      if (!auth) { publish(denied("signed_out")); return; }
      if (!workspaceId) { publish(denied("unavailable")); return; }
      const me = await createApiClient(auth).get<UserMeResponse>("/v1/users/me", { cache: "no-store" });
      publish(workspaceEvidenceAccess(me, workspaceId));
    }).catch(() => publish(denied("unavailable")));
    return () => { live = false; };
  }, [workspaceId, authLoading, life]);
  return { access: !authLoading && state?.owner === life.owner ? state.access : denied("loading"), retry: () => setAttempt(n => n + 1) };
}
