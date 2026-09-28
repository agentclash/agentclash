"use client";

import { downloadJSON, exportRuns } from "@/lib/vibe-export";

import { readBuildDrafts, writeBuildDrafts } from "@/lib/vibe-build-drafts";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent } from "@/lib/vibe-build-analytics";

import type { ConversationAction } from "@/lib/vibe-conversation";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useAccessToken, useAuth } from "@workos-inc/authkit-nextjs/components";
import { useCallback, useEffect, useRef, useState } from "react";
import { ArchivedConversation } from "@/components/vibe/archived-conversation";
import { PrototypeTrial } from "@/components/vibe/prototype-trial";
import { EvaluationWorkspace } from "@/components/vibe/evaluation-workspace";
import { buildVersion, workingBuildArtifact } from "@/lib/vibe-build-timeline";
import { EvaluationNavigation, evaluationIdentity } from "@/components/vibe/evaluation-navigation";
import { VibeButton } from "@/components/vibe/vibe-button";
import { CreditsDialog } from "@/components/vibe/credits-dialog";
import { AgentSettings } from "@/components/vibe/agent-settings";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { createApiClient } from "@/lib/api/client";
import type { UserMeResponse } from "@/lib/api/types";
import {
  editableEvaluation,
  defaultModels,
  terminal,
  retryVibeOperation,
  VibeError,
  vibeFetch,
  watchVibe,
  type CaseResult,
  type BuildQuote,
  type RunQuote,
  dollars,
  type Models,
  type Operation,
  type Session,
  type VibeConfig,
  type SavedCheck,
} from "@/lib/vibe";

import { keepReturnURL, savedWorkURL } from "@/lib/vibe-keep";

// POST /messages faults from api/vibe.go, Service.Prepare and Store.Submit:
// these exact code/status pairs reject before admission or roll back its transaction.
// Unknown responses (including generic 503s) cannot prove non-admission.
const submissionRejections: Partial<Record<number, readonly string[]>> = {
  400: [
    "quote_expired",
    "unsupported_capability",
    "invalid_request",
    "invalid_message",
    "invalid_operation",
    "invalid_evidence",
    "invalid_evaluation",
    "evidence_roles_required",
    "invalid_import",
    "import_limit",
    "unsupported_schema",
    "invalid_encoding",
    "free_model_required",
    "unsupported_model",
    "evaluator_pinned",
    "artifact_required",
    "agent_required",
    "preview_changed",
    "preview_consent_required",
    "context_limit",
    "baseline_required",
    "comparison_changed",
    "case_limit",
    "graph_limit",
    "budget_limit",
    "conversation_limit",
    "workspace_required",
  ],
  401: ["unauthenticated"],
  402: ["insufficient_credits"],
  403: ["forbidden"],
  404: ["not_found"],
  409: ["revision_conflict", "operation_running", "invalid_state"],
  413: ["request_too_large"],
  429: ["rate_limit", "capacity_limit", "trial_limit"],
  503: [
    "hosted_disabled",
    "pricing_unavailable",
    "accounting_unavailable",
    "trial_capacity_reached",
  ],
};
const retryRejections: Partial<Record<number, readonly string[]>> = {
  429: ["retry_cooldown"],
  400: ["retry_manual_edit"],
  409: ["idempotency_conflict", "retry_not_allowed", "retry_committed", "retry_running", "retry_uncertain", "retry_stale"],
};
const sessionAccessLost =
  "This browser can’t access the saved session. Your unsent message is still here.";
function isSessionAccessError(error: unknown) {
  return (
    error instanceof VibeError && [401, 403, 404].includes(error.status || 0)
  );
}

