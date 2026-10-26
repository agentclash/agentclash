"use client";
import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from "react";
import { useSearchParams } from "next/navigation";
import {pinVibeReturn} from "./vibe-keep";
import { createApiClient } from "@/lib/api/client";
import type { UserMeResponse } from "@/lib/api/types";
import {
  vibeFetch,
  VibeError,
  terminal,
  type Session,
  type Artifact,
  type Operation,
  type SavedCheck,
  type Models,
} from "@/lib/vibe";
type Context = {
  session: Session | null;
  artifact?: Artifact;
  artifacts: Artifact[];
  content: string;
  models: Models;
  workspace: string;
  actorScope?: string;
  savedChecks: SavedCheck[];
  loadWorkspace?: boolean;
  token: () => Promise<string | undefined>;
  authLoading: boolean;
  loadAttempt: number;
  setContent: Dispatch<SetStateAction<string>>;
  setModels: Dispatch<SetStateAction<Models>>;
  setWorkspace: Dispatch<SetStateAction<string>>;
  setError: Dispatch<SetStateAction<string>>;
  setPending: Dispatch<SetStateAction<boolean>>;
  setSavedChecks: Dispatch<SetStateAction<SavedCheck[]>>;
  setSelectedArtifactID: (id: string | null) => void;
  reload: () => Promise<Session | undefined>;
  writeURL: (url: URL) => void;
};
export function useVibeSaving(c: Context) {
  const {
    actorScope = "browser",
    loadWorkspace,
    savedChecks,
    session,
    artifact: currentArtifact,
    artifacts,
    content,
    models: currentModels,
    workspace,
    token,
    authLoading,
    loadAttempt,
    setContent,
    setModels,
    setWorkspace,
    setError,
    setPending,
    setSavedChecks,
    setSelectedArtifactID,
    reload,
    writeURL,
  } = c;
  const params = useSearchParams();
  const attachedWorkspace = session?.workspace_id;
  const scopeRef = useRef({ actor: actorScope, session: session?.id, workspace });
  if (
    scopeRef.current.actor !== actorScope ||
    scopeRef.current.session !== session?.id ||
    scopeRef.current.workspace !== workspace
  )
    scopeRef.current = { actor: actorScope, session: session?.id, workspace };

  const [saveOpen, setSaveOpen] = useState(false);
  const [intent, setIntent] = useState<{
    actor: string;
    session: string;
    artifact: Artifact;
    models: Models;
    returnPath: string;
    advanced: boolean;
  }>();
  const selectedIntent =
    intent?.actor === actorScope && intent.session === session?.id ? intent : undefined;
  const artifact = selectedIntent?.artifact || currentArtifact,
    models = selectedIntent?.models || currentModels;
  const [advancedReceipt, setAdvancedReceipt] = useState<{
    draft_id: string;
    workspace_id: string;
  }>();
  const saveGeneration = useRef(0);
  const saveRequest = useRef<{ owner: typeof scopeRef.current; generation: number } | undefined>(undefined);

  const [saveAccess, setSaveAccess] = useState<"loading" | "signed_out" | "ready" | "error">(
    "loading",
  );
  const [saveBaseline, setSaveBaseline] = useState<string>();
  const saveResumed = useRef(false);
  const [saveTarget, setSaveTarget] = useState<Operation>();
  const [savedCheck, setSavedCheck] = useState<SavedCheck>();
  const [workspaceRows, setWorkspaceRows] = useState<{
    owner: string;
    items: { id: string; name: string }[];
  }>({ owner: actorScope, items: [] });
  const workspaces = workspaceRows.owner === actorScope ? workspaceRows.items : [];
  const setWorkspaces = (items: { id: string; name: string }[]) =>
    setWorkspaceRows({ owner: actorScope, items });
  function returnFor(exact:Artifact,operation?:Operation){
    const matches=(run:Operation)=>(run.kind==="check"||run.kind==="retest")&&(run.result_scope?.artifact_id||run.source?.artifact_id||run.results[0]?.version)===exact.id;
    const selectedRun=operation || (params.get("run")?session?.operations.find(run=>run.id===params.get("run")&&matches(run)):session?.operations.findLast(matches));
    return pinVibeReturn(window.location.pathname+window.location.search,session!.id,exact.id,selectedRun?.id);
  }
  async function openSave(operation?: Operation) {
    const selected =
      operation &&
      artifacts.find(
        (a) =>
          a.id ===
          (operation.result_scope?.artifact_id ||
            operation.source?.artifact_id ||
            operation.results[0]?.version),
      );
    const exact = selected || (!operation ? currentArtifact : undefined);
    if (!session || !exact) {
      setError("The chosen version is unavailable. Reload before saving.");
      return;
    }
    saveGeneration.current++;
    setIntent({
      actor: actorScope,
      session: session.id,
      artifact: structuredClone(exact),
      models: { ...currentModels },
      returnPath: returnFor(exact,operation),
      advanced: !!exact.unavailable_reason,
    });
    setAdvancedReceipt(undefined);
    if (selected) setSelectedArtifactID(selected.id);
    setSaveBaseline(operation?.id);
    setSaveTarget(exact.kind === "test_suite" ? undefined : operation);
    setSavedCheck(undefined);
    setError("");
    setSaveOpen(true);
  }
  useEffect(() => {
    if ((!saveOpen && !loadWorkspace) || authLoading) return;
    let current = true;
    setSaveAccess("loading");
    void token()
      .then(async (auth) => {
        if (!current) return;
        if (!auth) {
          setWorkspaces([]);
          setSaveAccess("signed_out");
          return;
        }
        try {
          const me = await createApiClient(auth).get<UserMeResponse>("/v1/users/me");
          if (!current) return;
          const available = me.organizations.flatMap((o) =>
            o.workspaces.filter(
              (w) =>
                (["workspace_admin", "workspace_member"].includes(w.role) ||
                  o.role === "org_admin") &&
                (!saveOpen || !attachedWorkspace || w.id === attachedWorkspace),
            ),
          );
          setWorkspaces(available);
          setWorkspace(
            (old) =>
              (saveOpen && attachedWorkspace) ||
              (available.some((w) => w.id === old)
                ? old
                : (saveOpen ? available[0]?.id : "") || ""),
          );
          setSaveAccess("ready");
        } catch (e) {
          if (!current) return;
          const expired = (e as { status?: number }).status === 401;
          setSaveAccess(expired ? "signed_out" : "error");
          setError(
            expired
              ? "Sign in again to keep your work. Your tests are still here."
              : "Couldn’t load your workspaces. Try again.",
          );
        }
      })
      .catch((error) => {
        if (!current) return;
        setWorkspaces([]);
        setSaveAccess("signed_out");
        setError(error instanceof Error ? error.message : "Sign in again to load workspaces.");
      });
    return () => {
      current = false;
    };
  }, [saveOpen, token, attachedWorkspace, loadAttempt, authLoading, loadWorkspace, actorScope]);
  useEffect(() => {
    if (!session || saveResumed.current) return;
    if (params.get("keep") !== "1") {
      // Browser Back from a canceled login returns to the original URL. Restore
      // the same draft and selection without reopening or submitting the save.
      try {
        const draft = JSON.parse(sessionStorage.getItem(`vibe-keep:${session.id}`) || "null");
        if (
          draft?.version === 1 &&
          (!params.get("agent") || params.get("agent") === draft.artifact) &&
          session.document.artifacts.some((a) => a.id === draft.artifact)
        ) {
          saveResumed.current = true;
          setSelectedArtifactID(draft.artifact);
          if (typeof draft.content === "string") setContent(draft.content);
          if (
            [draft.models?.assistant, draft.models?.target, draft.models?.evaluator].every(
              (v) => typeof v === "string",
            )
          )
            setModels(draft.models);
        }
      } catch {
        /* A canceled login never discards durable tests. */
      }
      return;
    }
    saveResumed.current = true;
    const exact = session.document.artifacts.find((a) => a.id === params.get("agent"));
    const baselineID = params.get("keep_run") || undefined;
    const baseline = session.operations.find((o) => o.id === baselineID);
    if (
      !exact ||
      (baselineID &&
        (!baseline ||
          !terminal(baseline.state) ||
          (baseline.result_scope?.artifact_id ||
            baseline.source?.artifact_id ||
            baseline.results[0]?.version) !== exact.id))
    ) {
      setError("This saved selection is unavailable. Choose the tests you want to keep.");
      return;
    }
    setIntent({
      actor: actorScope,
      session: session.id,
      artifact: structuredClone(exact),
      models: { ...currentModels },
      returnPath: returnFor(exact,baseline),
      advanced: params.get("keep_advanced") === "1" || !!exact.unavailable_reason,
    });
    setSelectedArtifactID(exact.id);
    setSaveBaseline(baselineID);
    setSaveTarget(exact.kind === "test_suite" ? undefined : baseline);
    try {
      const raw = sessionStorage.getItem(`vibe-keep:${session.id}`);
      if (raw) {
        const draft = JSON.parse(raw);
        if (draft.version === 1 && draft.artifact === exact.id && typeof draft.content === "string")
          setContent(draft.content);
        if (
          draft.version === 1 &&
          draft.artifact === exact.id &&
          [draft.models?.assistant, draft.models?.target, draft.models?.evaluator].every(
            (v) => typeof v === "string",
          )
        ) {
          setModels(draft.models);
          setIntent((old) => (old ? { ...old, models: { ...draft.models } } : old));
        }
      }
    } catch {
      /* Storage may be blocked; the durable tests remain accessible. */
    }
    setSaveOpen(true);
  }, [session, params]);
  function closeSave(open: boolean) {
    if (!open) {
      if (saveRequest.current?.owner === scopeRef.current) setPending(false);
      saveRequest.current = undefined;
      saveGeneration.current++;
    }
    setSaveOpen(open);
    if (!open) {
      const url = new URL(window.location.href);
      url.searchParams.delete("keep");
      url.searchParams.delete("keep_run");
      url.searchParams.delete("keep_advanced");
      writeURL(url);
      if (session) {
        try {
          sessionStorage.removeItem(`vibe-keep:${session.id}`);
        } catch {}
      }
    }
  }
  function rememberSave() {
    if (session && artifact) {
      try {
        sessionStorage.setItem(
          `vibe-keep:${session.id}`,
          JSON.stringify({ version: 1, artifact: artifact.id, content, models }),
        );
      } catch {}
    }
  }

  async function save() {
    if (!session || !selectedIntent || !artifact || !workspace) return;
    const scope = scopeRef.current;
    if (saveRequest.current?.owner === scope) return;
    const generation = saveGeneration.current;
    const request = { owner: scope, generation };
    saveRequest.current = request;
    const current = () => scopeRef.current === scope && generation === saveGeneration.current;
    setPending(true);
    setError("");
    try {
      const auth = await token();
      if (!current()) return;
      if (!auth) {
        setSaveAccess("signed_out");
        return;
      }
      await vibeFetch(`/sessions/${session.id}/claim`, auth, {
        method: "POST",
        body: "{}",
      });
      if (!current()) return;
      const latest = await reload();
      if (!current() || !latest) return;
      if (selectedIntent.advanced) {
        const receipt = await vibeFetch<{ draft_id: string; workspace_id: string }>(
          `/sessions/${session.id}/advanced`,
          auth,
          {
            method: "POST",
            body: JSON.stringify({
              revision: latest.revision,
              artifact_id: artifact.id,
              workspace_id: workspace,
            }),
          },
        );
        if (!current()) return;
        setAdvancedReceipt(receipt);
        await reload();
        return;
      }
      if (artifact.kind === "test_plan") {
        const receipt = await vibeFetch<SavedCheck>(`/sessions/${session.id}/save-brief`, auth, {
          method: "POST",
          body: JSON.stringify({
            revision: latest.revision,
            artifact_id: artifact.id,
            workspace_id: workspace,
          }),
        });
        if (!current()) return;
        setSavedCheck(receipt);
        setSavedChecks((old) => [receipt, ...old.filter((c) => c.id !== receipt.id)]);
        await reload();
        return;
      }
      if (saveTarget) {
        const receipt = await vibeFetch<SavedCheck>(`/sessions/${session.id}/save-check`, auth, {
          method: "POST",
          body: JSON.stringify({
            revision: latest.revision,
            workspace_id: workspace,
            baseline_operation_id: saveTarget.id,
          }),
        });
        if (!current()) return;
        setSavedCheck(receipt);
        setSavedChecks((old) => [receipt, ...old.filter((c) => c.id !== receipt.id)]);
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
      const kept = await vibeFetch<SavedCheck[]>(
        `/saved-checks?workspace=${encodeURIComponent(workspace)}`,
        auth,
      );
      if (!current()) return;
      setSavedChecks(Array.isArray(kept) ? kept : []);
    } catch (e) {
      if (!current()) return;
      if (e instanceof VibeError && e.status === 401) setSaveAccess("signed_out");
      if (e instanceof VibeError && e.status === 403) {
        setWorkspaces([]);
        setSaveAccess("ready");
        setError("Your workspace access changed. Ask its owner for permission to save here.");
      } else setError((e as Error).message);
    } finally {
      if (current()) setPending(false);
      if (saveRequest.current === request) saveRequest.current = undefined;
    }
  }

  const saveKept = savedChecks.find(
    (check) =>
      check.session_id === session?.id && check.artifact_id === artifact?.id && check.draft_id,
  );
  const sourceModels =
    saveKept?.models ||
    (session && artifact && session.saved_artifact_id === artifact.id
      ? session.saved_models
      : undefined);
  const savedArtifact = session?.document.artifacts.find((item) => item.id === artifact?.id);
  const saveDraft =
    savedArtifact?.accepted && saveKept?.draft_id
      ? { draft_id: saveKept.draft_id, workspace_id: saveKept.workspace_id }
      : savedArtifact?.accepted &&
          session &&
          artifact &&
          session.saved_artifact_id === artifact.id &&
          session.saved_draft_id &&
          session.workspace_id
        ? { draft_id: session.saved_draft_id, workspace_id: session.workspace_id }
        : null;
  const saveComplete =
    !!saveDraft &&
    sourceModels?.assistant === models.assistant &&
    sourceModels.target === models.target &&
    sourceModels.evaluator === models.evaluator;
  const saveNotice = sourceModels
    ? "Your current model choices differ from those recorded when this agent was saved."
    : "Saved model choices are unknown. Your current model choices are not confirmed as saved.";
  return {
    saveDraft,
    saveComplete,
    saveNotice,
    saveKept,
    saveArtifact: artifact,
    saveReturnPath: selectedIntent?.returnPath,
    saveAdvanced: !!selectedIntent?.advanced,
    advancedReceipt,
    saveOpen: saveOpen && !!selectedIntent,
    saveAccess,
    saveBaseline,
    saveTarget,
    savedCheck,
    workspaces,
    openSave,
    closeSave,
    rememberSave,
    save,
  };
}
