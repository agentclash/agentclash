"use client";
import {loadCanonicalResult, type CanonicalResult} from "./vibe-canonical";
import type { RecordedIntakeRequest, RecordedIntakeOutcome, ResultRef, ProjectResultBinding } from "./vibe-result-evidence";


import { useVibeCollection } from "@/lib/use-vibe-collection";
import { useVibeInputs } from "@/lib/use-vibe-inputs";
import type { InputBinding } from "@/lib/vibe-inputs";


import { parseResultSelection, formatResultSelection } from "./vibe-result-selection";
import { useVibeWorkspaceAccess } from "./use-vibe-workspace-access";
import { useVibeEvidenceOwner } from "./use-vibe-evidence-owner";
import { reportBase, type SDKReport } from "./vibe-reports";
import { useVibeExport } from "./use-vibe-export";
import {pinVibeReturn} from "./vibe-keep";
import { useVibeSaving } from "@/lib/use-vibe-saving";
import { readEditorDraft, discardEditorDraft, draftMemory, readBuildDrafts, emptyDrafts, persistDrafts, editorDraftKey, type EditorDraft } from "@/lib/vibe-build-drafts";
import { useVibeEditorDraft } from "./use-vibe-editor-draft";
import { useVibeDrafts } from "@/lib/use-vibe-drafts";
import { useVibeNavigation, type VibePane } from "@/lib/use-vibe-navigation";
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



type StartConfig = VibeConfig & { first_build_ceiling_nano_usd?: number; guest_trial?: { initial_build_used: boolean; build_in_progress: boolean } };

export type StartInput = { kind: "build"; text: string } | { kind: "instructions"; text: string; model?: string } | { kind: "replies"; conversations: string; rules: string } | { kind: "pack"; file: File };

