"use client";
import type { RecordedIntakeRequest, RecordedIntakeOutcome, ResultRef } from "./vibe-result-evidence";


import { useVibeCollection } from "@/lib/use-vibe-collection";
import { useVibeInputs } from "@/lib/use-vibe-inputs";
import type { InputBinding } from "@/lib/vibe-inputs";


import { useVibeExport } from "./use-vibe-export";
import {pinVibeReturn} from "./vibe-keep";
import { useVibeSaving } from "@/lib/use-vibe-saving";
import { readEditorDraft, discardEditorDraft, type EditorDraft } from "@/lib/vibe-build-drafts";
import { useVibeEditorDraft } from "./use-vibe-editor-draft";
import { useVibeDrafts } from "@/lib/use-vibe-drafts";
import { useVibeNavigation } from "@/lib/use-vibe-navigation";
import { useVibeSession, sessionAccessLost, isSessionAccessError } from "@/lib/use-vibe-session";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent, captureVibeAttempt } from "@/lib/vibe-build-analytics";

import type { ConversationAction } from "@/lib/vibe-conversation";


import { useSearchParams } from "next/navigation";
import { useAccessToken, useAuth } from "@workos-inc/authkit-nextjs/components";
import { useCallback, useEffect, useRef, useState, type SetStateAction } from "react";



import { workingBuildArtifact } from "@/lib/vibe-build-timeline";
import { evaluationIdentity } from "@/components/vibe/evaluation-navigation";







import { recoverRequest } from "./vibe-request-recovery";
import { editableEvaluation, defaultModels, terminal, retryVibeOperation, VibeError, vibeFetch, type CaseResult, type BuildQuote, type RunQuote, type Models, type Operation, type Session, type VibeConfig, type SavedCheck } from "@/lib/vibe";



type RequestScope = { actor: string; sessionID?: string };
type PendingSubmission = {
  scope: RequestScope;
  sessionID: string;
  readonly body: string;
  composer: string | null;
  composerVersion: number;
  trialKey?: string;
  retryOperationID?: string;
  uncertain: boolean;
};
type SubmissionRecovery = {
  attempt?: object;
  submission: { current: PendingSubmission | null };
  sending: { current: boolean };
  retry: { current: (() => void) | undefined };
  message?: { id: string; content: string };
  error: string;
};
const recoveryKey = (scope: RequestScope) => JSON.stringify([scope.actor, scope.sessionID]);


