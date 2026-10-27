"use client";
import { useEffect, useMemo, useRef, useState } from "react";
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
export function useVibeWorkspaceAccess(actorScope: string, workspaceId: string | undefined, token: () => Promise<string | null | undefined>, authLoading = false, refresh = 0) {
  const [attempt, setAttempt] = useState(0);
  const life = useVibeEvidenceOwner(actorScope, String(authLoading), String(attempt), String(refresh));
  const tokenRef = useRef(token);
  useEffect(() => { tokenRef.current = token; }, [token]);
  const [state, setState] = useState<{ owner: object; status: WorkspaceEvidenceAccess["status"]; me?: UserMeResponse }>();
  useEffect(() => {
    let live = true; const captured = life.owner, current = life.capture();
    const publish = (status: WorkspaceEvidenceAccess["status"], me?: UserMeResponse) => { if (live && current()) setState({ owner: captured, status, me }); };
    if (!authLoading) void tokenRef.current().then(async auth => {
      if (!live || !current()) return;
      if (!auth) { publish("signed_out"); return; }
      const me = await createApiClient(auth).get<UserMeResponse>("/v1/users/me", { cache: "no-store" });
      publish("ready", me);
    }).catch(error => publish(error?.status === 401 ? "signed_out" : "unavailable"));
    return () => { live = false; };
  }, [authLoading, life]);
  const active = !authLoading && state?.owner === life.owner ? state : undefined;
  const status = active?.status || "loading";
  const workspaces = useMemo(() => active?.me?.organizations.flatMap(org => org.workspaces.filter(workspace =>
    org.role === "org_admin" || ["workspace_admin", "workspace_member"].includes(workspace.role),
  )) || [], [active?.me]);
  return { status, workspaces, access: active?.me && workspaceId ? workspaceEvidenceAccess(active.me, workspaceId) : denied(status === "ready" ? "unavailable" : status), retry: () => setAttempt(n => n + 1) };
}
