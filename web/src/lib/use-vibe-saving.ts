"use client";
import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from "react";
import { useSearchParams } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { UserMeResponse } from "@/lib/api/types";
import { vibeFetch, VibeError, terminal, type Session, type Artifact, type Operation, type SavedCheck, type Models } from "@/lib/vibe";
type Context = {
 session: Session | null; artifact?: Artifact; artifacts: Artifact[]; content: string; models: Models; workspace: string;
 actorScope?: string;
 loadWorkspace?: boolean;
 token: () => Promise<string | undefined>; authLoading: boolean; loadAttempt: number;
 setContent: Dispatch<SetStateAction<string>>; setModels: Dispatch<SetStateAction<Models>>; setWorkspace: Dispatch<SetStateAction<string>>;
 setError: Dispatch<SetStateAction<string>>; setPending: Dispatch<SetStateAction<boolean>>; setSavedChecks: Dispatch<SetStateAction<SavedCheck[]>>;
 setSelectedArtifactID: (id: string | null) => void; reload: () => Promise<Session | undefined>; writeURL: (url: URL) => void;
};
export function useVibeSaving(c: Context) {
 const { actorScope = "browser",loadWorkspace,session,artifact,artifacts,content,models,workspace,token,authLoading,loadAttempt,setContent,setModels,setWorkspace,setError,setPending,setSavedChecks,setSelectedArtifactID,reload,writeURL } = c;
 const params = useSearchParams();
 const attachedWorkspace = session?.workspace_id;
 const scopeRef = useRef({ actor: actorScope, session: session?.id, workspace });
 if (scopeRef.current.actor !== actorScope || scopeRef.current.session !== session?.id || scopeRef.current.workspace !== workspace)
   scopeRef.current = { actor: actorScope, session: session?.id, workspace };

  const [saveOpen, setSaveOpen] = useState(false);
  const [saveAccess, setSaveAccess] = useState<"loading" | "signed_out" | "ready" | "error">("loading");
  const [saveBaseline, setSaveBaseline] = useState<string>();
  const saveResumed = useRef(false);
  const [saveTarget, setSaveTarget] = useState<Operation>();
  const [savedCheck, setSavedCheck] = useState<SavedCheck>();
  const [workspaceRows, setWorkspaceRows] = useState<{ owner: string; items: { id: string; name: string }[] }>({ owner: actorScope, items: [] });
  const workspaces = workspaceRows.owner === actorScope ? workspaceRows.items : [];
  const setWorkspaces = (items: { id: string; name: string }[]) => setWorkspaceRows({ owner: actorScope, items });
  async function openSave(operation?: Operation) {
    const selected =
      operation &&
      artifacts.find(
        (a) =>
          a.id ===
          (operation.source?.artifact_id || operation.results[0]?.version),
      );
    if (selected) setSelectedArtifactID(selected.id);
    setSaveBaseline(operation?.id);
    setSaveTarget(selected?.kind === "test_suite" ? undefined : operation);
    setSavedCheck(undefined);
    setError("");
    setSaveOpen(true);
  }
  useEffect(() => {
    if ((!saveOpen && !loadWorkspace) || authLoading) return;
    let current = true;
    setSaveAccess("loading");
    void token().then(async auth => {
      if (!current) return;
      if (!auth) { setWorkspaces([]); setSaveAccess("signed_out"); return; }
      try {
        const me = await createApiClient(auth).get<UserMeResponse>("/v1/users/me");
        if (!current) return;
        const available = me.organizations.flatMap(o => o.workspaces.filter(w =>
          (["workspace_admin", "workspace_member"].includes(w.role) || o.role === "org_admin") && (!saveOpen || !attachedWorkspace || w.id === attachedWorkspace)));
        setWorkspaces(available);
        setWorkspace(old => (saveOpen && attachedWorkspace) || (available.some(w => w.id === old) ? old : (saveOpen ? available[0]?.id : "") || ""));
        setSaveAccess("ready");
      } catch (e) {
        if (!current) return;
        const expired = (e as { status?: number }).status === 401;
        setSaveAccess(expired ? "signed_out" : "error");
        setError(expired ? "Sign in again to keep your work. Your tests are still here." : "Couldn’t load your workspaces. Try again.");
      }
    }).catch(error => {
      if (!current) return;
      setWorkspaces([]); setSaveAccess("signed_out");
      setError(error instanceof Error ? error.message : "Sign in again to load workspaces.");
    });
    return () => { current = false; };
  }, [saveOpen, token, attachedWorkspace, loadAttempt, authLoading, loadWorkspace, actorScope]);
  useEffect(() => {
    if (!session || saveResumed.current) return;
    if (params.get("keep") !== "1") {
      // Browser Back from a canceled login returns to the original URL. Restore
      // the same draft and selection without reopening or submitting the save.
      try {
        const draft = JSON.parse(sessionStorage.getItem(`vibe-keep:${session.id}`) || "null");
        if (draft?.version === 1 && (!params.get("agent") || params.get("agent") === draft.artifact) && session.document.artifacts.some(a => a.id === draft.artifact)) {
          saveResumed.current = true;
          setSelectedArtifactID(draft.artifact);
          if (typeof draft.content === "string") setContent(draft.content);
          if ([draft.models?.assistant, draft.models?.target, draft.models?.evaluator].every(v => typeof v === "string")) setModels(draft.models);
        }
      } catch { /* A canceled login never discards durable tests. */ }
      return;
    }
    saveResumed.current = true;
    const exact = session.document.artifacts.find(a => a.id === params.get("agent"));
    const baselineID = params.get("keep_run") || undefined;
    const baseline = session.operations.find(o => o.id === baselineID);
    if (!exact || (baselineID && (!baseline || !terminal(baseline.state) ||
      (baseline.source?.artifact_id || baseline.results[0]?.version) !== exact.id))) {
      setError("This saved selection is unavailable. Choose the tests you want to keep."); return;
    }
    setSelectedArtifactID(exact.id);
    setSaveBaseline(baselineID);
    setSaveTarget(exact.kind === "test_suite" ? undefined : baseline);
    try {
      const raw = sessionStorage.getItem(`vibe-keep:${session.id}`);
      if (raw) {
        const draft = JSON.parse(raw);
        if (draft.version === 1 && draft.artifact === exact.id && typeof draft.content === "string") setContent(draft.content);
        if (draft.version === 1 && draft.artifact === exact.id && [draft.models?.assistant, draft.models?.target, draft.models?.evaluator].every(v => typeof v === "string")) setModels(draft.models);
      }
    } catch { /* Storage may be blocked; the durable tests remain accessible. */ }
    setSaveOpen(true);
  }, [session, params]);
  function closeSave(open: boolean) {
    setSaveOpen(open);
    if (!open) {
      const url = new URL(window.location.href);
      url.searchParams.delete("keep"); url.searchParams.delete("keep_run");
      writeURL(url);
      if (session) { try { sessionStorage.removeItem(`vibe-keep:${session.id}`); } catch {} }
    }
  }
  function rememberSave() {
    if (session && artifact) {
      try { sessionStorage.setItem(`vibe-keep:${session.id}`, JSON.stringify({ version: 1, artifact: artifact.id, content, models })); } catch {}
    }
  }

  async function save() {
    if (!session || !artifact || !workspace) return;
    const scope = scopeRef.current;
    const current = () => scopeRef.current === scope;
    setPending(true);
    setError("");
    try {
      const auth = await token();
      if (!current()) return;
      if (!auth) { setSaveAccess("signed_out"); return; }
      await vibeFetch(`/sessions/${session.id}/claim`, auth, {
        method: "POST",
        body: "{}",
      });
      if (!current()) return;
      const latest = await reload();
      if (!current() || !latest) return;
      if (artifact.kind === "test_plan") {
        const receipt = await vibeFetch<SavedCheck>(`/sessions/${session.id}/save-brief`, auth, {
          method: "POST", body: JSON.stringify({ revision: latest.revision, artifact_id: artifact.id, workspace_id: workspace }),
        });
        if (!current()) return;
        setSavedCheck(receipt);
        setSavedChecks(old => [receipt, ...old.filter(c => c.id !== receipt.id)]);
        await reload();
        return;
      }
      if (saveTarget) {
        const receipt = await vibeFetch<SavedCheck>(
          `/sessions/${session.id}/save-check`,
          auth,
          {
            method: "POST",
            body: JSON.stringify({
              revision: latest.revision,
              workspace_id: workspace,
              baseline_operation_id: saveTarget.id,
            }),
          },
        );
        if (!current()) return;
        setSavedCheck(receipt);
        setSavedChecks((old) => [
          receipt,
          ...old.filter((c) => c.id !== receipt.id),
        ]);
        await reload();
        return;
      }
      await vibeFetch<{
        draft_id: string;
        workspace_id: string;
      }>(`/sessions/${session.id}/save`, auth, {
        method: "POST",
        body: JSON.stringify({
          revision: latest.revision,
          artifact_id: artifact.id,
          approve_artifact: true,
          workspace_id: workspace,
          baseline_operation_id: saveBaseline,
          models,
        }),
      });
      if (!current()) return;
      await reload();
      if (!current()) return;
      const kept = await vibeFetch<SavedCheck[]>(`/saved-checks?workspace=${encodeURIComponent(workspace)}`, auth);
      if (!current()) return;
      setSavedChecks(Array.isArray(kept) ? kept : []);
    } catch (e) {
      if (!current()) return;
      if (e instanceof VibeError && e.status === 401) setSaveAccess("signed_out");
      if (e instanceof VibeError && e.status === 403) {
        setWorkspaces([]); setSaveAccess("ready");
        setError("Your workspace access changed. Ask its owner for permission to save here.");
      } else setError((e as Error).message);
    } finally {
      if (current()) setPending(false);
    }
  }

 return { saveOpen,saveAccess,saveBaseline,saveTarget,savedCheck,workspaces,openSave,closeSave,rememberSave,save };
}