export function useVibeWorkspace() {

  const params = useSearchParams();
  const navigation = useVibeNavigation();
  const requestedSessionID = navigation.session || null;
  const [loadAttempt, setLoadAttempt] = useState(0);
  const { getAccessToken } = useAccessToken();
  const { loading: authLoading, user: authUser } = useAuth();
  const authUserID = authUser?.id;
  const token = useCallback(async () => {
    if (authLoading || !authUserID) return undefined;
    try { const auth = await getAccessToken(); if (auth) return auth; } catch { /* Report authentication failure without falling back to guest cookies. */ }
    throw new VibeError("unauthorized", "Sign in again to access this account. Your unsent drafts are still here.", 401);
  }, [getAccessToken, authLoading, authUserID]);
  const snapshots = useVibeSession(requestedSessionID, token, authLoading, loadAttempt, authUserID || "guest");
  const { session, accept, select, reload, activeSessionRef, loadingSession, connection, setConnection } = snapshots;
  const drafts = useVibeDrafts(requestedSessionID || undefined);
  const { content, setContent, trialBuffers, setTrialBuffers, composerEdits, trialEdits } = drafts;
  useEffect(() => { drafts.activate(requestedSessionID || undefined); }, [requestedSessionID, drafts.activate]);
  const initializedModels = useRef<{ session: string; thread: string } | undefined>(undefined);
  useEffect(() => {
    if (!session) return;
    const identity = `${authUserID || "guest"}:${session.id}`;
    const previous = initializedModels.current;
    if (previous?.session === identity && previous.thread === navigation.thread) return;
    initializedModels.current = { session: identity, thread: navigation.thread };
    const message = session.document.messages.find(m => m.preview_thread_id === navigation.thread);
    const trial = message && session.operations.find(o => o.id === message.operation_id);
    if (previous?.session !== identity) setModels(trial ? { ...session.document.models, target: trial.models.target } : session.document.models);
    else if (trial) setModels(current => ({ ...current, target: trial.models.target }));
  }, [session, navigation.thread, authUserID]);
  const contextChoice = useRef<{ door: "build" | "test"; id: string } | undefined>(undefined);
  // Choosing the wrong door is reversible until a context has any saved work.
  // The server intentionally keeps context creation append-only; hide the empty
  // abandoned child locally instead of deleting history or changing its API.
  const entryOrigin = useRef<string | undefined>(undefined);
  const discardedContextIDs = useRef(new Set<string>());

  const [newEvaluation, setNewEvaluation] = useState(false);
  const [savedWorkOpen, setSavedWorkOpen] = useState(false);
  const [buildQuote, setBuildQuote] = useState<BuildQuote>();
  const [quoteError, setQuoteError] = useState("");
  const [quoteAttempt, setQuoteAttempt] = useState(0);
  const [quotePreparing, setQuotePreparing] = useState(false);
  const [clock, setClock] = useState(Date.now());
  const serverOffset = useRef(0);
  const syncClock = (value?: string) => { if (value && Number.isFinite(Date.parse(value))) serverOffset.current = Date.parse(value) - Date.now(); };
  useEffect(() => { const timer = setInterval(() => setClock(Date.now()), 1000); return () => clearInterval(timer); }, []);
  const serverNow = clock + serverOffset.current;
  const [runQuotePreparing, setRunQuotePreparing] = useState(false);
  const [runQuoteError, setRunQuoteError] = useState("");
  const [runQuote, setRunQuote] = useState<{ quote: RunQuote; recordedArtifactID?: string; selectionArtifact?: string; requestModels: Models; kind: string; baseline?: Operation; extra: { case_keys?: string[]; retry_source_result_ref?: ResultRef; client_id?: string; cycle_id?: string; additional_examples?: number; evidence_set_id?: string; artifact_id?: string; purpose?: "regrade" }; content?: string; sessionID: string; revision: number }>();
  const [config, setConfig] = useState<VibeConfig | null>(null);
  const [configError, setConfigError] = useState(false);
  // Loading configuration must not render the legacy entry before the V1 shell.
  const twoDoor = config === null ? true : config.two_door;
  const [models, setModels] = useState<Models>(defaultModels);
  const [pending, setPending] = useState(false);
  const [pendingMessage, renderPendingMessage] = useState<{
    id: string;
    content: string;
  }>();
  const [runPending, setRunPending] = useState(false);
  const requestedRunID = navigation.run;
  const [pendingAction, setPendingAction] = useState<string>();
  const [error, renderError] = useState("");
  const [importError, setImportError] = useState<{ sessionID?: string; message: string }>();
  const importNotice = useRef<HTMLDivElement>(null);
  const quoteTrigger = useRef<HTMLElement | null>(null);
  const quoteHeading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (importError && !newEvaluation) {
      importNotice.current?.focus();
      importNotice.current?.scrollIntoView?.({ block: "nearest" });
    }
  }, [importError, newEvaluation]);
  const view = navigation.legacyView;
  const selectedArtifactID = navigation.artifact;
  const setSelectedArtifactID = (artifact: string | null) => navigation.update({ artifact }, "replace");
  const threadID = navigation.thread;
  const setThreadID = (thread: string) => navigation.update({ thread }, "replace");
  const buildEvidence = useRef<
    { operation: string; artifact: string } | undefined
  >(undefined);
  const savedCaseReads = useRef(new Map<string, Promise<CaseResult>>());
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [checksDirtyID, setChecksDirtyID] = useState<string | null>(null);
  const [pendingEdit, setPendingEdit] = useState<{ operationID: string; artifactID: string; draft?: EditorDraft }>();
  const [dirtyArtifactID, setDirtyArtifactID] = useState<string | null>(null);

  const workspaceActor = authUserID || "guest";
  const workspaceActorRef = useRef(workspaceActor); workspaceActorRef.current = workspaceActor;
  const [workspaceRow, setWorkspaceRow] = useState({ actor: workspaceActor, value: params.get("workspace") || "" });
  const workspace = workspaceRow.actor === workspaceActor ? workspaceRow.value : "";
  const setWorkspace = useCallback((next: SetStateAction<string>) => setWorkspaceRow(old => {
    if (workspaceActorRef.current !== workspaceActor) return old;
    const previous = old.actor === workspaceActor ? old.value : "";
    return { actor: workspaceActor, value: typeof next === "function" ? next(previous) : next };
  }), [workspaceActor]);
  const [archiveMode,setArchiveMode]=useState<"exclude"|"only"|"all">("exclude");
 const listScope = `${authUserID || "guest"}:${workspace}`;
  const contextList = useVibeCollection<Session>(`/sessions?archived=${archiveMode}${workspace ? `&workspace=${encodeURIComponent(workspace)}` : ""}`, `${listScope}:${archiveMode}`, token, !authLoading && !!config?.two_door, !!authUserID);
  const savedList = useVibeCollection<SavedCheck>(`/saved-checks${workspace ? `?workspace=${encodeURIComponent(workspace)}` : ""}`, listScope, token, !authLoading, true);
  const { rows: contexts, setRows: setContexts } = contextList;
  const { rows: savedChecks, setRows: setSavedChecks } = savedList;

  const currentView = useRef(view); currentView.current = view;
  const [uncertain, setUncertain] = useState(false);
  const scopeRef = useRef<RequestScope>({ actor: workspaceActor, sessionID: requestedSessionID || undefined });
  if (scopeRef.current.actor !== workspaceActor || scopeRef.current.sessionID !== (requestedSessionID || undefined))
    scopeRef.current = { actor: workspaceActor, sessionID: requestedSessionID || undefined };
  const requestScope = scopeRef.current;
  const ownsScope = (scope: RequestScope) => scopeRef.current === scope;
  // Request ownership survives navigation; only its visible projection changes.
  // Each render's async closures retain their own actor/project record.
  const recoveries = useRef(new Map<string, SubmissionRecovery>());
  const key = recoveryKey(requestScope);
  const retained = recoveries.current.get(key);
  const recovery: SubmissionRecovery = retained || { submission: { current: null }, sending: { current: false }, retry: { current: undefined }, error: "" };
  if (!retained) recoveries.current.set(key, recovery);
  const visibleRecovery = useCallback(() => recoveries.current.get(recoveryKey(scopeRef.current)) === recovery, [recovery]);
  const submission = recovery.submission;
  const sending = recovery.sending;
  const retryUnsentMessage = recovery.retry;
  const setPendingMessage = useCallback((message?: { id: string; content: string }) => {
    recovery.message = message;
    if (visibleRecovery()) renderPendingMessage(message);
  }, [recovery, visibleRecovery]);
  const setError = useCallback((next: SetStateAction<string>) => {
    recovery.error = typeof next === "function" ? next(recovery.error) : next;
    if (visibleRecovery()) renderError(recovery.error);
  }, [recovery, visibleRecovery]);
  useEffect(() => {
    const request = submission.current;
    contextChoice.current = undefined;
    setPending(sending.current); setRunPending(sending.current && ["check", "retest"].includes(request ? JSON.parse(request.body).kind : ""));
    setUncertain(!!request?.uncertain); setPendingMessage(recovery.message);
    setPendingAction(sending.current && request?.retryOperationID ? "Retrying your request…" : undefined);
    setPendingEdit(undefined); setError(recovery.error); setRunQuote(undefined); setRunQuotePreparing(false); setRunQuoteError("");
  }, [requestScope]);
  const file = useRef<HTMLInputElement>(null);
  const active = session?.operations.find((o) => !terminal(o.state));
  const sessionUnavailable =
    !!requestedSessionID && session?.id !== requestedSessionID;
  const funding = config?.local_testing && !config.free_only ? "Local operator budget" : session ? session.anonymous ? "Guest trial allowance" : session.workspace_id ? "Workspace credits" : "Workspace required" : authUserID ? workspace ? "Workspace credits" : "Workspace required" : "Guest trial allowance";
  const fundingKey = session ? session.anonymous ? "guest" : "workspace" : authUserID ? "workspace" : "guest";
  const inferenceReady = !!config?.enabled && !!config.models?.length && config.availability?.[fundingKey]?.available === true && funding !== "Workspace required";
  const inferenceAvailable = inferenceReady && Object.values(models).every(id => config!.models.some(model => model.id === id));
  const unavailableReason = !config ? "Connecting…" : !config.models?.length ? "No approved models are available." : funding === "Workspace required" ? "Choose a workspace to use its credits." : config.availability?.[fundingKey]?.reason_code === "trial_capacity_reached" ? "Guest inference capacity is currently reached. Your saved work remains available." : config.availability?.[fundingKey]?.reason_code === "guest_not_configured" ? "Guest inference isn’t configured here. Your saved work remains available." : config.availability?.[fundingKey]?.reason_code === "worker_unavailable" ? "Inference is temporarily unavailable while workers reconnect." : "New inference is unavailable. You can still review and export saved work.";
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
  const pinnedRun = session?.operations.find(item => item.id === requestedRunID);
  const pinnedArtifactID = pinnedRun?.source?.artifact_id || pinnedRun?.results[0]?.version;
  const artifact = pinnedArtifactID ? artifacts.find(item => item.id === pinnedArtifactID) : selectedArtifactID ? artifacts.find((a) => a.id === selectedArtifactID) : session?.document.evaluation?.door === "build" ? workingArtifact : latestArtifact;
  function evaluationAvailable(baseline?: Operation, purpose?: string) {
    if (purpose === "regrade" || baseline?.source?.kind === "provided_conversations" || artifact?.conversation_evaluation) {
      return inferenceReady && !!config?.models.some(model=>model.id===(baseline?.models.evaluator||models.evaluator));
    }
    return inferenceAvailable;
  }
  const runInferenceAvailable = evaluationAvailable(runQuote?.baseline, runQuote?.extra.purpose);
  const dirtyArtifact =
    !!artifact &&
    (!!readEditorDraft(session!.id,"intake","recorded-request")?.value || dirtyArtifactID === artifact.id || checksDirtyID === artifact.id || !!artifact.conversation_evaluation && (() => {
      const draft = readEditorDraft(session!.id, artifact.id, "recorded-expectations");
      return !!draft && draft.value !== artifact.conversation_evaluation.expectations.map(rule=>rule.statement).join("\n");
    })());
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
  useEffect(() => {
    if (!session || session.id !== requestedSessionID) return;
    const patch: Parameters<typeof navigation.update>[0] = {};
    if (navigation.artifact && !session.document.artifacts.some(item => item.id === navigation.artifact)) patch.artifact = null;
    const run = session.operations.find(item => item.id === navigation.run && (item.kind === "check" || item.kind === "retest"));
    if (navigation.run && !run) { patch.run = undefined; patch.case = undefined; }
    const tested = run?.source?.artifact_id || run?.results[0]?.version;
    if (tested && session.document.artifacts.some(item => item.id === tested) && navigation.artifact !== tested) patch.artifact = tested;
    const cases = (artifact?.blueprint as { cases?: { key?: string }[] } | undefined)?.cases || [];
    if (navigation.case && !cases.some(item => item.key === navigation.case) && !run?.results.some(item => item.case_key === navigation.case)) patch.case = undefined;
    if (threadID && !sending.current && !trialHistory.some(item => item.id === threadID) && !Object.hasOwn(trialBuffers, trialDraftKey) && !submission.current?.trialKey?.endsWith(`:${threadID}`)) patch.thread = "";
    if (Object.keys(patch).length) navigation.update(patch, "replace");
  }, [session, requestedSessionID, navigation.artifact, navigation.run, navigation.case, threadID, artifact]);
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
    navigation.update({ view: next, artifact: agent || null, thread, ...(agent && agent !== artifact?.id ? { run: undefined, case: undefined } : {}) });
  }
  function newTrial() {
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
  const recordedUpdateDraft = useVibeEditorDraft(sessionID || "", "intake", "recorded-baseline", "recorded-v1", "");
  let recordedBinding: { baseline_id: string; client_id?: string; artifact_id?: string } | undefined;
  try { if (recordedUpdateDraft.value) recordedBinding = JSON.parse(recordedUpdateDraft.value); } catch { /* Preserve an unreadable draft; never guess its baseline. */ }
  const recordedBaseline = session?.operations.find(op => op.id === recordedBinding?.baseline_id && terminal(op.state) && op.source?.kind === "provided_conversations");
  const recordedUpdate = recordedBaseline ? { baseline: recordedBaseline, artifactID: recordedBinding?.artifact_id } : undefined;
  function startRecordedUpdate(baseline: Operation) {
    if (!session || busy || dirtyArtifact || !terminal(baseline.state) || baseline.source?.kind !== "provided_conversations") return;
    recordedUpdateDraft.setValue(JSON.stringify({ baseline_id: baseline.id }));
    setRunQuote(undefined);
  }
  function closeRecordedUpdate() {
    // A successful intake keeps its baseline binding for the explicit quote.
    const latest = readEditorDraft(sessionID || "", "intake", "recorded-baseline");
    if (readEditorDraft(sessionID || "", "intake", "recorded-request")?.value) return;
    try { if (latest && !JSON.parse(latest.value).artifact_id) recordedUpdateDraft.discard(latest.value); } catch { /* Retain the unreadable draft. */ }
  }
  const attachedWorkspace = session?.workspace_id;
  useEffect(() => {
    if (attachedWorkspace) setWorkspace(attachedWorkspace);
  }, [attachedWorkspace]);

  useEffect(() => {
    let alive = true;
    setConfigError(false);
    vibeFetch<VibeConfig>("/config")
      .then((c) => {
        if (alive) {
          syncClock(c.server_time); setConfig(c);
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
  async function ensureSession(scope = scopeRef.current, ownsAttempt = () => true) {
    if (requestedSessionID && session?.id !== requestedSessionID)
      throw new Error(
        "Load this conversation before sending. No new conversation was created.",
      );
    if (session) return session;
    const id = crypto.randomUUID();
    const auth = await token();
    if (!ownsScope(scope) || !ownsAttempt()) throw new Error("The active account or project changed.");
    await vibeFetch<Session>("/sessions", auth, {
      method: "POST",
      body: JSON.stringify({
        id,
        ...(workspace ? { workspace_id: workspace } : {}),
      }),
    });
    if (!ownsScope(scope) || !ownsAttempt()) throw new Error("The active account or project changed.");
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
    if (!ownsScope(scope) || !ownsAttempt()) throw new Error("The active account or project changed.");
    recoveries.current.delete(recoveryKey(scope));
    scope.sessionID = v.id;
    recoveries.current.set(recoveryKey(scope), recovery);
    select(v.id); if (!accept(v)) throw new Error("The active account changed. Reopen this project to continue."); drafts.activate(v.id);
    navigation.writeURL(new URL(`/vibe-evals?session=${id}${workspace ? `&workspace=${workspace}` : ""}`, window.location.origin));
    return v;
  }
  async function metadata(target:Session,patch:{title?:string;archived?:boolean}){
    const captured=scopeRef.current;const auth=await token();if(!ownsScope(captured))return false;
    const result=await vibeFetch<Session>(`/sessions/${target.id}/metadata`,auth,{method:"PATCH",body:JSON.stringify({revision:target.revision,...patch})});if(!ownsScope(captured))return false;
    if(target.id===session?.id)accept(result);setContexts(old=>old.map(row=>row.id===result.id?result:row));contextList.retry();return true;
  }
  const exporter=useVibeExport(workspaceActor,session,token);
  const selectedExportRun=requestedRunID?session?.operations.find(o=>o.id===requestedRunID):session?.operations.findLast(o=>(o.kind==="check"||o.kind==="retest")&&(o.result_scope?.artifact_id||o.source?.artifact_id||o.results[0]?.version)===artifact?.id);
  const projectReturnPath=session&&artifact?pinVibeReturn(`/vibe-evals?${params.toString()}`,session.id,artifact.id,selectedExportRun?.id):undefined;
  function exportConversation(){if(!artifact)return;setSettingsOpen(false);void exporter.start(artifact,selectedExportRun)}
  function loadSavedCase(id: string, key: string): Promise<CaseResult> {
    const owner = session?.id;
    if (!owner || activeSessionRef.current !== owner)
      return Promise.reject(new Error("This conversation changed. Reopen its saved checks."));
    const cacheKey = JSON.stringify([workspaceActor, owner, id, key]);
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
  const materialScope = "guide";
  const materials = useVibeInputs(sessionID || "", materialScope, token, workspaceActor);
  const trialMaterials = useVibeInputs(sessionID || "", `trial:${artifact?.id || ""}:${threadID}`, token, workspaceActor);
  const materialFingerprint = JSON.stringify(materials.bindings);
  const adoptionFingerprint = JSON.stringify(materials.adoptions);
  const messageLimit = (session?.anonymous === false ? config?.signed_in_limits : config?.anonymous_limits)?.max_message_bytes || (session?.anonymous === false ? 65536 : 16384);
  const messageError = new TextEncoder().encode(content).length > messageLimit ? `Message exceeds the ${messageLimit.toLocaleString()} byte UTF-8 limit.` : "";
  const buildStart = session?.document.evaluation?.door === "build" && !session.document.build && !session.document.artifacts.length;
  const continuingBuild = (session?.document.build?.phase === "clarifying" || session?.document.build?.phase === "waiting");
  const quoteMatches = !!buildQuote && buildQuote.request.content === content && JSON.stringify(buildQuote.request.inputs || []) === materialFingerprint && JSON.stringify(buildQuote.request.adopt_rules || []) === adoptionFingerprint && JSON.stringify(buildQuote.request.models) === JSON.stringify(models) && Date.parse(buildQuote.expires_at) > serverNow;
  useEffect(() => {
    if (!inferenceAvailable || !buildStart || !sessionID || !content.trim() || new TextEncoder().encode(content).length > messageLimit) { setBuildQuote(undefined); setQuoteError(""); return; }
    let live = true;
    setQuoteError(""); setQuotePreparing(true);
    const timer = setTimeout(() => {
      captureVibeAttempt("build", "estimate", sessionID);
      void token().then(auth => vibeFetch<BuildQuote>(`/sessions/${sessionID}/build-quote`, auth, { method: "POST", body: JSON.stringify({ content, models, ...(materials.bindings.length ? { inputs: materials.bindings } : {}), ...(materials.adoptions.length ? { adopt_rules: materials.adoptions } : {}) }) }))
        .then(q => { if (live) { syncClock(q.server_time); setClock(Date.now()); setBuildQuote(q); } }).catch(e => { captureVibeAttempt("build", "estimate", sessionID, e instanceof VibeError ? e.code : "transport_failed"); if (live) setQuoteError(e.message); }).finally(() => { if (live) setQuotePreparing(false); });
    }, 400);
    return () => { live = false; clearTimeout(timer); };
  }, [buildStart, sessionID, content, models, token, materialFingerprint, adoptionFingerprint, quoteAttempt, inferenceAvailable]);
  const buildQuoteState = quoteError ? "failure" : quotePreparing || !buildQuote ? "preparation" : !quoteMatches ? "expired" : "current";
  const refreshBuildQuote = () => setQuoteAttempt(attempt => attempt + 1);
  const quoteScopeMatches = !!runQuote && runQuote.selectionArtifact === artifact?.id && JSON.stringify(runQuote.requestModels) === JSON.stringify(runQuote.baseline ? { ...models, evaluator: runQuote.baseline.models.evaluator } : models);
  const isRunQuoteCurrent = () => !!runQuote && quoteScopeMatches && !runQuoteError && !runQuotePreparing && runQuote.sessionID === activeSessionRef.current && runQuote.revision === session?.revision && Date.parse(runQuote.quote.expires_at) > Date.now() + serverOffset.current;
  const runQuoteCurrent = !!runQuote && quoteScopeMatches && !runQuoteError && !runQuotePreparing && runQuote.sessionID === sessionID && runQuote.revision === session?.revision && Date.parse(runQuote.quote.expires_at) > serverNow;
  const runQuoteState = runQuotePreparing ? "preparation" : runQuoteError ? "failure" : runQuote ? runQuoteCurrent ? "current" : "expired" : undefined;
  function adoptContext(v: Session) {
    setNewEvaluation(false);
    select(v.id);
    if (!accept(v)) return;
    drafts.activate(v.id); contextList.retry(); setModels(v.document.models);
    const selected = navigation.selectSession(v.id);
    setError(""); setImportError(undefined); setConnection(""); setPendingMessage(undefined); setPendingEdit(undefined);
    setChecksDirtyID(null); setDirtyArtifactID(null); setBuildQuote(undefined); setRunQuote(undefined);
    buildEvidence.current = undefined;

  }
  async function chooseDoor(door: "build" | "test") {
    const scope = scopeRef.current;
    captureVibeAttempt(door, "entry", sessionID);
    if (!entryAvailable || sending.current || pending || uncertain) return;
    setPending(true);
    try {
      entryOrigin.current ||= sessionID;
      if (!contextChoice.current || contextChoice.current.door !== door)
        contextChoice.current = { door, id: crypto.randomUUID() };
      const auth = await token();
      if (!ownsScope(scope)) return;
      const id = contextChoice.current!.id;
      await vibeFetch<Session>("/sessions", auth, { method: "POST", body: JSON.stringify({ id, door, ...(workspace ? { workspace_id: workspace } : {}) }) });
      if (!ownsScope(scope)) return;
      // Confirm that private-cookie ownership survived before adopting the URL.
      const agent = await vibeFetch<Session>(`/sessions/${id}`, auth);
      if (!ownsScope(scope)) return;
      const draft = canChangeDoor ? content : "";
      if (canChangeDoor && sessionID) discardedContextIDs.current.add(sessionID);
      adoptContext(agent);
      setContent(draft);
      contextChoice.current = undefined;
      if (door === "build") captureBuildEvent(WEB_EVENTS.VIBE_BUILD_ENTRY_SELECTED, { session_id: agent.id, entry_source: "two_door" }, agent.id);
    } catch (e) { if (!ownsScope(scope)) return; captureVibeAttempt(door, "entry", sessionID, e instanceof VibeError ? e.code : "transport_failed"); setError((e as Error).message); }
    finally { if (ownsScope(scope)) { setPending(false); } }
  }
  async function switchContext(id: string) {
    const scope = scopeRef.current;
    if (id === sessionID || pending || uncertain) return;
    setPending(true);
    try { const next = await vibeFetch<Session>(`/sessions/${id}`, await token()); if (ownsScope(scope)) adoptContext(next); }
    catch (e) { if (!ownsScope(scope)) return; setError((e as Error).message); }
    finally { if (ownsScope(scope)) { setPending(false); } }
  }
  function openNewEvaluation() {
    setImportError(undefined);
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
    const scope = scopeRef.current;
    if (!canChangeDoor || !sessionID) return;
    const origin = entryOrigin.current;
    if (!origin || origin === sessionID) {
      setImportError(undefined);
      setNewEvaluation(true);
      return;
    }
    setPending(true);
    try {
      const previous = await vibeFetch<Session>(`/sessions/${origin}`, await token());
      if (!ownsScope(scope)) return;
      discardedContextIDs.current.add(sessionID);
      adoptContext(previous);
      setNewEvaluation(true);
    } catch (e) { if (!ownsScope(scope)) return;
      setError((e as Error).message);
    } finally { if (ownsScope(scope)) {
      setPending(false);
    } }
  }
  function sendMessage(text = content, context?: { viewed_run_id?: string; viewed_case_key?: string; artifact_id?: string }, demoID?: string) {
    if (buildStart && (!quoteMatches || Date.parse(buildQuote!.expires_at) <= Date.now() + serverOffset.current)) return;
    if (materials.blocked && buildStart) return;
    void submit("message", text, undefined, {
      ...context,
      ...(demoID ? { demo_id: demoID } : {}),
      ...(buildStart ? { cycle_id: buildQuote!.id } : continuingBuild ? { cycle_id: session!.document.build!.cycle_id } : {}),
      ...(testJourney || session?.document.evaluation ? {} : { quick_check: !buildEvidence.current }),
    });
  }
  async function requestPreparation(text: string, count: number, artifactID: string) {
    const scope = scopeRef.current;
    if (!session || busy || !inferenceAvailable) return;
    quoteTrigger.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const clientID = crypto.randomUUID();
    const extra = { client_id: clientID, additional_examples: count, artifact_id: artifactID };
    setPending(true); setError(""); setRunQuotePreparing(true); setRunQuoteError("");
    captureVibeAttempt(session.document.evaluation?.door || "test", "estimate", session.id);
    try {
      const quote = await vibeFetch<BuildQuote>(`/sessions/${session.id}/build-quote`, await token(), {method:"POST", body:JSON.stringify({ content:text, models, artifact_id:artifactID, additional_examples:count })});
      if (!ownsScope(scope)) return;
      syncClock(quote.server_time); setClock(Date.now());
      setRunQuote({selectionArtifact: artifact?.id, requestModels: models, quote: {...quote, calls: quote.max_calls}, kind:"message", content:text, extra: {...extra, cycle_id:quote.id}, sessionID:session.id, revision:session.revision});
    } catch(e) { if (!ownsScope(scope)) return; setRunQuoteError((e as Error).message); setError((e as Error).message); captureVibeAttempt(session.document.evaluation?.door || "test", "estimate", session.id, e instanceof VibeError ? e.code : "transport_failed"); } finally { if (ownsScope(scope)) {setPending(false); setRunQuotePreparing(false);} }
  }
  async function requestRun(baseline?: Operation, evidenceID?: string, purpose?: "regrade", selection?: { case_keys: string[]; retry_source_result_ref: ResultRef; artifact_id: string }, recordedArtifactID?: string) {
    const scope = scopeRef.current;
    if (!evaluationAvailable(baseline,purpose) || dirtyArtifact) return;
    if (recordedArtifactID && (!baseline || recordedUpdate?.baseline.id !== baseline.id || recordedUpdate.artifactID !== recordedArtifactID || artifact?.id !== recordedArtifactID || artifact.conversation_evaluation?.evidence_set_id !== evidenceID)) return;
    quoteTrigger.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const extra = { ...selection, artifact_id: selection?.artifact_id || artifact?.id, ...(purpose ? {purpose} : {}), ...(evidenceID ? { evidence_set_id: evidenceID } : {}), ...(baseline?.source?.kind === "provided_conversations" ? { artifact_id: recordedArtifactID || baseline.source.artifact_id } : {}) };
    const kind = baseline ? "retest" : "check";
    if (!session?.document.evaluation) { void submit(kind, "", baseline, extra); return; }
    setPending(true); setRunQuotePreparing(true); setRunQuoteError("");
    captureVibeAttempt(session.document.evaluation?.door || "test", "estimate", session.id);
    try {
      const q = await vibeFetch<RunQuote>(`/sessions/${session.id}/run-quote`, await token(), { method: "POST", body: JSON.stringify({ client_id: crypto.randomUUID(), revision: session.revision, kind, models: baseline ? {...models, evaluator: baseline.models.evaluator} : models, baseline_id: baseline?.id, approve_artifact: true, ...extra }) });
      if (!ownsScope(scope)) return;
      syncClock(q.server_time); setClock(Date.now());
      setRunQuote({ recordedArtifactID, selectionArtifact: artifact?.id, requestModels: baseline ? { ...models, evaluator: baseline.models.evaluator } : models, quote: q, kind, baseline, extra, sessionID: session.id, revision: session.revision });
    } catch (e) { if (!ownsScope(scope)) return; setRunQuoteError((e as Error).message); setError((e as Error).message); captureVibeAttempt(session.document.evaluation?.door || "test", "estimate", session.id, e instanceof VibeError ? e.code : "transport_failed"); }
    finally { if (ownsScope(scope)) { setPending(false); setRunQuotePreparing(false); } }
  }
  async function submit(
    kind = "message",
    text = content,
    baseline?: Operation,
    extra: {
      case_keys?: string[];
      retry_source_result_ref?: ResultRef;
      inputs?: InputBinding[];
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
    if (sending.current || busy || submission.current || !(["check","retest"].includes(kind) ? evaluationAvailable(baseline,extra.purpose) : inferenceAvailable)) return;
    if (new TextEncoder().encode(text).length > messageLimit) { setError(`Message exceeds the ${messageLimit.toLocaleString()} byte UTF-8 limit.`); return; }
    if (kind === "message" && !text.trim()) return;
    if (
      dirtyArtifact ||
      (kind !== "message" && (!artifact || artifact.kind === "test_plan"))
    )
      return;
    let trialKey: string | undefined;
    let previewThread: string | undefined;
    if (kind === "playground") {
      if ((!text.trim() && !trialMaterials.bindings.length) || trialMaterials.blocked || legacyTrial) return;
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
        navigation.update({ thread: previewThread }, "replace");
      }
    }
    const composerVersion = trialKey
      ? trialEdits.current[trialKey] || 0
      : composerEdits.current;
    const capturedAdoptions = kind === "message" && buildStart ? materials.adoptions : [];
    const capturedInputs = kind === "playground" ? trialMaterials.bindings : kind === "message" && buildStart ? materials.bindings : [];
    const clientID = extra.client_id || crypto.randomUUID();
    const attempt = {};
    recovery.attempt = attempt;
    const ownsAttempt = () => recovery.attempt === attempt;
    let dispatched = false;
    if (kind === "message") {
      retryUnsentMessage.current = undefined;
      setPendingMessage({ id: clientID, content: text });
      // Move the submitted text into the conversation immediately. The version
      // check below protects anything the user types while admission is pending.
      if (!extra.demo_id) setContent((current) => (current === text ? "" : current));
    }
    const scope = scopeRef.current;
    captureVibeAttempt(session?.document.evaluation?.door || "test", kind, sessionID);
    sending.current = true;
    if (kind === "check" || kind === "retest") setRunPending(true);
    setPending(true);
    setError("");
    try {
      const v = await ensureSession(scope, ownsAttempt);
      if (!ownsAttempt()) return;
      if (!ownsScope(scope)) throw new Error("The active account or project changed.");
      submission.current = {
        scope,
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
          ...(capturedInputs.length ? { inputs: capturedInputs } : {}),
          ...(capturedAdoptions.length ? { adopt_rules: capturedAdoptions } : {}),
          models: baseline
            ? { ...models, evaluator: baseline.models.evaluator }
            : models,
          ...((kind === "message" && workingArtifact) ? { artifact_id: workingArtifact.id } : artifact ? { artifact_id: artifact.id } : {}),
          ...(baseline ? { baseline_id: baseline.id } : {}),
          ...(kind === "message" && (!twoDoor || !extra.viewed_run_id && !extra.viewed_case_key) &&
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
      dispatched = true;
      await dispatchSubmission();
      if (!ownsScope(scope)) return;
      if (kind === "message" && currentView.current === "build") {
        setSelectedArtifactID(null);
        setThreadID("");
        const url = new URL(window.location.href);
        url.searchParams.delete("agent");
        url.searchParams.delete("thread");
        navigation.writeURL(url);
      }
    } catch (e) {
      captureVibeAttempt(session?.document.evaluation?.door || "test", kind, sessionID, e instanceof VibeError ? e.code : "transport_failed");
      if (!ownsAttempt()) return;
      // Preadmission failures belong to the retained owner even when inactive.
      setError((e as Error).message);
      if (!dispatched && kind === "message" && !extra.demo_id) {
        retryUnsentMessage.current = () => {
          if (visibleRecovery() && !sending.current) void submit(kind, text, baseline, extra);
        };
      }
      if (!visibleRecovery()) return;
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
      if (ownsAttempt()) {
        sending.current = false;
        recovery.attempt = undefined;
        if (visibleRecovery()) { setPending(false); setRunPending(false); }
      }
    }
  }
  async function dispatchSubmission() {
    const request = submission.current;
    if (!request || !visibleRecovery()) return;
    const attempt = recovery.attempt;
    const ownsRequest = () => recovery.attempt === attempt && submission.current === request;
    try {
      let admitted: Operation;
      try {
        const auth = await token();
        if (!ownsRequest()) return;
        if (scopeRef.current.actor !== request.scope.actor) {
          request.uncertain = true;
          setError("The account changed before admission. Return to this account and retry your saved request.");
          return;
        }
        admitted = request.retryOperationID
          ? await retryVibeOperation(request.sessionID, request.retryOperationID, JSON.parse(request.body), auth)
          : await vibeFetch<Operation>(`/sessions/${request.sessionID}/messages`, auth, { method: "POST", body: request.body });
      } catch (e) {
        if (!ownsRequest()) return;
        // Settle only this actor/project's record, even when it is inactive.
        const rejected = recoverRequest(request, e) === "rejected";
        if (rejected) {
          submission.current = null;
          retryUnsentMessage.current = () => {
            if (!visibleRecovery() || sending.current) return;
            submission.current = request;
            void retrySubmission();
          };
        }
        setError((e as Error).message);
        if (visibleRecovery()) {
          setUncertain(!rejected);
          if (e instanceof VibeError && (e.code === "revision_conflict" || (request.retryOperationID && rejected)))
            await reload(request.sessionID).catch(() => undefined);
        }
        throw e;
      }
      if (!ownsRequest()) return;
      submission.current = null;
      retryUnsentMessage.current = undefined;
      setError("");
      if (!visibleRecovery()) { setPendingMessage(undefined); return; }
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
      if (requestKind === "check" || requestKind === "retest") {
        navigation.update({ run: admitted.id }, "replace");
      }
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
        if (visibleRecovery() && activeSessionRef.current === request.sessionID) setConnection(
          isSessionAccessError(error)
            ? sessionAccessLost
            : "Reconnecting to saved progress…",
        );
        return undefined;
      });
      if (visibleRecovery() && fresh && !request.trialKey && request.composer !== null)
        setPendingMessage(undefined);
    } finally {
      // A late response must not release a newer request for this owner.
      if (recovery.attempt === attempt && (!submission.current || submission.current === request)) {
        sending.current = false;
        if (visibleRecovery()) { setPending(false); setRunPending(false); setPendingAction(undefined); }
      }
    }
  }

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
    if (!request?.uncertain || !visibleRecovery() || !request.retryOperationID || session?.id !== request.sessionID) return;
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
      if (pendingEdit.draft && session) discardEditorDraft(session.id, pendingEdit.artifactID, "cases", pendingEdit.draft);
      navigation.update({ artifact: artifactID, run: undefined, case: undefined, thread: "" }, "replace");
      if (currentView.current === "tests" && document.activeElement?.closest(`[data-proposal-id="${CSS.escape(pendingEdit.artifactID)}"]`)) requestAnimationFrame(() => document.querySelector<HTMLHeadingElement>(`[data-proposal-id="${CSS.escape(artifactID)}"] h2`)?.focus());
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
    const attempt = {};
    recovery.attempt = attempt;
    const ownsAttempt = () => recovery.attempt === attempt;
    sending.current = true;
    setPending(true);
    if (submission.current.retryOperationID) setPendingAction("Retrying your request…");
    setError("");
    try {
      await dispatchSubmission();
    } catch (e) {
      captureVibeAttempt(session?.document.evaluation?.door || "test", "retry", sessionID, e instanceof VibeError ? e.code : "transport_failed");
      if (!ownsAttempt() || !visibleRecovery()) return;
      setError((e as Error).message);
    } finally {
      if (ownsAttempt()) {
        sending.current = false;
        recovery.attempt = undefined;
        if (visibleRecovery()) { setPending(false); setPendingAction(undefined); }
      }
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
      scope: scopeRef.current,
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
    const scope = scopeRef.current;
    if (!session) return;
    setPending(true);
    try {
      const next = await vibeFetch<Session>(`/sessions/${session.id}/actions`, await token(), {
        method: "POST", body: JSON.stringify({ version: 1, kind: "action", payload: action }),
      });
      if (!ownsScope(scope) || !accept(next)) return;
      const restored = next.document.artifacts.at(-1);
      if (action.kind === "undo" && restored) {
        setSelectedArtifactID(restored.id);
        navigate("build", restored.id);
      }
    } finally { if (ownsScope(scope)) { setPending(false); } }
  }
  async function reloadChoices() {
    if (session) await reload(session.id);
  }

  async function addRecordedEvidence(request: RecordedIntakeRequest): Promise<RecordedIntakeOutcome> {
    const scope=scopeRef.current;
    if(!session || pending || busy) return "uncertain";
    const id=session.id;
    // Bind the baseline before transport; a lost acknowledgment retries this receipt.
    const intakeBinding = recordedBinding && !recordedBinding.artifact_id ? { ...recordedBinding, client_id: request.client_id } : undefined;
    if (intakeBinding) recordedUpdateDraft.setValue(JSON.stringify(intakeBinding));
    setPending(true);setError("");
    try {
      const next=await vibeFetch<Session>(`/sessions/${id}/evidence`,await token(),{method:"POST",body:JSON.stringify(request)});
      if(!ownsScope(scope))return "uncertain";
      const receipt=next.evidence_receipt;
      if(!receipt || receipt.client_id!==request.client_id || !next.document.artifacts.some(item=>item.id===receipt.artifact_id && item.conversation_evaluation?.evidence_set_id===receipt.evidence_id))throw Error("The exact saved reply receipt is unavailable. Retry the same request.");
      if(!accept(next))return "uncertain";
      if(intakeBinding) recordedUpdateDraft.setValue(JSON.stringify({...intakeBinding,artifact_id:receipt.artifact_id}));
      setSelectedArtifactID(receipt.artifact_id);navigate("tests",receipt.artifact_id,"");return "saved";
    } catch(e) {
      if(!ownsScope(scope))return "uncertain";
      setError((e as Error).message);
      return e instanceof VibeError && !!e.status && e.status>=400 && e.status<500 ? "rejected" : "uncertain";
    } finally { if(ownsScope(scope))setPending(false); }
  }

  async function edit(fields: Record<string, unknown>) {
    const scope = scopeRef.current;
    if (!session) return false;
    const submittedDraft = fields.case_changes !== undefined ? readEditorDraft(session.id, String(fields.artifact_id || artifact?.id || ""), "cases") : undefined;
    if (!inferenceAvailable && (fields.case_changes !== undefined || fields.build_from_policy === true)) { setError(unavailableReason); return false; }
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
      if (!ownsScope(scope) || !accept(v)) return false;
      const suiteChange = fields.case_changes !== undefined || fields.criteria !== undefined || fields.evaluation !== undefined;
      const admittedEdit = suiteChange && v.operations.find(operation => !session.operations.some(previous => previous.id === operation.id));
      if (admittedEdit && !admittedEdit.completion_receipt) {
        setPendingEdit({ operationID: admittedEdit.id, artifactID: String(fields.artifact_id || artifact?.id || ""), draft: submittedDraft });
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
          navigate(currentView.current, next.id, "");
        }
        setThreadID("");
        setDirtyArtifactID(null);
        setChecksDirtyID(null);
      }
      return true;
    } catch (e) { if (!ownsScope(scope)) return false;
      setError((e as Error).message);
      await reload();
      return false;
    } finally { if (ownsScope(scope)) {
      setPending(false);
    } }
  }
  async function operationAction(id: string, action: "stop" | "approve") {
    const scope = scopeRef.current;
    if (action === "stop" && buildJourney && session) captureBuildEvent(WEB_EVENTS.VIBE_BUILD_RECOVERY_CLICKED, { session_id: session.id, operation_id: id, action: "stop" });
    setPending(true);
    setPendingAction(action === "stop" ? "Stopping…" : "Starting your check…");
    setError("");
    try {
      await vibeFetch(`/operations/${id}/${action}`, await token(), {
        method: "POST",
        body: "{}",
      });
      if (!ownsScope(scope)) return;
      await reload();
    } catch (e) { if (!ownsScope(scope)) return;
      setError((e as Error).message);
    } finally { if (ownsScope(scope)) {
      setPending(false);
      setPendingAction(undefined);
    } }
  }
  async function upload(uploaded?: File) {
    const scope = scopeRef.current;
    if (!uploaded || busy) return;
    setPending(true);
    setImportError(undefined);
    let importSessionID = sessionID;
    try {
      const v = await ensureSession(scope);
      if (!ownsScope(scope)) return;
      importSessionID = v.id;
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
 "X-File-Name": encodeURIComponent(uploaded.name),
          },
          body: new Blob([uploaded]),
        },
      );
      if (ownsScope(scope) && accept(result)) { const imported=result.document.artifacts.at(-1); if(imported) navigation.update({artifact:imported.id,run:undefined,view:"tests"}); }
    } catch (e) { if (!ownsScope(scope)) return;
      const message = e instanceof VibeError && (!e.status || e.status >= 500)
        ? "Couldn’t import this file. Choose another file, or try importing again."
        : (e as Error).message;
      setImportError({ sessionID: importSessionID, message });
      setSettingsOpen(false);
    } finally { if (ownsScope(scope)) {
      setPending(false);
      if (file.current) file.current.value = "";
    } }
  }
  const { saveDraft,saveComplete,saveNotice,saveKept,saveArtifact,saveReturnPath,saveAdvanced,advancedReceipt,saveOpen,saveAccess,saveBaseline,saveTarget,savedCheck,workspaces,openSave,closeSave,rememberSave,save } = useVibeSaving({actorScope: authUserID || "guest",loadWorkspace: !!authUserID && (!requestedSessionID || newEvaluation),session,artifact,artifacts,content,models,workspace,savedChecks,token,authLoading,loadAttempt,setContent,setModels,setWorkspace,setError,setPending,setSavedChecks,setSelectedArtifactID,reload,writeURL:navigation.writeURL});
  const activeContext = session?.document.evaluation ? evaluationIdentity(session) : null;
  const setTrialText = (text: string) => {
    const key = trialDraftKey;
    trialEdits.current[key] = (trialEdits.current[key] || 0) + 1;
    setTrialBuffers(old => ({ ...old, [key]: text }));
  };
  // New work uses the prospective account/workspace and approved defaults,
  // independently of the historical project's funding and retired models.
  const entryAvailable = !!config?.enabled && Object.values(config.defaults).every(id => config.models.some(model => model.id === id)) && config.availability?.[authUserID ? "workspace" : "guest"]?.available === true && (!authUserID || saveAccess === "ready" && workspaces.some(item => item.id === workspace));
    return { projectReturnPath,saveDraft,saveComplete,saveNotice,saveKept,saveArtifact,saveReturnPath,saveAdvanced,advancedReceipt,archiveMode,setArchiveMode,metadata,exporter, recordedUpdate, startRecordedUpdate, closeRecordedUpdate, addRecordedEvidence, entryAvailable, authUser, authLoading, contextList, savedList, runQuoteState, runQuoteError, funding, inferenceAvailable, runInferenceAvailable, unavailableReason, messageLimit, messageError, buildQuote, buildQuoteState, refreshBuildQuote, runQuoteCurrent, isRunQuoteCurrent, accept, active, activeContext, adoptContext, applyChoice, artifact, artifacts, buildEvidence, buildJourney, buildStart, busy, canChangeDoor, changeDoor, changeModels, chooseDoor, closeSave, composerEdits, config, configError, connection, content, contexts, currentView, dirtyArtifact, discardedContextIDs, drafts, edit, error, evaluation, exportConversation, file, importError, importNotice, keptArtifact, latestArtifact, loadSavedCase, loadingSession, materialScope, materials, trialMaterials, models, navigate, navigation, newEvaluation, newTrial, openNewEvaluation, openSave, operationAction, pending, pendingAction, pendingEdit, pendingMessage, quoteError, quoteHeading, quoteMatches, quoteTrigger, reloadChoices, rememberSave, requestPreparation, requestRun, requestedRunID, retryOperation, retrySubmission, retryUnsentMessage, runPending, runQuote, save, saveAccess, saveBaseline, saveOpen, saveTarget, saved, savedCheck, savedChecks, savedDraft, savedModelNotice, savedWorkOpen, scenarioCount, select, selectedArtifactID, sendMessage, session, sessionID, sessionUnavailable, setChecksDirtyID, setConnection, setContent, setContexts, setError, setLoadAttempt, setModels, setNewEvaluation, setPending, setPendingMessage, setRunQuote, setSavedWorkOpen, setSelectedArtifactID, setSettingsOpen, setThreadID, setTrialText, setWorkspace, settingsOpen, snapshots, submission, submit, switchContext, testJourney, threadID, token, trialHistory, trialMessages, trialText, twoDoor, uncertain, upload, view, workspace, workspaces };
}
