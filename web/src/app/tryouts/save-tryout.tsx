"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useAccessToken, useAuth } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { claimAgentTryout } from "@/lib/api/agent-tryouts";
import type { AgentTryout, UserMeResponse, UserMeWorkspace } from "@/lib/api/types";

export function SaveTryout({ tryout, loginHref, onSaved }: {
  tryout: AgentTryout; loginHref: string; onSaved: (result: AgentTryout) => void;
}) {
  const { user, loading } = useAuth();
  const userId = user?.id;
  const { getAccessToken } = useAccessToken();
  const token = useRef(getAccessToken);
  useEffect(() => { token.current = getAccessToken; }, [getAccessToken]);
  const [workspaces, setWorkspaces] = useState<UserMeWorkspace[]>([]);
  const [workspaceId, setWorkspaceId] = useState("");
  const [ready, setReady] = useState(false);
  const [needsSetup, setNeedsSetup] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  useEffect(() => {
    let active = true;
    if (!userId || tryout.save_eligible !== true || tryout.workspace_id) return;
    void token.current().then(async accessToken => {
      if (!accessToken) throw new Error("Sign in again to save this tryout.");
      const me = await createApiClient(accessToken).get<UserMeResponse>("/v1/users/me", { cache: "no-store" });
      if (!active) return;
      setNeedsSetup(me.organizations.length === 0);
      setWorkspaces(me.organizations.flatMap(org => org.workspaces.filter(workspace =>
        org.role === "org_admin" || ["workspace_admin", "workspace_member"].includes(workspace.role),
      )));
      setReady(true);
    }).catch(err => { if (active) setError(err instanceof Error ? err.message : "Could not load workspaces."); });
    return () => { active = false; };
  }, [userId, tryout.id, tryout.workspace_id, tryout.save_eligible, attempt]);

  async function save() {
    if (saving || tryout.save_eligible !== true || !workspaces.some(workspace => workspace.id === workspaceId)) return;
    setSaving(true); setError(null);
    try {
      const accessToken = await token.current();
      if (!accessToken) throw new Error("Sign in again to save this tryout.");
      const result = await claimAgentTryout(createApiClient(accessToken), tryout.id, workspaceId);
      if (result.id !== tryout.id || result.workspace_id !== workspaceId || !result.claimed_by_user_id || !result.claimed_at) {
        throw new Error("Could not confirm this save. Reload the result to check its status.");
      }
      if (mounted.current) onSaved(result);
    } catch (err) {
      if (mounted.current) setError(err instanceof Error ? err.message : "Could not save this tryout. Try again.");
    } finally { if (mounted.current) setSaving(false); }
  }

  return <section id="save-tryout" aria-label="Save tryout" className="shrink-0 border-b border-white/10 px-4 py-3 text-sm">
    {tryout.workspace_id ? <p role="status">Saved result <span className="break-all">{tryout.id}</span> to workspace <span className="break-all">{tryout.workspace_id}</span>.</p>
      : tryout.save_eligible !== true ? <p>This result cannot be saved from this browser. <Link href="/tryouts" className="underline">Start a new tryout</Link>.</p>
      : loading ? <p>Checking your account…</p>
      : !user ? <Link href={loginHref} className="underline">Sign in to save this result</Link>
      : <div className="flex flex-wrap items-center gap-3">
        {ready && workspaces.length > 0 ? <>
          <label htmlFor="tryout-workspace">Save to workspace</label>
          <select id="tryout-workspace" value={workspaceId} disabled={saving} onChange={event => setWorkspaceId(event.target.value)} className="max-w-full rounded border border-white/20 bg-[#131312] p-2">
            <option value="">Choose a workspace</option>
            {workspaces.map(workspace => <option key={workspace.id} value={workspace.id}>{workspace.name}</option>)}
          </select>
          <button type="button" disabled={!workspaceId || saving} onClick={() => void save()} className="rounded bg-white px-3 py-2 text-black disabled:opacity-40">{saving ? "Saving…" : "Save result"}</button>
        </> : ready ? <p>No writable workspace is available. <Link className="underline" href={needsSetup ? `/onboard?${new URLSearchParams({ returnTo: `/tryouts?tryout=${tryout.id}` })}` : "/dashboard"}>Open your account</Link>.</p> : !error ? <p>Loading workspaces…</p> : <button type="button" onClick={() => { setReady(false); setError(null); setAttempt(value => value + 1); }} className="underline">Retry workspaces</button>}
        {error && <p role="alert" className="w-full">{error} <Link href="/tryouts" className="underline">Start a new tryout</Link>.</p>}
      </div>}
  </section>;
}
