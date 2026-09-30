"use client";


import { useVibeInputs } from "@/lib/use-vibe-inputs";
import type { InputBinding } from "@/lib/vibe-inputs";

import { downloadJSON, exportRuns } from "@/lib/vibe-export";

import { useVibeSaving } from "@/lib/use-vibe-saving";
import { useVibeDrafts } from "@/lib/use-vibe-drafts";
import { useVibeNavigation } from "@/lib/use-vibe-navigation";
import { useVibeSession, sessionAccessLost, isSessionAccessError } from "@/lib/use-vibe-session";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent } from "@/lib/vibe-build-analytics";

import type { ConversationAction } from "@/lib/vibe-conversation";


import { useSearchParams } from "next/navigation";
import { useAccessToken, useAuth } from "@workos-inc/authkit-nextjs/components";
import { useCallback, useEffect, useRef, useState } from "react";



import { workingBuildArtifact } from "@/lib/vibe-build-timeline";
import { evaluationIdentity } from "@/components/vibe/evaluation-navigation";







import { editableEvaluation, defaultModels, terminal, retryVibeOperation, VibeError, vibeFetch, type CaseResult, type BuildQuote, type RunQuote, type Models, type Operation, type Session, type VibeConfig, type SavedCheck } from "@/lib/vibe";



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
    try { return await getAccessToken(); } catch { return undefined; }
  }, [getAccessToken, authLoading, authUserID]);
  const snapshots = useVibeSession(requestedSessionID, token, authLoading, loadAttempt);
  const { session, accept, select, reload, activeSessionRef, loadingSession, connection, setConnection } = snapshots;
  const drafts = useVibeDrafts(requestedSessionID || undefined);
  const { content, setContent, trialBuffers, setTrialBuffers, composerEdits, trialEdits } = drafts;
  useEffect(() => { drafts.activate(requestedSessionID || undefined); }, [requestedSessionID, drafts.activate]);
  const initializedModels = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!session || initializedModels.current === session.id) return;
    initializedModels.current = session.id;
    const message = session.document.messages.find(m => m.preview_thread_id === navigation.thread);
    const trial = message && session.operations.find(o => o.id === message.operation_id);
    setModels(trial ? { ...session.document.models, target: trial.models.target } : session.document.models);
  }, [session, navigation.thread]);
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
  const [pending, setPending] = useState(false);
  const [pendingMessage, setPendingMessage] = useState<{
    id: string;
    content: string;
  }>();
  const [runPending, setRunPending] = useState(false);
  const requestedRunID = navigation.run;
  const setRequestedRunID = (run: string | undefined) => navigation.update({ run });
  const navigatedRunID = useRef<string | undefined>(undefined);
  const [pendingAction, setPendingAction] = useState<string>();
  const [error, setError] = useState("");
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
  const view = navigation.view;
  const selectedArtifactID = navigation.artifact;
  const setSelectedArtifactID = (artifact: string | null) => navigation.update({ artifact });
  const threadID = navigation.thread;
  const setThreadID = (thread: string) => navigation.update({ thread });
  const buildEvidence = useRef<
    { operation: string; artifact: string } | undefined
  >(undefined);
  const savedCaseReads = useRef(new Map<string, Promise<CaseResult>>());
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [checksDirtyID, setChecksDirtyID] = useState<string | null>(null);
  const [pendingEdit, setPendingEdit] = useState<{ operationID: string; artifactID: string }>();
  const [dirtyArtifactID, setDirtyArtifactID] = useState<string | null>(null);
  const [savedChecks, setSavedChecks] = useState<SavedCheck[]>([]);
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
    currentView.current = next;
    navigation.update({ view: next, artifact: agent && !(buildJourney && next === "checks") ? agent : null, thread });
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
  const attachedWorkspace = session?.workspace_id;
  useEffect(() => {
    if (attachedWorkspace) setWorkspace(attachedWorkspace);
  }, [attachedWorkspace]);
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
    select(v.id); drafts.activate(v.id); accept(v);
    navigation.writeURL(new URL(`/vibe-evals?session=${id}${workspace ? `&workspace=${workspace}` : ""}`, window.location.origin));
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
  const materialScope = view === "try" ? `trial:${artifact?.id || ""}:${threadID}` : "guide";
  const materials = useVibeInputs(sessionID || "", materialScope, token);
  const materialFingerprint = JSON.stringify(materials.bindings);
  const adoptionFingerprint = JSON.stringify(materials.adoptions);
  const buildStart = session?.document.evaluation?.door === "build" && !session.document.build && !session.document.artifacts.length;
  const continuingBuild = (session?.document.build?.phase === "clarifying" || session?.document.build?.phase === "waiting");
  const quoteMatches = !!buildQuote && buildQuote.request.content === content && JSON.stringify(buildQuote.request.inputs || []) === materialFingerprint && JSON.stringify(buildQuote.request.adopt_rules || []) === adoptionFingerprint && JSON.stringify(buildQuote.request.models) === JSON.stringify(models) && Date.parse(buildQuote.expires_at) > Date.now();
  useEffect(() => {
    if (!buildStart || !sessionID || !content.trim()) { setBuildQuote(undefined); setQuoteError(""); return; }
    let live = true;
    setQuoteError("");
    const timer = setTimeout(() => {
      void token().then(auth => vibeFetch<BuildQuote>(`/sessions/${sessionID}/build-quote`, auth, { method: "POST", body: JSON.stringify({ content, models, ...(materials.bindings.length ? { inputs: materials.bindings } : {}), ...(materials.adoptions.length ? { adopt_rules: materials.adoptions } : {}) }) }))
        .then(q => { if (live) setBuildQuote(q); }).catch(e => { if (live) setQuoteError(e.message); });
    }, 400);
    return () => { live = false; clearTimeout(timer); };
  }, [buildStart, sessionID, content, models, token, materials.bindings, materials.adoptions]);
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
    select(v.id);
    drafts.activate(v.id);
    accept(v); setModels(v.document.models);
    const selected = navigation.selectSession(v.id);
    currentView.current = selected.view;
    setError(""); setImportError(undefined); setConnection(""); setPendingMessage(undefined); setPendingEdit(undefined);
    setChecksDirtyID(null); setDirtyArtifactID(null); setBuildQuote(undefined); setRunQuote(undefined);
    buildEvidence.current = undefined;

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
    if (materials.blocked && buildStart) return;
    void submit("message", text, undefined, {
      ...context,
      ...(demoID ? { demo_id: demoID } : {}),
      ...(buildStart ? { cycle_id: buildQuote!.id } : continuingBuild ? { cycle_id: session!.document.build!.cycle_id } : {}),
      ...(testJourney || session?.document.evaluation ? {} : { quick_check: !buildEvidence.current }),
    });
  }
  async function requestPreparation(text: string, count: number, artifactID: string) {
    if (!session || busy) return;
    quoteTrigger.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const clientID = crypto.randomUUID();
    const extra = { client_id: clientID, additional_examples: count, artifact_id: artifactID };
    setPending(true); setError("");
    try {
      const quote = await vibeFetch<BuildQuote>(`/sessions/${session.id}/build-quote`, await token(), {method:"POST", body:JSON.stringify({ content:text, models, artifact_id:artifactID, additional_examples:count })});
      setRunQuote({quote: {...quote, calls: quote.max_calls}, kind:"message", content:text, extra: {...extra, cycle_id:quote.id}, sessionID:session.id, revision:session.revision});
    } catch(e) { setError((e as Error).message); } finally {setPending(false);}
  }
  async function requestRun(baseline?: Operation, evidenceID?: string, purpose?: "regrade") {
    quoteTrigger.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
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
      if ((!text.trim() && !materials.bindings.length) || materials.blocked || legacyTrial) return;
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
    const capturedAdoptions = kind === "message" && buildStart ? materials.adoptions : [];
    const capturedInputs = (kind === "playground" || kind === "message" && buildStart) ? materials.bindings : [];
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
          ...(capturedInputs.length ? { inputs: capturedInputs } : {}),
          ...(capturedAdoptions.length ? { adopt_rules: capturedAdoptions } : {}),
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
        navigation.writeURL(url);
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
      if (!accept(next)) return;
      const restored = next.document.artifacts.at(-1);
      if (action.kind === "undo" && restored) {
        setSelectedArtifactID(restored.id);
        navigate("build", restored.id);
      }
    } finally { setPending(false); }
  }
  async function reloadChoices() {
    if (session) await reload(session.id);
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
      if (!accept(v)) return false;
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
    setImportError(undefined);
    let importSessionID = sessionID;
    try {
      const v = await ensureSession();
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
          },
          body: new Blob([uploaded]),
        },
      );
      accept(result);
    } catch (e) {
      const message = e instanceof VibeError && (!e.status || e.status >= 500)
        ? "Couldn’t import this file. Choose another file, or try importing again."
        : (e as Error).message;
      setImportError({ sessionID: importSessionID, message });
      setSettingsOpen(false);
    } finally {
      setPending(false);
      if (file.current) file.current.value = "";
    }
  }
  const { saveOpen,saveAccess,saveBaseline,saveTarget,savedCheck,workspaces,openSave,closeSave,rememberSave,save } = useVibeSaving({session,artifact,artifacts,content,models,workspace,token,authLoading,loadAttempt,setContent,setModels,setWorkspace,setError,setPending,setSavedChecks,setSelectedArtifactID,reload,writeURL:navigation.writeURL});
  const activeContext = session?.document.evaluation ? evaluationIdentity(session) : null;
  const setTrialText = (text: string) => {
    const key = trialDraftKey;
    trialEdits.current[key] = (trialEdits.current[key] || 0) + 1;
    setTrialBuffers(old => ({ ...old, [key]: text }));
  };
    return { accept, active, activeContext, adoptContext, applyChoice, artifact, artifacts, buildEvidence, buildJourney, buildStart, busy, canChangeDoor, changeDoor, changeModels, chooseDoor, closeSave, composerEdits, config, configError, connection, content, contexts, currentView, dirtyArtifact, discardedContextIDs, drafts, edit, error, evaluation, exportConversation, file, importError, importNotice, keptArtifact, latestArtifact, loadSavedCase, loadingSession, materialScope, materials, models, navigate, navigation, newEvaluation, newTrial, openNewEvaluation, openSave, operationAction, pending, pendingAction, pendingEdit, pendingMessage, quoteError, quoteHeading, quoteMatches, quoteTrigger, reloadChoices, rememberSave, requestPreparation, requestRun, requestedRunID, retryOperation, retrySubmission, retryUnsentMessage, runPending, runQuote, save, saveAccess, saveBaseline, saveOpen, saveTarget, saved, savedCheck, savedChecks, savedDraft, savedModelNotice, savedWorkOpen, scenarioCount, select, selectedArtifactID, sendMessage, session, sessionID, sessionUnavailable, setChecksDirtyID, setConnection, setContent, setContexts, setError, setLoadAttempt, setModels, setNewEvaluation, setPending, setPendingMessage, setRunQuote, setSavedWorkOpen, setSelectedArtifactID, setSettingsOpen, setThreadID, setTrialText, setWorkspace, settingsOpen, snapshots, submission, submit, switchContext, testJourney, threadID, token, trialHistory, trialMessages, trialText, twoDoor, uncertain, upload, view, workspace, workspaces };
}