export function VibeClient() {
  const params = useSearchParams();
  const requestedSessionID = params.get("session");
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [loadingSession, setLoadingSession] = useState(!!requestedSessionID);
  const { getAccessToken } = useAccessToken();
  const { loading: authLoading, user: authUser } = useAuth();
  const authUserID = authUser?.id;
  const [session, setSession] = useState<Session | null>(null);
  const activeSessionRef = useRef<string | undefined>(requestedSessionID || undefined);
  const contextDrafts = useRef<Record<string, string>>({});
  const contextTrials = useRef<Record<string, { artifact?: string; thread: string; target: string; trying: boolean }>>({});
  const contextChoice = useRef<{ door: "build" | "test"; id: string } | undefined>(undefined);
  // Choosing the wrong door is reversible until a context has any saved work.
  // The server intentionally keeps context creation append-only; hide the empty
  // abandoned child locally instead of deleting history or changing its API.
  const entryOrigin = useRef<string | undefined>(undefined);
  const discardedContextIDs = useRef(new Set<string>());
  const [contexts, setContexts] = useState<Session[]>([]);
  const [newEvaluation, setNewEvaluation] = useState(false);
  const [savedWorkOpen, setSavedWorkOpen] = useState(false);
  const [buildQuote, setBuildQuote] = useState<BuildQuote>();
  const [quoteError, setQuoteError] = useState("");
  const [runQuote, setRunQuote] = useState<{ quote: RunQuote; kind: string; baseline?: Operation; extra: { client_id?: string; cycle_id?: string; additional_examples?: number; evidence_set_id?: string; artifact_id?: string; purpose?: "regrade" }; content?: string; sessionID: string; revision: number }>();
  const [config, setConfig] = useState<VibeConfig | null>(null);
  const [configError, setConfigError] = useState(false);
  // Loading configuration must not render the legacy entry before the V1 shell.
  const twoDoor = config === null ? true : config.two_door;
  const [models, setModels] = useState<Models>(defaultModels);
  const [content, setContent] = useState("");
  const [pending, setPending] = useState(false);
  const [pendingMessage, setPendingMessage] = useState<{
    id: string;
    content: string;
  }>();
  const [runPending, setRunPending] = useState(false);
  const [requestedRunID, setRequestedRunID] = useState<string>();
  const navigatedRunID = useRef<string | undefined>(undefined);
  const [pendingAction, setPendingAction] = useState<string>();
  const [error, setError] = useState("");
  const [connection, setConnection] = useState("");
  const [view, setView] = useState<"build" | "try" | "checks">(
    params.get("view") === "try"
      ? "try"
      : params.get("view") === "checks"
        ? "checks"
        : "build",
  );
  const [selectedArtifactID, setSelectedArtifactID] = useState<string | null>(
    params.get("agent"),
  );
  const [threadID, setThreadID] = useState(params.get("thread") || "");
  const buildEvidence = useRef<
    { operation: string; artifact: string } | undefined
  >(undefined);
  const savedCaseReads = useRef(new Map<string, Promise<CaseResult>>());
  const restoredThread = useRef(params.get("thread"));
  const [trialBuffers, setTrialBuffers] = useState<Record<string, string>>({});
  const trialEdits = useRef<Record<string, number>>({});
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [checksDirtyID, setChecksDirtyID] = useState<string | null>(null);
  const [pendingEdit, setPendingEdit] = useState<{ operationID: string; artifactID: string }>();
  const [dirtyArtifactID, setDirtyArtifactID] = useState<string | null>(null);
  const [saveOpen, setSaveOpen] = useState(false);
  const [saveAccess, setSaveAccess] = useState<"loading" | "signed_out" | "ready" | "error">("loading");
  const [saveBaseline, setSaveBaseline] = useState<string>();
  const saveResumed = useRef(false);
  const [saveTarget, setSaveTarget] = useState<Operation>();
  const [savedCheck, setSavedCheck] = useState<SavedCheck>();
  const [savedChecks, setSavedChecks] = useState<SavedCheck[]>([]);
  const [workspaces, setWorkspaces] = useState<{ id: string; name: string }[]>(
    [],
  );
  const [workspace, setWorkspace] = useState(params.get("workspace") || "");

  const sending = useRef(false);
  const currentView = useRef(view);
  const submission = useRef<{
    sessionID: string;
    body: string;
    composer: string | null;
    composerVersion: number;
    trialKey?: string;
    retryOperationID?: string;
    uncertain: boolean;
  } | null>(null);
  const composerEdits = useRef(0);
  const retryUnsentMessage = useRef<(() => void) | undefined>(undefined);
  const [uncertain, setUncertain] = useState(false);
  const file = useRef<HTMLInputElement>(null);
  const active = session?.operations.find((o) => !terminal(o.state));
  const sessionUnavailable =
    !!requestedSessionID && session?.id !== requestedSessionID;
  const busy =
    !config ||
    pending ||
    !!active ||
    uncertain ||
    sessionUnavailable ||
    connection === sessionAccessLost;
  const journey = session?.document.journey;
  const testJourney = session?.document.evaluation?.door === "test" && !session.document.test_journey ? false :
    !!session?.document.test_journey ||
    (!session?.document.evaluation_first &&
      !session?.document.messages.length &&
      !session?.document.artifacts.length &&
      !session?.operations.length);
  const artifacts =
    session?.document.artifacts.filter(
      (a) =>
        journey?.mode !== "existing" ||
        a.kind === "test_suite" ||
        journey.preview_consent ||
        a.kind === "conversation_evaluation" ||
        a.kind === "test_plan",
    ) || [];
  const latestArtifact = artifacts.at(-1);
  const workingArtifact = session?.document.evaluation?.door === "build" ? workingBuildArtifact(session) : undefined;
  const artifact = selectedArtifactID ? artifacts.find((a) => a.id === selectedArtifactID) : session?.document.evaluation?.door === "build" ? workingArtifact : latestArtifact;
  const dirtyArtifact =
    !!artifact &&
    (dirtyArtifactID === artifact.id || checksDirtyID === artifact.id);
  const evaluation = artifact ? editableEvaluation(artifact.blueprint) : null;
  const scenarioCount =
    artifact?.test_plan?.scenarios.length ||
    evaluation?.scenarios?.length ||
    evaluation?.examples.length;
  const buildJourney = session?.document.evaluation?.door === "build";
  const trialPrefix = `${session?.id || "new"}:${artifact?.id || ""}:`;
  const emptyTrialKey = `${trialPrefix}new:${models.target}`;
  const trialDraftKey = threadID ? `${trialPrefix}${threadID}` : emptyTrialKey;
  const trialText = trialBuffers[trialDraftKey] || "";
  const trialHistory = [
    ...new Map(
      (session?.document.messages || [])
        .filter(
          (m) => m.origin === "playground" && m.artifact_id === artifact?.id,
        )
        .map((m) => {
          const operation = session?.operations.find(
            (o) => o.id === m.operation_id,
          );
          const id = m.preview_thread_id || `legacy:${m.operation_id}`;
          return [
            id,
            {
              id,
              model: operation?.models.target,
              legacy: !m.preview_thread_id,
            },
          ];
        }),
    ).values(),
  ];
  const legacyTrial = threadID.startsWith("legacy:");
  const trialMessages =
    session?.document.messages.filter(
      (m) =>
        m.origin === "playground" &&
        m.artifact_id === artifact?.id &&
        (m.preview_thread_id === threadID ||
          (legacyTrial && `legacy:${m.operation_id}` === threadID)),
    ) || [];
  function navigate(
    next: typeof view,
    agent = artifact?.id,
    thread = threadID,
  ) {
    setView(next);
    currentView.current = next;
    if (agent && !(buildJourney && next === "checks")) setSelectedArtifactID(agent);
    const url = new URL(window.location.href);
    url.searchParams.set("view", next);
    if (agent && !(buildJourney && next === "checks")) url.searchParams.set("agent", agent);
    else url.searchParams.delete("agent");
    if (thread) url.searchParams.set("thread", thread);
    else url.searchParams.delete("thread");
    window.history.replaceState(null, "", url.pathname + url.search);
  }
  function newTrial() {
    setThreadID("");
    setTrialBuffers((old) => ({ ...old, [emptyTrialKey]: "" }));
    navigate(buildJourney && view !== "try" ? "build" : "try", artifact?.id, "");
  }
  function changeModels(next: Models) {
    if (next.target !== models.target) {
      setThreadID("");
      navigate(view, artifact?.id, "");
    }
    setModels(next);
  }
  const keptArtifact = savedChecks.find(c => c.session_id === session?.id && c.artifact_id === artifact?.id && c.draft_id);
  const savedModels = keptArtifact?.models || (session?.saved_artifact_id === artifact?.id ? session?.saved_models : undefined);
  // Canonical identity survives unknown legacy receipts or changed selections;
  // only the immutable model receipt can confirm that these choices were saved.
  const savedDraft = artifact?.accepted && !dirtyArtifact && keptArtifact?.draft_id ? { draft_id: keptArtifact.draft_id, workspace_id: keptArtifact.workspace_id } :
    artifact?.accepted &&
    !dirtyArtifact &&
    session?.saved_artifact_id === artifact.id &&
    session.saved_draft_id &&
    session.workspace_id
      ? { draft_id: session.saved_draft_id, workspace_id: session.workspace_id }
      : null;
  const saved =
    !!savedDraft &&
    savedModels?.assistant === models.assistant &&
    savedModels.target === models.target &&
    savedModels.evaluator === models.evaluator;
  const savedModelNotice = savedModels
    ? "Your current model choices differ from those recorded when this agent was saved."
    : "Saved model choices are unknown. Your current model choices are not confirmed as saved.";
  const sessionID = session?.id;
  useEffect(() => {
    if (sessionID && buildJourney && !loadingSession && activeSessionRef.current === sessionID)
      writeBuildDrafts(sessionID, content, trialBuffers);
  }, [sessionID, buildJourney, loadingSession, content, trialBuffers]);
  const attachedWorkspace = session?.workspace_id;
  useEffect(() => {
    if (attachedWorkspace) setWorkspace(attachedWorkspace);
  }, [attachedWorkspace]);
  const token = useCallback(async () => {
    if (authLoading || !authUserID) return undefined;
    try {
      return await getAccessToken();
    } catch {
      return undefined;
    }
  }, [getAccessToken, authLoading, authUserID]);
  useEffect(() => {
    let current = true;
    void token()
      .then(async (auth) => {
        if (!auth) return;
        const items = await vibeFetch<SavedCheck[]>(
          `/saved-checks${workspace ? `?workspace=${workspace}` : ""}`,
          auth,
        );
        if (current) setSavedChecks(Array.isArray(items) ? items : []);
      })
      .catch(() => undefined);
    return () => {
      current = false;
    };
  }, [token, workspace]);

  useEffect(() => {
    let alive = true;
    setConfigError(false);
    vibeFetch<VibeConfig>("/config")
      .then((c) => {
        if (alive) {
          setConfig(c);
          if (!requestedSessionID) setModels(c.defaults);
        }
      })
      .catch(() => {
        if (alive) setConfigError(true);
      });
    return () => {
      alive = false;
    };
  }, [requestedSessionID, loadAttempt]);
  useEffect(() => {
    const id = requestedSessionID;
    if (!id || authLoading) return;
    let alive = true;
    setLoadingSession(true);
    void (async () => {
      const auth = await token();
      try {
        let v: Session;
        try {
          v = await vibeFetch<Session>(`/sessions/${id}`, auth);
        } catch (e) {
          if (!auth || !(e instanceof VibeError) || e.code !== "not_found")
            throw e;
          v = await vibeFetch<Session>(`/sessions/${id}/claim`, auth, {
            method: "POST",
            body: "{}",
          });
        }
        if (alive) {
          activeSessionRef.current = v.id;
          setSession(v);
          if (v.document.evaluation?.door === "build") {
            const draft = readBuildDrafts(v.id);
            if (draft) {
              if (composerEdits.current === 0) setContent(draft.guide);
              setTrialBuffers(old => ({ ...draft.trials, ...old }));
            }
          }
          const message = v.document.messages.find(
            (m) => m.preview_thread_id === restoredThread.current,
          );
          const trial =
            message && v.operations.find((o) => o.id === message.operation_id);
          setModels(
            trial
              ? { ...v.document.models, target: trial.models.target }
              : v.document.models,
          );
        }
      } catch (e) {
        if (alive) {
          if (isSessionAccessError(e)) {
            setError("");
            setConnection(sessionAccessLost);
          } else setError((e as Error).message);
        }
      } finally {
        if (alive) setLoadingSession(false);
      }
    })();
    return () => {
      alive = false;
    };
  }, [requestedSessionID, token, loadAttempt, authLoading]);
  useEffect(() => {
    if (!sessionID || authLoading) return;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    let claimAttempted = false;
    const connect = async () => {
      const auth = await token();
      try {
        await watchVibe(sessionID, auth, controller.signal, (v) => {
          if (controller.signal.aborted || activeSessionRef.current !== v.id) return;
          setSession((old) =>
            old &&
            old.id === v.id &&
            ((old.event_cursor || 0) > (v.event_cursor || 0) ||
              old.revision > v.revision)
              ? old
              : v,
          );
          setConnection("");
        });
      } catch (e) {
        if (!controller.signal.aborted) {
          // AuthKit may finish loading after the first anonymous snapshot. The
          // signed-in stream then needs the same ownership claim as a reload.
          if (auth && e instanceof VibeError && e.code === "not_found" && !claimAttempted) {
            claimAttempted = true;
            try {
              const claimed = await vibeFetch<Session>(`/sessions/${sessionID}/claim`, auth, { method: "POST", body: "{}" });
              if (!controller.signal.aborted && activeSessionRef.current === claimed.id) {
                setSession(old => old && old.id === claimed.id && (old.revision > claimed.revision || (old.event_cursor || 0) > (claimed.event_cursor || 0)) ? old : claimed);
                setConnection("");
                timer = setTimeout(connect, 0);
              }
              return;
            } catch { /* A different owner's session must remain inaccessible. */ }
          }
          if (isSessionAccessError(e)) {
            setConnection(sessionAccessLost);
            return;
          }
          setConnection("Reconnecting to saved progress…");
        }
      }
      if (!controller.signal.aborted) timer = setTimeout(connect, 2000);
    };
    void connect();
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [sessionID, token, loadAttempt, authLoading]);
  const reload = async (id = sessionID) => {
    if (!id) return;
    const v = await vibeFetch<Session>(`/sessions/${id}`, await token());
    if (activeSessionRef.current !== id) return v;
    setSession((old) =>
      old && old.id === v.id && (old.event_cursor || 0) > (v.event_cursor || 0)
        ? old
        : v,
    );
    return v;
  };
  async function ensureSession() {
    if (requestedSessionID && session?.id !== requestedSessionID)
      throw new Error(
        "Load this conversation before sending. No new conversation was created.",
      );
    if (session) return session;
    const id = crypto.randomUUID();
    const auth = await token();
    await vibeFetch<Session>("/sessions", auth, {
      method: "POST",
      body: JSON.stringify({
        id,
        ...(workspace ? { workspace_id: workspace } : {}),
      }),
    });
    // Verify the browser kept the private cookie before adopting the URL or
    // sending a message. A 201 alone does not prove subsequent ownership.
    let v: Session;
    try {
      v = await vibeFetch<Session>(`/sessions/${id}`, auth);
    } catch (e) {
      if (isSessionAccessError(e))
        throw new Error(
          "The browser could not keep its private session. Your message has not been sent and is still here. Check that cookies are allowed, then try again.",
        );
      throw e;
    }
    activeSessionRef.current = v.id;
    setSession(v);
    window.history.replaceState(
      null,
      "",
      `/vibe-evals?session=${id}${workspace ? `&workspace=${workspace}` : ""}`,
    );
    return v;
  }
  async function exportConversation() {
    if (!session || pending) return;
    const snapshot = session;
    setPending(true);
    setError("");
    try {
      const auth = await token();
      const operations = await exportRuns(snapshot, (id, key) => vibeFetch<CaseResult>(`/operations/${id}/case?key=${encodeURIComponent(key)}`, auth));
      downloadJSON("agentclash-vibe-conversation.json", {format: "agentclash-vibe-conversation-v1", document: snapshot.document, operations, capabilities: config?.capabilities || []});
      if (snapshot.document.evaluation?.door === "build") captureBuildEvent(WEB_EVENTS.VIBE_BUILD_EXPORT_REQUESTED, { session_id: snapshot.id, artifact_id: artifact?.id, sample: !!artifact?.sample, format: "json" });
    } catch (e) { setError((e as Error).message); }
    finally { setPending(false); }
  }
  function loadSavedCase(id: string, key: string): Promise<CaseResult> {
    const owner = session?.id;
    if (!owner || activeSessionRef.current !== owner)
      return Promise.reject(new Error("This conversation changed. Reopen its saved checks."));
    const cacheKey = JSON.stringify([owner, id, key]);
    const existing = savedCaseReads.current.get(cacheKey);
    if (existing) return existing;
    const request = token().then(auth => vibeFetch<CaseResult>(`/operations/${id}/case?key=${encodeURIComponent(key)}`, auth));
    savedCaseReads.current.set(cacheKey, request);
    const forget = () => {
      if (savedCaseReads.current.get(cacheKey) === request) savedCaseReads.current.delete(cacheKey);
    };
    void request.then(forget, forget);
    return request;
  }
  const buildStart = session?.document.evaluation?.door === "build" && !session.document.build && !session.document.artifacts.length;
  const continuingBuild = (session?.document.build?.phase === "clarifying" || session?.document.build?.phase === "waiting");
  const quoteMatches = !!buildQuote && buildQuote.request.content === content && JSON.stringify(buildQuote.request.models) === JSON.stringify(models) && Date.parse(buildQuote.expires_at) > Date.now();
  useEffect(() => {
    if (!buildStart || !sessionID || !content.trim()) { setBuildQuote(undefined); setQuoteError(""); return; }
    let live = true;
    setQuoteError("");
    const timer = setTimeout(() => {
      void token().then(auth => vibeFetch<BuildQuote>(`/sessions/${sessionID}/build-quote`, auth, { method: "POST", body: JSON.stringify({ content, models }) }))
        .then(q => { if (live) setBuildQuote(q); }).catch(e => { if (live) setQuoteError(e.message); });
    }, 400);
    return () => { live = false; clearTimeout(timer); };
  }, [buildStart, sessionID, content, models, token]);
  useEffect(() => {
    if (!config?.two_door) return;
    let live = true;
    void token().then(auth => vibeFetch<Session[]>("/sessions", auth))
      .then(items => { if (live) setContexts(items); }).catch(() => undefined);
    return () => { live = false; };
  }, [sessionID, session?.event_cursor, token, config?.two_door]);
  useEffect(() => {
    const id = session?.document.build?.check_id;
    if (id && !requestedRunID) setRequestedRunID(id);
  }, [session?.document.build?.check_id, requestedRunID]);
  function adoptContext(v: Session) {
    setNewEvaluation(false);
    if (sessionID) {
      contextDrafts.current[sessionID] = content;
      contextTrials.current[sessionID] = { artifact: artifact?.id, thread: threadID, target: models.target, trying: view === "try" };
    }
    activeSessionRef.current = v.id;
    composerEdits.current++;
    setContent(contextDrafts.current[v.id] ?? readBuildDrafts(v.id)?.guide ?? "");
    const previousTrial = contextTrials.current[v.id];
    const restoreTrial = previousTrial && v.document.artifacts.some(a => a.id === previousTrial.artifact);
    setSession(v); setModels(restoreTrial ? { ...v.document.models, target: previousTrial.target } : v.document.models);
    setSelectedArtifactID(restoreTrial ? previousTrial.artifact || null : null);
    setThreadID(restoreTrial ? previousTrial.thread : "");
    const nextView = restoreTrial && previousTrial.trying ? "try" : "build";
    setView(nextView); currentView.current = nextView; setRequestedRunID(undefined);
    setError(""); setConnection(""); setPendingMessage(undefined); setPendingEdit(undefined);
    setChecksDirtyID(null); setDirtyArtifactID(null); setBuildQuote(undefined); setRunQuote(undefined);
    buildEvidence.current = undefined;
    const restoredURL = new URL("/vibe-evals", window.location.origin);
    restoredURL.searchParams.set("session", v.id);
    restoredURL.searchParams.set("view", nextView);
    if (restoreTrial && previousTrial.artifact) restoredURL.searchParams.set("agent", previousTrial.artifact);
    if (restoreTrial && previousTrial.thread) restoredURL.searchParams.set("thread", previousTrial.thread);
    window.history.replaceState(null, "", restoredURL.pathname + restoredURL.search);
  }
  async function chooseDoor(door: "build" | "test") {
    if (!config || sending.current || pending || uncertain || dirtyArtifact) return;
    setPending(true);
    try {
      entryOrigin.current ||= sessionID;
      if (!contextChoice.current || contextChoice.current.door !== door)
        contextChoice.current = { door, id: crypto.randomUUID() };
      const auth = await token();
      const id = contextChoice.current.id;
      await vibeFetch<Session>("/sessions", auth, { method: "POST", body: JSON.stringify({ id, door, ...(workspace ? { workspace_id: workspace } : {}) }) });
      // Confirm that private-cookie ownership survived before adopting the URL.
      const agent = await vibeFetch<Session>(`/sessions/${id}`, auth);
      const draft = canChangeDoor ? content : "";
      if (canChangeDoor && sessionID) discardedContextIDs.current.add(sessionID);
      adoptContext(agent);
      setContent(draft);
      contextChoice.current = undefined;
      if (door === "build") captureBuildEvent(WEB_EVENTS.VIBE_BUILD_ENTRY_SELECTED, { session_id: agent.id, entry_source: "two_door" }, agent.id);
    } catch (e) { setError((e as Error).message); }
    finally { setPending(false); }
  }
  async function switchContext(id: string) {
    if (id === sessionID || pending || uncertain || dirtyArtifact) return;
    setPending(true);
    try { adoptContext(await vibeFetch<Session>(`/sessions/${id}`, await token())); }
    catch (e) { setError((e as Error).message); }
    finally { setPending(false); }
  }
  function openNewEvaluation() {
    if (sessionID) entryOrigin.current = sessionID;
    setNewEvaluation(true);
  }
  const canChangeDoor = !!session?.document.evaluation &&
    session.document.messages.length === 0 &&
    session.document.artifacts.length === 0 &&
    session.operations.length === 0 &&
    !(session.document.evidence_sets?.length) &&
    !session.document.build &&
    !pending && !uncertain && !dirtyArtifact;
  async function changeDoor() {
    if (!canChangeDoor || !sessionID) return;
    const origin = entryOrigin.current;
    if (!origin || origin === sessionID) {
      setNewEvaluation(true);
      return;
    }
    setPending(true);
    try {
      const previous = await vibeFetch<Session>(`/sessions/${origin}`, await token());
      discardedContextIDs.current.add(sessionID);
      adoptContext(previous);
      setNewEvaluation(true);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setPending(false);
    }
  }
  function sendMessage(text = content, context?: { viewed_run_id: string }, demoID?: string) {
    if (buildStart && !quoteMatches) return;
    void submit("message", text, undefined, {
      ...context,
      ...(demoID ? { demo_id: demoID } : {}),
      ...(buildStart ? { cycle_id: buildQuote!.id } : continuingBuild ? { cycle_id: session!.document.build!.cycle_id } : {}),
      ...(testJourney || session?.document.evaluation ? {} : { quick_check: !buildEvidence.current }),
    });
  }
  async function requestPreparation(text: string, count: number, artifactID: string) {
    if (!session || busy) return;
    const clientID = crypto.randomUUID();
    const extra = { client_id: clientID, additional_examples: count, artifact_id: artifactID };
    setPending(true); setError("");
    try {
      const quote = await vibeFetch<BuildQuote>(`/sessions/${session.id}/build-quote`, await token(), {method:"POST", body:JSON.stringify({ content:text, models, artifact_id:artifactID, additional_examples:count })});
      setRunQuote({quote: {...quote, calls: quote.max_calls}, kind:"message", content:text, extra: {...extra, cycle_id:quote.id}, sessionID:session.id, revision:session.revision});
    } catch(e) { setError((e as Error).message); } finally {setPending(false);}
  }
  async function requestRun(baseline?: Operation, evidenceID?: string, purpose?: "regrade") {
    const extra = { artifact_id: artifact?.id, ...(purpose ? {purpose} : {}), ...(evidenceID ? { evidence_set_id: evidenceID } : {}), ...(baseline?.source?.kind === "provided_conversations" ? { artifact_id: baseline.source.artifact_id } : {}) };
    const kind = baseline ? "retest" : "check";
    if (!session?.document.evaluation) { void submit(kind, "", baseline, extra); return; }
    setPending(true);
    try {
      const q = await vibeFetch<RunQuote>(`/sessions/${session.id}/run-quote`, await token(), { method: "POST", body: JSON.stringify({ client_id: crypto.randomUUID(), revision: session.revision, kind, models: baseline ? {...models, evaluator: baseline.models.evaluator} : models, baseline_id: baseline?.id, approve_artifact: true, ...extra }) });
      setRunQuote({ quote: q, kind, baseline, extra, sessionID: session.id, revision: session.revision });
    } catch (e) { setError((e as Error).message); }
    finally { setPending(false); }
  }
  async function submit(
    kind = "message",
    text = content,
    baseline?: Operation,
    extra: {
      client_id?: string;
      cycle_id?: string;
      run_quote_id?: string;
      additional_examples?: number;
      quick_check?: boolean;
      instructions?: string;
      purpose?: "suggest_change" | "regrade";
      viewed_run_id?: string;
      viewed_case_key?: string;
      demo_id?: string;
      evidence_set_id?: string;
      artifact_id?: string;
      baseline_id?: string;
    } = {},
  ) {
    if (sending.current || busy || submission.current) return;
    if (kind === "message" && !text.trim()) return;
    if (
      dirtyArtifact ||
      (kind !== "message" && (!artifact || artifact.kind === "test_plan"))
    )
      return;
    let trialKey: string | undefined;
    let previewThread: string | undefined;
    if (kind === "playground") {
      if (!text.trim() || legacyTrial) return;
      previewThread = threadID || crypto.randomUUID();
      trialKey = `${trialPrefix}${previewThread}`;
      if (!threadID) {
        trialEdits.current[trialKey] =
          trialEdits.current[emptyTrialKey] || 0;
        setTrialBuffers((old) => ({
          ...old,
          [trialKey!]: text,
          [emptyTrialKey]: "",
        }));
        setThreadID(previewThread);
        navigate(buildJourney && view !== "try" ? "build" : "try", artifact?.id, previewThread);
      }
    }
    const composerVersion = trialKey
      ? trialEdits.current[trialKey] || 0
      : composerEdits.current;
    const clientID = extra.client_id || crypto.randomUUID();
    if (kind === "message") {
      retryUnsentMessage.current = undefined;
      setPendingMessage({ id: clientID, content: text });
      // Move the submitted text into the conversation immediately. The version
      // check below protects anything the user types while admission is pending.
      if (!extra.demo_id) setContent((current) => (current === text ? "" : current));
    }
    sending.current = true;
    if (kind === "check" || kind === "retest") setRunPending(true);
    setPending(true);
    setError("");
    try {
      const v = await ensureSession();
      submission.current = {
        sessionID: v.id,
        uncertain: false,
        composer: kind === "playground" || kind === "message" && !extra.demo_id ? text : null,
        composerVersion,
        trialKey,
        body: JSON.stringify({
          client_id: clientID,
          revision: v.revision,
          kind,
          evaluation_first: true,
          ...(testJourney ? { test_journey: true } : {}),
          content: text,
          models: baseline
            ? { ...models, evaluator: baseline.models.evaluator }
            : models,
          ...((kind === "message" && workingArtifact) ? { artifact_id: workingArtifact.id } : artifact ? { artifact_id: artifact.id } : {}),
          ...(baseline ? { baseline_id: baseline.id } : {}),
          ...(kind === "message" &&
          buildEvidence.current?.artifact === artifact?.id
            ? { baseline_id: buildEvidence.current?.operation }
            : {}),
          ...(kind === "check" || kind === "retest"
            ? { approve_artifact: true }
            : {}),
          ...(previewThread ? { preview_thread_id: previewThread } : {}),
          ...extra,
        }),
      };
      await dispatchSubmission();
      if (kind === "message" && currentView.current === "build") {
        setSelectedArtifactID(null);
        setThreadID("");
        const url = new URL(window.location.href);
        url.searchParams.delete("agent");
        url.searchParams.delete("thread");
        window.history.replaceState(null, "", url.pathname + url.search);
      }
    } catch (e) {
      setError((e as Error).message);
      if (
        kind === "message" && !extra.demo_id &&
        !submission.current &&
        composerEdits.current === composerVersion
      ) {
        setContent((current) => current || text);
        setPendingMessage(undefined);
      } else if (kind === "message" && !submission.current && !extra.demo_id) {
        retryUnsentMessage.current = () =>
          void submit(kind, text, baseline, extra);
      } else if (extra.demo_id && !submission.current) {
        setPendingMessage(undefined);
      }
    } finally {
      sending.current = false;
      setPending(false);
      setRunPending(false);
    }
  }
  async function dispatchSubmission() {
    const request = submission.current;
    if (!request) return;
    let admitted: Operation;
    try {
      admitted = request.retryOperationID
        ? await retryVibeOperation(request.sessionID, request.retryOperationID, JSON.parse(request.body), await token())
        : await vibeFetch<Operation>(
            `/sessions/${request.sessionID}/messages`,
            await token(),
            { method: "POST", body: request.body },
          );
    } catch (e) {
      // Auth/rate/profile checks precede idempotency lookup. A later rejection
      // cannot disprove an earlier admission: retain every byte until acknowledged.
      const rejected =
        !request.uncertain &&
        e instanceof VibeError &&
        e.status !== undefined &&
        (submissionRejections[e.status]?.includes(e.code) === true ||
          (!!request.retryOperationID && retryRejections[e.status]?.includes(e.code) === true));
      if (rejected) submission.current = null;
      else request.uncertain = true;
      setUncertain(!rejected);
      if (e instanceof VibeError && (e.code === "revision_conflict" || (request.retryOperationID && rejected))) {
        await reload(request.sessionID).catch(() => undefined);
      }
      throw e;
    }
    submission.current = null;
    setUncertain(false);
    if (activeSessionRef.current !== request.sessionID) return;
    if (request.retryOperationID && JSON.parse(request.body).assistant_model)
      setModels(current => ({ ...current, assistant: admitted.models.assistant }));
    const submitted = JSON.parse(request.body);
    const requestKind = submitted.kind;
    if (session?.document.evaluation?.door === "build" && (requestKind === "message" || requestKind === "playground")) {
      const base = { session_id: request.sessionID, operation_id: admitted.id, artifact_id: submitted.artifact_id, sample: !!artifact?.sample };
      captureBuildEvent(WEB_EVENTS.VIBE_BUILD_MESSAGE_ADMITTED, { ...base, recipient: requestKind === "playground" ? "prototype" : "guide" }, admitted.id);
      if (submitted.demo_id) captureBuildEvent(WEB_EVENTS.VIBE_BUILD_DEMO_SELECTED, { ...base, sample_id: "email" }, admitted.id);
    }
    if (requestKind === "check" || requestKind === "retest")
      setRequestedRunID(admitted.id);
    if (!request.trialKey && request.composer !== null)
      buildEvidence.current = undefined;
    if (request.trialKey) {
      const key = request.trialKey;
      if (request.composerVersion === (trialEdits.current[key] || 0))
        setTrialBuffers((old) =>
          old[key] === request.composer ? { ...old, [key]: "" } : old,
        );
    } else if (
      request.composer !== null &&
      request.composerVersion === composerEdits.current
    ) {
      setContent((current) => (current === request.composer ? "" : current));
    }
    // A failed refresh cannot turn an acknowledged send into an unsent draft.
    // SSE can reconcile it; retain the visible pending message until then.
    const fresh = await reload(request.sessionID).catch((error: unknown) => {
      setConnection(
        isSessionAccessError(error)
          ? sessionAccessLost
          : "Reconnecting to saved progress…",
      );
      return undefined;
    });
    if (fresh && !request.trialKey && request.composer !== null)
      setPendingMessage(undefined);
  }

  useEffect(() => {
    const admitted = session?.operations.find(
      (operation) => operation.id === requestedRunID,
    );
    if (admitted && navigatedRunID.current !== requestedRunID) {
      navigatedRunID.current = requestedRunID;
      if (session?.document.evaluation?.door !== "build") navigate("checks", admitted.source?.artifact_id);
    }
    // Navigation happens only when the requested run is present, never against
    // the previous result while a new run is still being admitted.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [requestedRunID, session]);

  // Reconcile an SSE acknowledgement that can arrive before the POST returns.
  useEffect(() => {
    if (
      pendingMessage &&
      session?.document.messages.some(
        (message) => message.id === pendingMessage.id,
      )
    )
      setPendingMessage(undefined);
  }, [pendingMessage, session]);

  // A saved child operation confirms that the original failed action has a
  // retry in progress, even if its HTTP acknowledgement was lost.
  useEffect(() => {
    const request = submission.current;
    if (!request?.uncertain || !request.retryOperationID || session?.id !== request.sessionID) return;
    const admitted = session.operations.find(operation => operation.retry_of_operation_id === request.retryOperationID);
    if (!admitted) return;
    if (JSON.parse(request.body).assistant_model)
      setModels(current => ({ ...current, assistant: admitted.models.assistant }));
    submission.current = null;
    setUncertain(false);
    setError("");
  }, [session, uncertain]);

  useEffect(() => {
    if (!pendingEdit) return;
    const operation = session?.operations.find(item => item.id === pendingEdit.operationID);
    if (!operation) return;
    const artifactID = operation.completion_receipt?.artifact_id;
    if (artifactID && session?.document.artifacts.some(item => item.id === artifactID)) {
      setSelectedArtifactID(artifactID);
      navigate("build", artifactID, "");
      setChecksDirtyID(null);
      setPendingEdit(undefined);
    } else if (terminal(operation.state)) {
      // Leave the original editor and draft intact after failed validation.
      setPendingEdit(undefined);
    }
    // Navigation follows the persisted completion, not unrelated view changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pendingEdit, session]);

  async function retrySubmission() {
    if (sending.current || pending || !submission.current) return;
    sending.current = true;
    setPending(true);
    if (submission.current.retryOperationID) setPendingAction("Retrying your request…");
    setError("");
    try {
      await dispatchSubmission();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      sending.current = false;
      setPending(false);
      setPendingAction(undefined);
    }
  }
  async function retryOperation(id: string, assistantModel?: string) {
    if (submission.current?.retryOperationID === id) {
      await retrySubmission();
      return;
    }
    if (!session || sending.current || busy || dirtyArtifact || submission.current) return;
    const operation = session.operations.find(item => item.id === id);
    if (!operation?.retryable || operation.completion_receipt) return;
    if (buildJourney) captureBuildEvent(WEB_EVENTS.VIBE_BUILD_RECOVERY_CLICKED, { session_id: session.id, operation_id: id, artifact_id: operation.source?.artifact_id, action: assistantModel ? "alternate_assistant" : "retry", error_code: operation.error?.code });
    submission.current = {
      sessionID: session.id,
      retryOperationID: id,
      body: JSON.stringify({ client_id: crypto.randomUUID(), revision: session.revision, ...(assistantModel ? { assistant_model: assistantModel } : {}) }),
      composer: null,
      composerVersion: composerEdits.current,
      uncertain: false,
    };
    await retrySubmission();
  }
  async function applyChoice(action: ConversationAction) {
    if (!session) return;
    setPending(true);
    try {
      const next = await vibeFetch<Session>(`/sessions/${session.id}/actions`, await token(), {
        method: "POST", body: JSON.stringify({ version: 1, kind: "action", payload: action }),
      });
      setSession(next);
      const restored = next.document.artifacts.at(-1);
      if (action.kind === "undo" && restored) {
        setSelectedArtifactID(restored.id);
        navigate("build", restored.id);
      }
    } finally { setPending(false); }
  }
  async function reloadChoices() {
    if (session) setSession(await vibeFetch<Session>(`/sessions/${session.id}`, await token()));
  }

  async function edit(fields: Record<string, unknown>) {
    if (!session) return false;
    setPending(true);
    setError("");
    try {
      const v = await vibeFetch<Session>(
        `/sessions/${session.id}`,
        await token(),
        {
          method: "PATCH",
          body: JSON.stringify({ revision: session.revision, ...fields }),
        },
      );
      setSession(v);
      const suiteChange = fields.case_changes !== undefined || fields.criteria !== undefined || fields.evaluation !== undefined;
      const admittedEdit = suiteChange && v.operations.find(operation => !session.operations.some(previous => previous.id === operation.id));
      if (admittedEdit && !admittedEdit.completion_receipt) {
        setPendingEdit({ operationID: admittedEdit.id, artifactID: String(fields.artifact_id || artifact?.id || "") });
        return false;
      }
      if (
        fields.agent_prompt !== undefined ||
        fields.build_from_policy === true ||
        fields.evaluation !== undefined ||
        fields.case_changes !== undefined ||
        fields.criteria !== undefined ||
        fields.test_scenarios !== undefined ||
        fields.expectations !== undefined
      ) {
        const next = v.document.artifacts.at(-1);
        if (next) {
          setSelectedArtifactID(next.id);
          navigate(view, next.id, "");
        }
        setThreadID("");
        setDirtyArtifactID(null);
        setChecksDirtyID(null);
      }
      return true;
    } catch (e) {
      setError((e as Error).message);
      await reload();
      return false;
    } finally {
      setPending(false);
    }
  }
  async function operationAction(id: string, action: "stop" | "approve") {
    if (action === "stop" && buildJourney && session) captureBuildEvent(WEB_EVENTS.VIBE_BUILD_RECOVERY_CLICKED, { session_id: session.id, operation_id: id, action: "stop" });
    setPending(true);
    setPendingAction(action === "stop" ? "Stopping…" : "Starting your check…");
    setError("");
    try {
      await vibeFetch(`/operations/${id}/${action}`, await token(), {
        method: "POST",
        body: "{}",
      });
      await reload();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setPending(false);
      setPendingAction(undefined);
    }
  }
  async function upload(uploaded?: File) {
    if (!uploaded || busy) return;
    setPending(true);
    setError("");
    try {
      const v = await ensureSession();
      if (uploaded.size > (v.anonymous ? 256 * 1024 : 1024 * 1024))
        throw new Error("This file exceeds the import size limit.");
      const result = await vibeFetch<Session>(
        `/sessions/${v.id}/import`,
        await token(),
        {
          method: "POST",
          headers: {
            "Content-Type": uploaded.name.endsWith(".json")
              ? "application/json"
              : "application/yaml",
            "If-Match": String(v.revision),
          },
          body: new Blob([uploaded]),
        },
      );
      setSession(result);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setPending(false);
      if (file.current) file.current.value = "";
    }
  }
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
    if (!saveOpen || authLoading) return;
    let current = true;
    setSaveAccess("loading");
    void token().then(async auth => {
      if (!current) return;
      if (!auth) { setWorkspaces([]); setSaveAccess("signed_out"); return; }
      try {
        const me = await createApiClient(auth).get<UserMeResponse>("/v1/users/me");
        if (!current) return;
        const available = me.organizations.flatMap(o => o.workspaces.filter(w =>
          (["workspace_admin", "workspace_member"].includes(w.role) || o.role === "org_admin") && (!attachedWorkspace || w.id === attachedWorkspace)));
        setWorkspaces(available);
        setWorkspace(old => attachedWorkspace || (available.some(w => w.id === old) ? old : available[0]?.id || ""));
        setSaveAccess("ready");
      } catch (e) {
        if (!current) return;
        const expired = (e as { status?: number }).status === 401;
        setSaveAccess(expired ? "signed_out" : "error");
        setError(expired ? "Sign in again to keep your work. Your tests are still here." : "Couldn’t load your workspaces. Try again.");
      }
    });
    return () => { current = false; };
  }, [saveOpen, token, attachedWorkspace, loadAttempt, authLoading]);
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
      window.history.replaceState(null, "", url.pathname + url.search);
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
    setPending(true);
    setError("");
    try {
      const auth = await token();
      if (!auth) { setSaveAccess("signed_out"); return; }
      await vibeFetch(`/sessions/${session.id}/claim`, auth, {
        method: "POST",
        body: "{}",
      });
      const latest = await reload();
      if (!latest) return;
      if (artifact.kind === "test_plan") {
        const receipt = await vibeFetch<SavedCheck>(`/sessions/${session.id}/save-brief`, auth, {
          method: "POST", body: JSON.stringify({ revision: latest.revision, artifact_id: artifact.id, workspace_id: workspace }),
        });
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
      await reload();
      const kept = await vibeFetch<SavedCheck[]>("/saved-checks", auth);
      setSavedChecks(Array.isArray(kept) ? kept : []);
    } catch (e) {
      if (e instanceof VibeError && e.status === 401) setSaveAccess("signed_out");
      if (e instanceof VibeError && e.status === 403) {
        setWorkspaces([]); setSaveAccess("ready");
        setError("Your workspace access changed. Ask its owner for permission to save here.");
      } else setError((e as Error).message);
    } finally {
      setPending(false);
    }
  }

  const activeContext = session?.document.evaluation ? evaluationIdentity(session) : null;
  const setTrialText = (text: string) => {
    const key = trialDraftKey;
    trialEdits.current[key] = (trialEdits.current[key] || 0) + 1;
    setTrialBuffers(old => ({ ...old, [key]: text }));
  };
  return (
    <main className="vibe-workspace dark flex h-dvh flex-col overflow-hidden font-sans">
      <Dialog open={!!runQuote} onOpenChange={open => { if (!open) setRunQuote(undefined); }}>
        <DialogContent><DialogTitle>{runQuote?.kind === "message" ? "Try tougher situations" : runQuote?.baseline ? "Rerun the same examples" : "Try these examples"}</DialogTitle>
          <DialogDescription>{runQuote?.kind === "message" ? `${runQuote.extra.additional_examples} new situations + ${(runQuote.quote.cases || 0) - (runQuote.extra.additional_examples || 0)} existing examples` : `${runQuote?.quote.cases} examples`} · up to {dollars(runQuote?.quote.max_cost_nano_usd || 0)}. Usually a few minutes; provider queues can take longer. {runQuote?.kind === "message" ? "Includes preparation, review, bounded repairs and running this batch. Existing cases and their earlier results stay unchanged." : runQuote?.baseline ? "The same examples and grading stay fixed. Earlier results are kept." : "This run uses the selected instructions or recorded replies."}</DialogDescription>
          <VibeButton variant="primary" disabled={busy || runQuote?.sessionID !== sessionID || runQuote?.revision !== session?.revision} onClick={() => { if (!runQuote) return; const q=runQuote; setRunQuote(undefined); void submit(q.kind,q.content || "",q.baseline,{...q.extra,...(q.extra.cycle_id ? {} : {run_quote_id:q.quote.id})}); }}>{runQuote?.kind === "message" ? "Prepare and run this batch" : `Run ${runQuote?.quote.cases} examples`}</VibeButton>
        </DialogContent>
      </Dialog>
      <EvaluationNavigation enabled={twoDoor} contexts={contexts.filter(context => !discardedContextIDs.current.has(context.id))} session={session}
        choosing={newEvaluation} disabled={!config || pending || uncertain || dirtyArtifact}
        onNew={openNewEvaluation}
        onSwitch={id => { if (id === sessionID) setNewEvaluation(false); else void switchContext(id); }}
        onSettings={() => setSettingsOpen(true)} onSavedWork={() => setSavedWorkOpen(true)}>
      {navigationToggle => session && session.document.format_version !== 1 && !newEvaluation && config?.two_door ? <ArchivedConversation key={session.id} session={session} navigation={navigationToggle} busy={pending} error={error}
        onExport={exportConversation} loadEvidence={(id, key) => token().then(auth => vibeFetch<CaseResult>(`/operations/${id}/case?key=${encodeURIComponent(key)}`, auth))} onContinue={async (artifactID, clientID) => {
          setPending(true); setError("");
          try {
            const copy = await vibeFetch<Session>(`/sessions/${session.id}/continue`, await token(), { method: "POST", body: JSON.stringify({ client_id: clientID, artifact_id: artifactID }) });
            adoptContext(copy);
          } catch (e) { setError((e as Error).message); } finally { setPending(false); }
        }} /> : <EvaluationWorkspace
        key={session?.id || "entry"}
        twoDoor={twoDoor}
        navigationToggle={navigationToggle}
        newEvaluation={newEvaluation}
        contextNavigationBlocked={!config || pending || uncertain || dirtyArtifact}
        onCancelNew={() => setNewEvaluation(false)}
        canChangeDoor={canChangeDoor}
        onChangeDoor={() => void changeDoor()}
        savedWorkOpen={savedWorkOpen}
        onSavedWorkOpenChange={setSavedWorkOpen}
        onDoor={door => void chooseDoor(door)}
        contextControl={activeContext && <p className="vibe-context-label" aria-label="Active evaluation" title={`${activeContext.title} · ${activeContext.scope}`}>
          <span className="vibe-context-dot" aria-hidden="true" /><span className="vibe-context-title">{activeContext.title}</span><span>· {activeContext.scope}</span>
        </p>}
        buildStart={buildStart}
        onSample={() => sendMessage("I don’t know. Use a clearly labelled sample policy or a narrower sample demonstration.")}
        onDemo={id => sendMessage("Try a sample email assistant", undefined, id)}
        sendBlocked={buildStart && !quoteMatches}
        sendError={buildStart ? quoteError : undefined}
        testJourney={testJourney}
        interactionActions={config?.interaction_actions}
        onChoice={applyChoice}
        onReloadChoices={reloadChoices}
        modelLabel={
          config?.models?.find((m) => m.id === models.target)?.name ||
          models.target
        }
        session={session}
        artifact={artifact}
        view={view}
        busy={busy}
        onRetry={(id, model) => void retryOperation(id, model)}
        retryModels={config?.models}
        retryPendingOperationID={pending ? submission.current?.retryOperationID : undefined}
        retryUncertainOperationID={uncertain ? submission.current?.retryOperationID : undefined}
        checkingTestChanges={pendingEdit?.artifactID === artifact?.id && !!pendingEdit}
        pendingMessage={pendingMessage}
        requestedRunID={requestedRunID}
        quickChecking={runPending}
        pendingLabel={
          (loadingSession && !session ? "Loading your conversation…" : undefined) ||
          pendingAction ||
          (pending && !active
            ? runPending
              ? "Starting your check…"
              : pendingMessage
                ? "Sending your message…"
                : "Saving your changes…"
            : undefined)
        }
        dirty={dirtyArtifact}
        content={content}
        onContent={(value) => {
          composerEdits.current++;
          setContent(value);
        }}
        onSend={context => sendMessage(content, context)}
        onTougher={(text, count, artifactID) => {
          navigate("build");
          setSelectedArtifactID(null);
          void requestPreparation(text, count, artifactID);
        }}
        onMessage={(text, operation, instructions) => {
          navigate("build");
          void submit("message", instructions && session?.document.format_version === 1 ? `${text}\n\nAgent instructions to check:\n${instructions}` : text, undefined, {
            ...(instructions ? { instructions } : {}),
            ...(operation
              ? {
                  artifact_id:
                    operation.source?.artifact_id ||
                    operation.results[0]?.version,
                  baseline_id: operation.id,
                  purpose: "suggest_change",
                }
              : {}),
          });
        }}
        onNavigate={navigate}
        onRun={(baseline, evidenceID) => void requestRun(baseline, evidenceID)}
        onEdit={edit}
        onDirty={(dirty) => {
          setChecksDirtyID(dirty ? artifact?.id || null : null);
          if (dirty && artifact) setSelectedArtifactID(artifact.id);
        }}
        onSave={openSave}
        onSettings={() => setSettingsOpen(true)}
        onImport={() => file.current?.click()}
        loadEvidence={loadSavedCase}
        onAction={operationAction}
        onRegrade={config?.grading_recheck ? operation => void requestRun(operation, undefined, "regrade") : undefined}
        onDispute={(rule, result, operation) => {
          const source = artifacts.find((a) => a.id === result.version);
          if (source) setSelectedArtifactID(source.id);
          buildEvidence.current = testJourney
            ? undefined
            : {
                operation: operation.id,
                artifact: result.version,
              };
          composerEdits.current++;
          setContent(
            `That’s not our rule: “${rule}”\n\nThe correct expectation is: `,
          );
          navigate("build", result.version);
          requestAnimationFrame(() =>
            document.getElementById("vibe-message")?.focus(),
          );
        }}
        savedChecks={savedChecks}
        notice={
          <>
            {configError && (
              <div className="mb-3 space-y-2 text-sm">
                <p role="alert">
                  Couldn’t connect. Your message is still here.
                </p>
                <VibeButton
                  onClick={() => setLoadAttempt((attempt) => attempt + 1)}
                >
                  Retry connection
                </VibeButton>
              </div>
            )}
            {savedDraft && (
              <div className="mb-3 space-y-2 text-sm vibe-muted">
                <p>
                  {saved
                    ? testJourney
                      ? "Your tests are saved. Find them in Saved work whenever you update your agent."
                      : "Your agent and checks are saved."
                    : savedModelNotice}
                </p>
                <Link
                  className="underline"
                  href={`/workspaces/${savedDraft.workspace_id}/challenge-packs/builder/${savedDraft.draft_id}`}
                >
                  {testJourney
                    ? "Open tests in the advanced editor"
                    : "Open your evaluation"}
                </Link>
              </div>
            )}
            {workspace && !testJourney && !session?.document.evaluation_first && (
              <CreditsDialog workspace={workspace} />
            )}
            {artifact &&
              artifact.kind !== "test_plan" &&
              artifact.kind !== "conversation_evaluation" &&
              !testJourney && !session?.document.evaluation_first && (
                <VibeButton
                  variant="quiet"
                  disabled={busy || dirtyArtifact}
                  onClick={() => void openSave()}
                >
                  Save agent
                </VibeButton>
              )}
            {sessionUnavailable && !loadingSession && (
              <p role="status" className="mb-3 text-sm vibe-muted">
                This conversation could not be loaded.
              </p>
            )}
            {session && selectedArtifactID && !artifact && <p role="alert" className="text-sm text-builder-warn">This test version is unavailable. Open History to choose saved work.</p>}
            {error && (
              <p role="alert" className="mb-3 text-sm text-builder-warn">
                {error}
              </p>
            )}
            {error && pendingMessage && !busy && (
              <VibeButton onClick={() => retryUnsentMessage.current?.()}>
                Retry unsent message
              </VibeButton>
            )}
            {session?.operations.at(-1)?.error && !session.operations.at(-1)?.completion_receipt && !session.operations.at(-1)?.retry_of_operation_id && !session.document.messages.some(message => message.role === "user" && message.origin !== "playground" && message.operation_id === session.operations.at(-1)?.id) && (
              <p role="alert" className="mb-3 text-sm text-builder-warn">
                {session.operations.at(-1)?.error?.message}
              </p>
            )}
            {connection && (
              <p role="status" className="mb-3 text-sm vibe-muted">
                {connection}
              </p>
            )}
            {(sessionUnavailable || connection === sessionAccessLost) &&
              !loadingSession && (
                <VibeButton
                  onClick={() => {
                    setConnection("");
                    setError("");
                    setLoadAttempt((n) => n + 1);
                  }}
                >
                  Retry connection
                </VibeButton>
              )}
            {connection === sessionAccessLost &&
              (!session ||
                (!session.document.messages.length &&
                  !session.document.artifacts.length &&
                  !session.document.requirements.length &&
                  !session.operations.length)) && (
                <VibeButton
                  disabled={pending || uncertain}
                  onClick={() => {
                    setSession(null);
                    setError("");
                    setConnection("");
                    window.history.replaceState(
                      null,
                      "",
                      `/vibe-evals${workspace ? `?workspace=${encodeURIComponent(workspace)}` : ""}`,
                    );
                    document.getElementById("vibe-message")?.focus();
                  }}
                >
                  Keep message in a new conversation
                </VibeButton>
              )}
            {uncertain && !submission.current?.retryOperationID && (
              <div className="space-y-2 text-sm vibe-muted">
                <p>
                  The submission acknowledgement was not confirmed. Retry to
                  recover its saved status. Your next message stays here.
                </p>
                <VibeButton disabled={pending} onClick={retrySubmission}>
                  Retry submission
                </VibeButton>
              </div>
            )}
            {dirtyArtifact && !testJourney && (
              <p className="mb-3 text-sm vibe-muted">
                Save or discard your edits before running a check or sending a
                message.
              </p>
            )}
            {latestArtifact && artifact?.id !== latestArtifact.id && (
              <VibeButton
                disabled={busy || dirtyArtifact}
                onClick={() => {
                  setSelectedArtifactID(latestArtifact.id);
                  navigate("build", latestArtifact.id, "");
                }}
              >
                Review latest version
              </VibeButton>
            )}
          </>
        }
        preview={
          <PrototypeTrial inline={buildJourney} dock={buildJourney} title={artifact?.title} version={session && artifact ? buildVersion(session, artifact) : undefined} busy={busy || dirtyArtifact} text={trialText}
            onText={setTrialText} onSend={() => void submit("playground", trialText)}
            onBack={() => navigate("build")} onNew={newTrial} thread={threadID}
            history={trialHistory} messages={trialMessages}
            model={config?.models?.find(m => m.id === models.target)}
            example={evaluation?.scenarios?.[0]?.input || evaluation?.examples[0]}
            onThread={id => {
              const next = trialHistory.find(t => t.id === id);
              setThreadID(id);
              if (next?.model) setModels(old => ({ ...old, target: next.model! }));
              navigate(buildJourney && view !== "try" ? "build" : "try", artifact?.id, id);
            }} />
        }
      />
      }
      </EvaluationNavigation>
      <AgentSettings open={settingsOpen} onOpenChange={setSettingsOpen}
        config={config} session={session} artifact={artifact} artifacts={artifacts}
        models={models} busy={busy || dirtyArtifact} workspace={workspace}
        testJourney={testJourney} savedDraft={savedDraft} onModels={changeModels}
        onVersion={id => {setSelectedArtifactID(id); setThreadID(""); navigate(view, id, "");}}
        onSave={() => void openSave()} onImport={() => file.current?.click()}
        onExportConversation={() => void exportConversation()} />
      <input
        ref={file}
        type="file"
        accept=".json,.yaml,.yml"
        className="hidden"
        aria-label="Evaluation file"
        onChange={(e) => upload(e.target.files?.[0])}
      />
      <Dialog open={saveOpen} onOpenChange={closeSave}>
        <DialogContent className="vibe-workspace">
          <DialogTitle>
            {artifact?.kind === "test_plan" ? "Keep this brief" : testJourney
              ? "Keep these tests"
              : saveTarget
                ? "Save this check"
                : "Save agent and checks"}
          </DialogTitle>
          <DialogDescription>
            {artifact?.kind === "test_plan" ? "Keep your plan for when you’re ready. No agent or results are needed." : testJourney
              ? "Save this test set so you can run it again after changing your agent."
              : saveTarget
                ? "Keep the source, expectations and this result together. Only you can reopen this check. Future runs use your workspace’s AI credits."
                : `Save these instructions and ${scenarioCount || "the reviewed"} checks to your workspace. Future checks use your workspace’s AI credits.`}
          </DialogDescription>
          {error && (
            <p role="alert" className="text-xs text-builder-warn">
              {error}
            </p>
          )}
          {saveAccess === "loading" ? <p role="status" className="text-sm vibe-muted">Loading your workspaces…</p>
            : saveAccess === "error" ? <Button onClick={() => setLoadAttempt(n => n + 1)}>Try again</Button>
            : saveAccess === "ready" && !workspaces.length ? <p className="text-sm">You need a workspace you can save to. <Link className="underline" href="/dashboard">Open your workspace</Link>, or ask its owner for access. Your work is still here.</p>
            : saveAccess === "ready" ? (
            <>
              <label className="text-sm">
                Workspace
                <select
                  aria-label="Save workspace"
                  value={workspace}
                  onChange={(e) => setWorkspace(e.target.value)}
                  className="mt-2 w-full rounded-lg border bg-background p-3"
                >
                  {workspaces.map((w) => (
                    <option key={w.id} value={w.id}>
                      {w.name}
                    </option>
                  ))}
                </select>
              </label>
              <Button
                onClick={save}
                disabled={
                  busy ||
                  dirtyArtifact ||
                  !artifact ||
                  !workspaces.some(w => w.id === workspace) ||
                  (artifact?.kind === "test_plan" || saveTarget ? !!savedCheck : saved)
                }
              >
                {artifact?.kind === "test_plan" ? (savedCheck ? "Saved" : "Keep this brief") : saveTarget
                  ? savedCheck
                    ? "Saved"
                    : "Save this check"
                  : saved
                    ? "Saved"
                    : testJourney
                      ? "Keep these tests"
                      : "Save agent and checks"}
              </Button>
            </>
          ) : (
            <Link
              href={`/auth/login?returnTo=${encodeURIComponent(keepReturnURL(sessionID || "", artifact?.id || "", workspace, saveBaseline))}`}
              onClick={rememberSave}
              className="rounded-lg bg-primary px-4 py-3 text-center text-sm text-primary-foreground"
            >
              Sign in to save your work
            </Link>
          )}
          {savedCheck && (
            <p role="status" className="text-sm">Saved. <a className="underline" href={savedWorkURL(savedCheck)}>Open saved {savedCheck.kind === "brief" ? "brief" : "check"}</a></p>
          )}
          {!saveTarget && savedDraft && (
            <>
              {!saved && (
                <p className="text-xs text-builder-fg-muted">
                  {savedModelNotice}
                </p>
              )}
              <a
                href={
                  testJourney
                    ? savedWorkURL(keptArtifact || { session_id: sessionID!, artifact_id: artifact!.id, workspace_id: savedDraft.workspace_id, baseline_operation_id: saveBaseline || "" })
                    : `/workspaces/${savedDraft.workspace_id}/challenge-packs/builder/${savedDraft.draft_id}`
                }
                className="text-sm underline"
              >
                {testJourney ? "Open saved tests" : "Open your evaluation"}
              </a>
              {testJourney && <Link className="text-sm underline" href={`/workspaces/${savedDraft.workspace_id}/challenge-packs/builder/${savedDraft.draft_id}`}>Open in full pack builder</Link>}
            </>
          )}
        </DialogContent>
      </Dialog>
    </main>
  );
}