type RequestScope = { actor: string; sessionID?: string };
// The work the user started and the view they started it from. When it ends, the
// target view opens if they are still there; otherwise a link to it shows.
type Follow = { view: VibePane; target: "results" | "tests"; cycle?: string };
type PendingSubmission = {
  scope: RequestScope;
  sessionID: string;
  readonly body: string;
  composer: string | null;
  composerVersion: number;
  trialKey?: string;
  retryOperationID?: string;
  uncertain: boolean;
  startDraft?: { kind: "build" | "instructions"; value: string };
  follow?: Follow;
  // "Help me fix this": the tested version and the versions that existed before the request.
  fix?: { from?: string; known: string[] };
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
function clearStartDraft(request: PendingSubmission) {
  if (!request.startDraft) return;
  const project = `start:${request.scope.actor}:${request.startDraft.kind === "build" ? "build" : "test"}`;
  const editor = request.startDraft.kind === "build" ? "brief" : "inputs";
  const row = readEditorDraft(project, "start", editor);
  if (row?.value === request.startDraft.value) discardEditorDraft(project, "start", editor, row);
}



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
  const initializedModels = useRef<{ session: string; thread: string | null } | undefined>(undefined);
  useEffect(() => {
    if (!session) return;
    const identity = `${authUserID || "guest"}:${session.id}`;
    const previous = initializedModels.current;
    if (previous?.session === identity && previous.thread === navigation.thread) return;
    initializedModels.current = { session: identity, thread: navigation.thread };
    const message = session.document.messages.find(m => m.origin === "playground" && (m.preview_thread_id === navigation.thread || `legacy:${m.operation_id}` === navigation.thread));
    const trial = message && session.operations.find(o => o.id === message.operation_id);
    if (previous?.session !== identity) setModels(trial ? { ...session.document.models, target: trial.models.target } : session.document.models);
    else if (trial) setModels(current => ({ ...current, target: trial.models.target }));
  }, [session, navigation.thread, authUserID]);
  const discardedContextIDs = useRef(new Set<string>());
  const startDoor = !requestedSessionID && (navigation.start === "build" || navigation.start === "improve") ? navigation.start : undefined;
  const [, renderStartDrafts] = useState(0);
  useEffect(() => {
    if (!authUserID || authLoading) return;
    let migrated = false;
    for (const [door, editor] of [["build", "brief"], ["test", "inputs"]] as const) {
      const guest = `start:guest:${door}`, account = `start:${authUserID}:${door}`;
      const row = readEditorDraft(guest, "start", editor);
      if (!row || readEditorDraft(account, "start", editor)) continue;
      const all = draftMemory.get(account) || readBuildDrafts(account) || emptyDrafts();
      if (persistDrafts(account, { ...all, editors: { ...all.editors, [editorDraftKey(account, "start", editor)]: row } })) discardEditorDraft(guest, "start", editor, row);
      migrated = true;
    }
    if (migrated) renderStartDrafts(value => value + 1);
  }, [authUserID, authLoading]);
  const startBuildDraft = useVibeEditorDraft(`start:${authUserID || "guest"}:build`, "start", "brief", "", "");
  const startImproveDraft = useVibeEditorDraft(`start:${authUserID || "guest"}:test`, "start", "inputs", "", "{}");
  let startInputs: { instructions: string; model?: string; conversations: string; rules: string; mode?: "instructions" | "replies" | "pack" } = { instructions: "", conversations: "", rules: "" };
  try { startInputs = { ...startInputs, ...JSON.parse(startImproveDraft.value) }; } catch { /* Keep an unreadable draft in storage. */ }
  const setStartInputs = (patch: Partial<typeof startInputs>) => startImproveDraft.setValue(JSON.stringify({ ...startInputs, ...patch }));
  const hiddenStartDraft = useVibeEditorDraft(`start:${authUserID || "guest"}:hidden`, "start", "projects", "", "[]");
  let hiddenStarts: string[] = [];
  try { const stored: unknown = JSON.parse(hiddenStartDraft.value); if (Array.isArray(stored)) hiddenStarts = stored.filter((id): id is string => typeof id === "string"); } catch { /* Optional mirror. */ }
  for (const id of hiddenStarts) discardedContextIDs.current.add(id);
  const [startError, setStartError] = useState("");
  const starting = useRef(false);
  const previousStart = useRef(startDoor);
  useEffect(() => {
    const from = previousStart.current;
    previousStart.current = startDoor;
    if (!from || startDoor || requestedSessionID) return;
    const frame = requestAnimationFrame(() => document.getElementById(`vibe-door-${from === "build" ? "build" : "test"}`)?.focus());
    return () => cancelAnimationFrame(frame);
  }, [startDoor, requestedSessionID]);
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
  const threadID = navigation.thread || "";
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
  const currentPane = useRef(navigation.view); currentPane.current = navigation.view;
  const follow = useRef<Follow & { sessionID: string; run?: string } | undefined>(undefined);
  const fixRequest = useRef<{ sessionID: string; operation: string; from?: string; known: string[] } | undefined>(undefined);
  const [arrival, setArrival] = useState<{ sessionID: string; target: Follow["target"]; run?: string }>();
  // A8: an admitted run is in the URL before the session snapshot contains it.
  const admittedRuns = useRef(new Set<string>());
  const startFocus = useRef<string | undefined>(undefined);
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
  const requestedResult = parseResultSelection(requestedRunID);
  const evidenceAccess = useVibeWorkspaceAccess(workspaceActor, session?.workspace_id, token, authLoading, loadAttempt);
  const [resultAttempt, setResultAttempt] = useState(0);
  const resultLife = useVibeEvidenceOwner(workspaceActor, requestedSessionID || undefined, session?.workspace_id, requestedRunID, requestedResult?.kind === "run" || requestedResult?.kind === "eval_session" ? navigation.artifact || undefined : undefined, String(resultAttempt), evidenceAccess.access.canRead ? "read" : evidenceAccess.access.status);
  const [sourceSelection, setSourceSelection] = useState<{owner:object;value?:CanonicalResult | SDKReport;error?:string;invalid?:boolean}>();
  const externalKind = requestedResult && requestedResult.kind !== "vibe_operation" ? requestedResult.kind : undefined;
  const externalID = externalKind ? requestedResult?.id : undefined;
  useEffect(() => {
    if (!externalID || !session || session.id !== requestedSessionID || authLoading || !evidenceAccess.access.canRead) return;
    const captured = resultLife.capture();
    let live = true;
    const current = () => live && captured();
    let timer: ReturnType<typeof setTimeout>;
    async function load() {
      try {
        const auth = await token();
        if (!current()) return;
        const value = externalKind === "run" || externalKind === "eval_session"
          ? await loadCanonicalResult(session!.id, { kind: externalKind, id: externalID! }, auth, navigation.artifact || undefined)
          : await vibeFetch<SDKReport>(`${reportBase(session!.id)}/${externalID}`, auth);
        if (!current()) return;
        if (value.binding.session_id !== session!.id || value.binding.result_ref.kind !== externalKind || value.binding.result_ref.id !== externalID ||
            (externalKind === "sdk_report" && (value as SDKReport).id !== externalID))
          throw new VibeError("not_found", "The selected result is unavailable in this project.", 404);
        setSourceSelection({ owner: resultLife.owner, value });
        if (externalKind !== "sdk_report" && ["queued", "running", "provisioning", "scoring", "aggregating", "pending"].includes((value as CanonicalResult).status))
          timer = setTimeout(load, 3000);
      } catch (e) {
        if (!current()) return;
        const invalid = e instanceof VibeError && (e.status === 404 || e.code === "invalid_request");
        const denied = invalid || e instanceof VibeError && (e.status === 401 || e.status === 403);
        setSourceSelection(old => ({ owner: resultLife.owner, value: !denied && old?.owner === resultLife.owner ? old.value : undefined,
          error: invalid ? "The selected result is unavailable in this project." : (e as Error).message, invalid }));
      }
    }
    void load();
    return () => { live = false; clearTimeout(timer); };
  }, [externalID, externalKind, session?.id, requestedSessionID, authLoading, token, resultLife, evidenceAccess.access.canRead]);
  const sourceState = sourceSelection?.owner === resultLife.owner ? sourceSelection : undefined;
  const selectedResult = sourceState?.value?.binding;
  const selectedCanonical = selectedResult?.result_ref.kind === "run" || selectedResult?.result_ref.kind === "eval_session" ? sourceState?.value as CanonicalResult : undefined;
  const selectedSDKReport = selectedResult?.result_ref.kind === "sdk_report" ? sourceState?.value as SDKReport : undefined;
  const selectedResultLoading = !!externalKind && !sourceState && (evidenceAccess.access.status === "loading" || evidenceAccess.access.canRead);
  const selectedResultError = sourceState?.error || (externalKind && evidenceAccess.access.status !== "loading" && !evidenceAccess.access.canRead
    ? evidenceAccess.access.status === "signed_out" ? "Sign in to read this result." : "Workspace access could not be confirmed." : undefined);
  const retrySelectedResult = () => evidenceAccess.access.canRead ? setResultAttempt(n => n + 1) : evidenceAccess.retry();
  const pinnedRun = session?.operations.find(item => item.id === (requestedResult?.kind === "vibe_operation" ? requestedResult.id : requestedRunID));
  const pinnedArtifactID = selectedResult?.artifact_id || pinnedRun?.result_scope?.artifact_id || pinnedRun?.source?.artifact_id || pinnedRun?.results[0]?.version;
  const artifact = pinnedArtifactID ? artifacts.find(item => item.id === pinnedArtifactID) : selectedArtifactID ? artifacts.find((a) => a.id === selectedArtifactID) : session?.document.evaluation?.door === "build" ? workingArtifact : latestArtifact;
  const selectionLife = useVibeEvidenceOwner(workspaceActor, requestedSessionID || undefined, session?.workspace_id, artifact?.id, requestedRunID);
  function onSelectResult(binding: ProjectResultBinding, caseKey?: string) {
    if (!selectionLife.current() || !session || session.id !== requestedSessionID || binding.session_id !== session.id || !session.workspace_id || !artifacts.some(a => a.id === binding.artifact_id)) return;
    const ref = parseResultSelection(formatResultSelection(binding.result_ref));
    if (!ref) return;
    navigation.update({view:"results",artifact:binding.artifact_id,run:formatResultSelection(ref),case:caseKey});
  }
  async function onSDKPromoted(promoted: import("./vibe").Artifact) {
    const current = selectionLife.capture();
    try {
      const fresh = await reload();
      if (current() && fresh?.document.artifacts.some(a => a.id === promoted.id)) navigation.update({view:"tests",artifact:promoted.id,run:undefined,case:undefined});
    } catch(e) { if (current()) setError((e as Error).message); }
  }
  async function onSDKRegraded(operation: Operation) {
    const current = selectionLife.capture();
    try {
      const fresh = await reload();
      if (current() && fresh?.operations.some(o => o.id === operation.id)) navigation.update({view:"results",artifact:artifact?.id || null,run:operation.id,case:undefined});
    } catch(e) { if (current()) setError((e as Error).message); }
  }
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
    const run = session.operations.find(item => item.id === (requestedResult?.kind === "vibe_operation" ? requestedResult.id : navigation.run) && (item.kind === "check" || item.kind === "retest"));
    if (run) admittedRuns.current.delete(run.id);
    // A8: wait for a run this page admitted; only another missing run is unavailable.
    if (navigation.run && !run && !externalKind && !admittedRuns.current.has(requestedResult?.id || navigation.run)) { patch.run = undefined; patch.case = undefined; setError("The selected result is unavailable in this project."); }
    if (run && requestedResult?.kind === "vibe_operation" && navigation.run !== run.id) patch.run = formatResultSelection(requestedResult);
    if (sourceState?.invalid || selectedResult && !artifacts.some(a => a.id === selectedResult.artifact_id)) {
      patch.run = undefined; patch.case = undefined; setError("The selected result is unavailable in this project.");
    }
    if (selectedResult && artifacts.some(a => a.id === selectedResult.artifact_id) && navigation.artifact !== selectedResult.artifact_id) patch.artifact = selectedResult.artifact_id;
    const tested = run?.source?.artifact_id || run?.results[0]?.version;
    if (tested && session.document.artifacts.some(item => item.id === tested) && navigation.artifact !== tested) patch.artifact = tested;
    const cases = (artifact?.blueprint as { cases?: { key?: string }[] } | undefined)?.cases || [];
    if (!externalKind && navigation.case && !cases.some(item => item.key === navigation.case) && !run?.results.some(item => item.case_key === navigation.case)) patch.case = undefined;
    if (navigation.thread === null && !sending.current && trialHistory.length) patch.thread = trialHistory.at(-1)!.id;
    if (threadID && !sending.current && !trialHistory.some(item => item.id === threadID) && !Object.hasOwn(trialBuffers, trialDraftKey) && !submission.current?.trialKey?.endsWith(`:${threadID}`)) patch.thread = null;
    if (Object.keys(patch).length) navigation.update(patch, "replace");
  }, [session, requestedSessionID, navigation.artifact, navigation.run, navigation.case, navigation.thread, threadID, artifact, sourceState, selectedResult, externalKind]);
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
    thread = navigation.thread,
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
    if (authLoading) return;
    let alive = true;
    setConfigError(false);
    token().then(auth => vibeFetch<VibeConfig>("/config", auth))
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
  }, [requestedSessionID, loadAttempt, token, authLoading]);
  async function ensureSession(scope = scopeRef.current, ownsAttempt = () => true) {
    if (requestedSessionID && session?.id !== requestedSessionID)
      throw new Error(
        "Load this project before sending. No new project was created.",
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
  const projectReturnPath=session&&artifact?pinVibeReturn(window.location.pathname+window.location.search,session.id,artifact.id,selectedResult ? formatResultSelection(selectedResult.result_ref) : selectedExportRun?.id):undefined;
  function exportConversation(){if(!artifact)return;setSettingsOpen(false);void exporter.startResult(artifact,selectedResult?.result_ref || (selectedExportRun ? {kind:"vibe_operation",id:selectedExportRun.id} : undefined),"selected",selectedExportRun)}
  function loadSavedCase(id: string, key: string): Promise<CaseResult> {
    const owner = session?.id;
    if (!owner || activeSessionRef.current !== owner)
      return Promise.reject(new Error("This project changed. Reopen its saved results."));
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
    if (!config || configError || !inferenceAvailable || !buildStart || !sessionID || !content.trim() || new TextEncoder().encode(content).length > messageLimit) { setBuildQuote(undefined); setQuoteError(""); return; }
    let live = true;
    setQuoteError(""); setQuotePreparing(true);
    const timer = setTimeout(() => {
      captureVibeAttempt("build", "estimate", sessionID);
      void token().then(auth => vibeFetch<BuildQuote>(`/sessions/${sessionID}/build-quote`, auth, { method: "POST", body: JSON.stringify({ content, models, ...(materials.bindings.length ? { inputs: materials.bindings } : {}), ...(materials.adoptions.length ? { adopt_rules: materials.adoptions } : {}) }) }))
        .then(q => { if (live) { syncClock(q.server_time); setClock(Date.now()); setBuildQuote(q); } }).catch(e => { captureVibeAttempt("build", "estimate", sessionID, e instanceof VibeError ? e.code : "transport_failed"); if (live) setQuoteError(e.message); }).finally(() => { if (live) setQuotePreparing(false); });
    }, 400);
    return () => { live = false; clearTimeout(timer); };
  }, [config, configError, buildStart, sessionID, content, models, token, materialFingerprint, adoptionFingerprint, quoteAttempt, inferenceAvailable]);
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
  function chooseDoor(door: "build" | "test") {
    if (!config || pending || uncertain) return;
    setStartError(""); setNewEvaluation(false);
    select();
    navigation.writeURL(new URL(`/vibe-evals?start=${door === "build" ? "build" : "improve"}${workspace ? `&workspace=${encodeURIComponent(workspace)}` : ""}`, window.location.origin), "push");
  }
  function backFromStart() {
    setStartError("");
    navigation.writeURL(new URL(`/vibe-evals${workspace ? `?workspace=${encodeURIComponent(workspace)}` : ""}`, window.location.origin), "push");
  }
  async function startProject(door: "build" | "test", first: StartInput): Promise<void> {
    if (starting.current || pending || !config || requestedSessionID) return;
    if (first.kind !== "pack" && !(first.kind === "replies" ? first.conversations.trim() && first.rules.trim() : first.text.trim())) return;
    if (first.kind !== "pack" && first.kind !== "replies" && new TextEncoder().encode(first.text).length > messageLimit) { setStartError(`Message exceeds the ${messageLimit.toLocaleString()} byte UTF-8 limit.`); return; }
    if ((first.kind === "build" || first.kind === "instructions") && !entryAvailable) { setStartError(unavailableReason); return; }
    if (first.kind === "build" && !authUserID && (config as StartConfig).guest_trial?.initial_build_used) { setStartError("Your free build is used. Sign in to continue."); return; }
    if (first.kind === "build" && !authUserID && (config as StartConfig).guest_trial?.build_in_progress) { setStartError("Another build is using your free trial. Try again when it finishes."); return; }
    if (first.kind === "pack" && first.file.size > (authUserID ? 1024 * 1024 : 256 * 1024)) { setStartError("This file exceeds the import size limit."); return; }
    const scope = scopeRef.current;
    const id = crypto.randomUUID();
    const clientID = crypto.randomUUID();
    const rawTitle = first.kind === "pack" ? first.file.name.replace(/\.[^.]+$/, "") : first.kind === "replies" ? "Recorded replies" : first.text.trim().split(/(?<=[.!?])\s|\n/)[0];
    const title = Array.from(rawTitle.replace(/[\u0000-\u001f\u007f-\u009f]/g, " ").trim()).slice(0, 60).join("");
    const requestModels = { ...models, ...(first.kind === "instructions" && first.model ? { target: first.model } : {}) };
    starting.current = true; setPending(true); setStartError("");
    let auth: string | undefined;
    let admitted = false;
    let firstRequest: PendingSubmission | undefined;
    let createdRevision: number | undefined;
    try {
      auth = await token();
      if (!ownsScope(scope)) return;
      await vibeFetch<Session>("/sessions", auth, { method: "POST", body: JSON.stringify({ id, door, title, ...(workspace ? { workspace_id: workspace } : {}) }) });
      let next = await vibeFetch<Session>(`/sessions/${id}`, auth);
      createdRevision = next.revision;
      if (!ownsScope(scope)) throw Error("The active account or project changed.");
      if (first.kind === "pack") {
        next = await vibeFetch<Session>(`/sessions/${id}/import`, auth, { method: "POST", headers: { "Content-Type": first.file.name.endsWith(".json") ? "application/json" : "application/yaml", "If-Match": String(next.revision), "X-File-Name": encodeURIComponent(first.file.name) }, body: new Blob([first.file]) });
      } else if (first.kind === "replies") {
        next = await vibeFetch<Session>(`/sessions/${id}/evidence`, auth, { method: "POST", body: JSON.stringify({ client_id: clientID, revision: next.revision, label: "Recorded replies", content: first.conversations, expectations: first.rules.split("\n").map(line => line.trim()).filter(Boolean).map((statement, index) => ({ id: `rule-${index + 1}`, statement })) }) });
        if (next.evidence_receipt?.client_id !== clientID) throw Error("The saved reply receipt is unavailable. Try again.");
      } else {
        const quote = first.kind === "build" ? await vibeFetch<BuildQuote>(`/sessions/${id}/build-quote`, auth, { method: "POST", body: JSON.stringify({ content: first.text, models: requestModels }) }) : undefined;
        if (!ownsScope(scope)) throw Error("The active account or project changed.");
        const content = first.kind === "build" ? first.text : `Write three tests for these instructions.\n\nAgent instructions to test:\n${first.text}`;
        const following: Follow = { view: "agent", target: door === "build" ? "results" : "tests", ...(quote ? { cycle: quote.id } : {}) };
        firstRequest = {
          scope: { actor: scope.actor, sessionID: id }, sessionID: id,
          body: JSON.stringify({ client_id: clientID, revision: next.revision, kind: "message", evaluation_first: true, test_journey: true, content, models: requestModels, ...(first.kind === "instructions" ? { instructions: first.text } : {}), ...(quote ? { cycle_id: quote.id } : {}) }),
          composer: null, composerVersion: 0, uncertain: false, follow: following,
          startDraft: { kind: first.kind, value: first.kind === "build" ? first.text : startImproveDraft.value },
        };
        try {
          const operation = await vibeFetch<Operation>(`/sessions/${id}/messages`, auth, { method: "POST", body: firstRequest.body });
          admitted = true;
          next = { ...next, operations: [...next.operations.filter(item => item.id !== operation.id), operation], document: { ...next.document, messages: [...next.document.messages, { id: clientID, role: "user", content, operation_id: operation.id }] } };
        } catch (e) {
          if (recoverRequest(firstRequest, e) === "rejected") throw e;
          // A missing response is not a rejection. Read the receipt before offering an exact replay.
          admitted = true;
          try { next = await vibeFetch<Session>(`/sessions/${id}`, auth); } catch { /* Keep the project visible while its receipt is unavailable. */ }
          const message = next.document.messages.find(item => item.id === clientID && item.role === "user");
          if (message && next.operations.some(item => item.id === message.operation_id)) firstRequest.uncertain = false;
          else recoveries.current.set(recoveryKey(firstRequest.scope), { submission: { current: firstRequest }, sending: { current: false }, retry: { current: undefined }, message: { id: clientID, content }, error: (e as Error).message });
        }
        follow.current = { sessionID: id, ...following };
      }
      admitted = true;
      if (!ownsScope(scope)) return;
      const url = new URL(`/vibe-evals?session=${id}${workspace ? `&workspace=${encodeURIComponent(workspace)}` : ""}`, window.location.origin);
      if (first.kind === "pack" || first.kind === "replies") { url.searchParams.set("view", "tests"); const artifact = next.document.artifacts.at(-1); if (artifact) url.searchParams.set("agent", artifact.id); }
      startFocus.current = id;
      navigation.writeURL(url, "push");
      adoptContext(next);
      // adoptContext restores remembered views; first intake always lands on its tests.
      if (first.kind === "pack" || first.kind === "replies") navigation.update({ view: "tests", artifact: next.document.artifacts.at(-1)?.id || null }, "replace");
      setContexts(old => [next, ...old.filter(row => row.id !== id)]);
      if (!firstRequest?.uncertain) {
        if (first.kind === "build") startBuildDraft.discard(first.text);
        if (first.kind === "instructions" || first.kind === "replies") startImproveDraft.discard(startImproveDraft.value);
      }
    } catch (e) {
      if (!admitted) {
        discardedContextIDs.current.add(id);
        hiddenStartDraft.setValue(JSON.stringify([...new Set([...hiddenStarts, id])]));
        try {
          const revision = createdRevision ?? (await vibeFetch<Session>(`/sessions/${id}`, auth)).revision;
          await vibeFetch(`/sessions/${id}?revision=${revision}`, auth, { method: "DELETE" });
        } catch { /* Failed starts remain hidden if cleanup is unavailable. */ }
      }
      if (ownsScope(scope)) setStartError((e as Error).message);
    } finally { starting.current = false; if (ownsScope(scope)) setPending(false); }
  }
  const takeStartFocus = (id: string) => { if (startFocus.current !== id) return false; startFocus.current = undefined; return true; };
  async function switchContext(id: string) {
    const scope = scopeRef.current;
    if (id === sessionID || pending || uncertain) return;
    setPending(true);
    try { const next = await vibeFetch<Session>(`/sessions/${id}`, await token()); if (ownsScope(scope)) adoptContext(next); }
    catch (e) { if (!ownsScope(scope)) return; setError((e as Error).message); }
    finally { if (ownsScope(scope)) { setPending(false); } }
  }
  function openNewEvaluation() {
    if (startDoor) backFromStart();
    setImportError(undefined);
    setNewEvaluation(true);
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
    } catch(e) { if (!ownsScope(scope)) return; setRunQuoteError((e as Error).message); captureVibeAttempt(session.document.evaluation?.door || "test", "estimate", session.id, e instanceof VibeError ? e.code : "transport_failed"); } finally { if (ownsScope(scope)) {setPending(false); setRunQuotePreparing(false);} }
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
    } catch (e) { if (!ownsScope(scope)) return; setRunQuoteError((e as Error).message); captureVibeAttempt(session.document.evaluation?.door || "test", "estimate", session.id, e instanceof VibeError ? e.code : "transport_failed"); }
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
    // In a two-door project, follow a run, the first Build run or the first test preparation from the view it starts on.
    const startedFrom = currentPane.current;
    const followed: Follow | undefined = !session?.document.evaluation ? undefined
      : kind === "check" || kind === "retest" ? { view: startedFrom, target: "results" }
      : kind !== "message" ? undefined
      : buildJourney && extra.cycle_id && (buildStart || continuingBuild) && !session.document.artifacts.length ? { view: startedFrom, target: "results", cycle: extra.cycle_id }
      : session.document.evaluation.door === "test" && !session.document.artifacts.some(a => a.kind === "test_suite") ? { view: startedFrom, target: "tests" } : undefined;
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
        follow: followed,
        fix: kind === "message" && extra.purpose === "suggest_change" ? { from: extra.artifact_id, known: v.document.artifacts.map(a => a.id) } : undefined,
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
      clearStartDraft(request);
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
      const run = requestKind === "check" || requestKind === "retest";
      if (request.follow) { follow.current = { ...request.follow, sessionID: request.sessionID, ...(run ? { run: admitted.id } : {}) }; setArrival(undefined); }
      if (request.fix) fixRequest.current = { ...request.fix, sessionID: request.sessionID, operation: admitted.id };
      if (run) {
        admittedRuns.current.add(admitted.id);
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


  useEffect(() => {
    const request = submission.current;
    if (!request?.uncertain || !request.startDraft || !visibleRecovery() || session?.id !== request.sessionID) return;
    const clientID = JSON.parse(request.body).client_id;
    const message = session.document.messages.find(item => item.id === clientID && item.role === "user");
    if (!message || !session.operations.some(item => item.id === message.operation_id)) return;
    submission.current = null; clearStartDraft(request); setPendingMessage(undefined); setUncertain(false); setError("");
  }, [session, uncertain, submission, visibleRecovery, setPendingMessage, setError]);

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

  // Opens a view and moves focus to its heading, unless the user is typing.
  function showView(target: Follow["target"], run?: string, mode: "push" | "replace" = "push") {
    navigation.update({ view: target, ...(run ? { run } : {}) }, mode);
    setArrival(undefined);
    requestAnimationFrame(() => {
      if (document.activeElement?.closest('input, textarea, select, [contenteditable="true"]')) return;
      document.querySelector<HTMLElement>(target === "results" ? '[aria-label="Run results"] h2' : '[aria-label="Your tests"] h2')?.focus();
    });
  }
  // The user is typing in the open view: an unsaved edit, or focus in a list, an editable region or a
  // text field with text in the main view. An empty field does not count: after "Start run" the cost
  // dialog returns focus to the empty trial composer. Decide this before the view changes: after it, focus is gone.
  function typingInView() {
    if (dirtyArtifact) return true;
    const field = document.activeElement?.closest('textarea, select, [contenteditable=""], [contenteditable="true"], input:not([type="button"]):not([type="submit"]):not([type="reset"]):not([type="checkbox"]):not([type="radio"]):not([type="file"]):not([type="range"]):not([type="color"]):not([type="image"])');
    if (!field?.closest(".vibe-project-main")) return false;
    return field instanceof HTMLTextAreaElement || field instanceof HTMLInputElement ? !!field.value : true;
  }
  // The result comes to the user: a finished run, first Build run or first test preparation
  // opens its view if the user is still where it started and is not typing there. Otherwise a link to it shows.
  useEffect(() => {
    const started = follow.current;
    if (!started || !session) return;
    if (session.id !== started.sessionID) { follow.current = undefined; return; }
    const build = started.cycle && session.document.build?.cycle_id === started.cycle ? session.document.build : undefined;
    if (build?.phase === "error") { follow.current = undefined; return; }
    const done = started.run ? session.operations.some(o => o.id === started.run && terminal(o.state))
      : started.cycle ? build?.phase === "results" : session.document.artifacts.some(a => a.kind === "test_suite");
    if (!done) return;
    follow.current = undefined;
    if (navigation.view === started.target) return;
    if (navigation.view === started.view && !document.querySelector('[role="dialog"]') && !typingInView()) showView(started.target, started.run, "replace");
    else setArrival({ sessionID: session.id, target: started.target, run: started.run });
    // Only a new snapshot can finish the followed work.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session]);
  useEffect(() => {
    if (arrival && (navigation.view === arrival.target || session?.id !== arrival.sessionID)) setArrival(undefined);
  }, [arrival, navigation.view, session?.id]);
  // After "Help me fix this", select Guide's new version on the Agent view, where the fix shows.
  useEffect(() => {
    const fix = fixRequest.current;
    if (!fix || !session) return;
    if (session.id !== fix.sessionID) { fixRequest.current = undefined; return; }
    const operations = session.operations.filter(o => o.id === fix.operation || o.retry_of_operation_id === fix.operation);
    const proposed = operations.map(o => o.completion_receipt?.artifact_id).find(Boolean) ||
      session.document.messages.find(m => m.role === "assistant" && m.artifact_id && operations.some(o => o.id === m.operation_id))?.artifact_id ||
      session.document.artifacts.findLast(a => !fix.known.includes(a.id) && !!fix.from && a.parent_id === fix.from)?.id;
    if (!proposed || !session.document.artifacts.some(a => a.id === proposed)) {
      if (operations.some(o => o.state === "COMPLETED")) fixRequest.current = undefined;
      return;
    }
    fixRequest.current = undefined;
    if (navigation.view !== "agent" || dirtyArtifact || artifact?.id === proposed) return;
    navigation.update({ view: "agent", artifact: proposed, run: undefined, case: undefined, thread: "" }, "replace");
    // Only a new snapshot can deliver the proposed version.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session]);

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
    const submittedIntake = readEditorDraft(id, "intake", "recorded-request");
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
      // Clear only this acknowledged request before URL subscribers render its saved version.
      if (submittedIntake?.value === JSON.stringify(request)) discardEditorDraft(id, "intake", "recorded-request", submittedIntake);
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
    setPendingAction(action === "stop" ? "Stopping…" : "Starting your run…");
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
  const { saveDraft,saveComplete,saveNotice,saveKept,saveArtifact,saveReturnPath,saveAdvanced,advancedReceipt,saveOpen,saveAccess,saveBaseline,saveTarget,savedCheck,workspaces,openSave,closeSave,rememberSave,save } = useVibeSaving({actorScope: authUserID || "guest",selectedResult,account:evidenceAccess,session,artifact,artifacts,content,models,workspace,savedChecks,token,setContent,setModels,setWorkspace,setError,setPending,setSavedChecks,setSelectedArtifactID,reload,writeURL:navigation.writeURL});
  useEffect(() => {
    if (requestedSessionID || authLoading || saveAccess !== "ready") return;
    const restored = params.get("workspace");
    const next = workspaces.find(item => item.id === restored)?.id || (workspaces.length === 1 ? workspaces[0].id : undefined);
    if (next && workspace !== next && (!workspace || workspaces.length === 1)) setWorkspace(next);
  }, [requestedSessionID, authLoading, saveAccess, params, workspaces, workspace, setWorkspace]);
  const activeContext = session?.document.evaluation ? evaluationIdentity(session) : null;
  const setTrialText = (text: string) => {
    const key = trialDraftKey;
    trialEdits.current[key] = (trialEdits.current[key] || 0) + 1;
    setTrialBuffers(old => ({ ...old, [key]: text }));
  };
  // New work uses the prospective account/workspace and approved defaults,
  // independently of the historical project's funding and retired models.
  const entryAvailable = !authLoading && !configError && !!config?.enabled && Object.values(config.defaults).every(id => config.models.some(model => model.id === id)) && config.availability?.[authUserID ? "workspace" : "guest"]?.available === true && (!authUserID || saveAccess === "ready" && workspaces.some(item => item.id === workspace));
  const visibleArrival = arrival && session?.id === arrival.sessionID && navigation.view !== arrival.target ? { target: arrival.target, open: () => showView(arrival.target, arrival.run) } : undefined;
    return { startDoor, startProject, backFromStart, startBuildDraft, startInputs, setStartInputs, startError, arrival: visibleArrival, takeStartFocus, evidenceResult:selectedResult || (pinnedRun?.result_scope&&session?{session_id:session.id,artifact_id:pinnedRun.result_scope.artifact_id,result_ref:{kind:"vibe_operation" as const,id:pinnedRun.id}}:undefined), selectedCanonical,selectedSDKReport,selectedResult,selectedResultLoading,selectedResultError,retrySelectedResult,onSelectResult,onSDKPromoted,onSDKRegraded,evidenceAccess, sdkInferenceAvailable:inferenceReady && !busy && !!config?.models.some(m=>m.id===models.evaluator), projectReturnPath,saveDraft,saveComplete,saveNotice,saveKept,saveArtifact,saveReturnPath,saveAdvanced,advancedReceipt,archiveMode,setArchiveMode,metadata,exporter, recordedUpdate, startRecordedUpdate, closeRecordedUpdate, addRecordedEvidence, entryAvailable, authUser, authLoading, contextList, savedList, runQuoteState, runQuoteError, funding, inferenceAvailable, runInferenceAvailable, unavailableReason, messageLimit, messageError, buildQuote, buildQuoteState, refreshBuildQuote, runQuoteCurrent, isRunQuoteCurrent, accept, active, activeContext, adoptContext, applyChoice, artifact, artifacts, buildEvidence, buildJourney, buildStart, busy, changeModels, chooseDoor, closeSave, composerEdits, config, configError, connection, content, contexts, currentView, dirtyArtifact, discardedContextIDs, drafts, edit, error, evaluation, exportConversation, file, importError, importNotice, keptArtifact, latestArtifact, loadSavedCase, loadingSession, materialScope, materials, trialMaterials, models, navigate, navigation, newEvaluation, newTrial, openNewEvaluation, openSave, operationAction, pending, pendingAction, pendingEdit, pendingMessage, quoteError, quoteHeading, quoteMatches, quoteTrigger, reloadChoices, rememberSave, requestPreparation, requestRun, requestedRunID, retryOperation, retrySubmission, retryUnsentMessage, runPending, runQuote, save, saveAccess, saveBaseline, saveOpen, saveTarget, saved, savedCheck, savedChecks, savedDraft, savedModelNotice, savedWorkOpen, scenarioCount, select, selectedArtifactID, sendMessage, session, sessionID, sessionUnavailable, setChecksDirtyID, setConnection, setContent, setContexts, setError, setLoadAttempt, setModels, setNewEvaluation, setPending, setPendingMessage, setRunQuote, setSavedWorkOpen, setSelectedArtifactID, setSettingsOpen, setThreadID, setTrialText, setWorkspace, settingsOpen, snapshots, submission, submit, switchContext, testJourney, threadID, token, trialHistory, trialMessages, trialText, twoDoor, uncertain, upload, view, workspace, workspaces };
}
