"use client";

import type { ConversationAction } from "@/lib/vibe-conversation";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { useCallback, useEffect, useRef, useState } from "react";
import { EvaluationWorkspace } from "@/components/vibe/evaluation-workspace";
import { pendingQuickCheck, quickCheckClientID } from "@/lib/vibe-quick-check";
import { VibeButton } from "@/components/vibe/vibe-button";
import { sendOnEnter } from "@/components/vibe/composer-keyboard";
import { Requirements } from "@/components/vibe/requirements";
import { CreditsDialog } from "@/components/vibe/credits-dialog";
import { ArrowUp, Paperclip } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { ArtifactPanel, ModelSelect } from "@/components/vibe/artifact-panel";
import { AgentReply } from "@/components/vibe/safe-markdown";
import { createApiClient } from "@/lib/api/client";
import type { UserMeResponse } from "@/lib/api/types";
import {
  editableEvaluation,
  exportAgent,
  defaultModels,
  terminal,
  retryVibeOperation,
  VibeError,
  vibeFetch,
  watchVibe,
  type CaseResult,
  type Models,
  type Operation,
  type Session,
  type VibeConfig,
  type SavedCheck,
} from "@/lib/vibe";

// POST /messages faults from api/vibe.go, Service.Prepare and Store.Submit:
// these exact code/status pairs reject before admission or roll back its transaction.
// Unknown responses (including generic 503s) cannot prove non-admission.
const submissionRejections: Partial<Record<number, readonly string[]>> = {
  400: [
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
  const [session, setSession] = useState<Session | null>(null);
  const [config, setConfig] = useState<VibeConfig | null>(null);
  const [configError, setConfigError] = useState(false);
  const [models, setModels] = useState<Models>(defaultModels);
  const [content, setContent] = useState("");
  const [pending, setPending] = useState(false);
  const [pendingMessage, setPendingMessage] = useState<{
    id: string;
    content: string;
  }>();
  const [autoCheckID, setAutoCheckID] = useState<string>();
  const [runPending, setRunPending] = useState(false);
  const [requestedRunID, setRequestedRunID] = useState<string>();
  const navigatedRunID = useRef<string | undefined>(undefined);
  const [pendingAction, setPendingAction] = useState<string>();
  const [blockedAutoCheckID, setBlockedAutoCheckID] = useState<string>();
  const quickCheckAttempts = useRef(new Set<string>());
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
  const restoredThread = useRef(params.get("thread"));
  const [trialBuffers, setTrialBuffers] = useState<Record<string, string>>({});
  const trialEdits = useRef<Record<string, number>>({});
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [instructionsOpen, setInstructionsOpen] = useState(false);
  const [checksDirtyID, setChecksDirtyID] = useState<string | null>(null);
  const [pendingEdit, setPendingEdit] = useState<{ operationID: string; artifactID: string }>();
  const [dirtyArtifactID, setDirtyArtifactID] = useState<string | null>(null);
  const [saveOpen, setSaveOpen] = useState(false);
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
  const testJourney =
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
  const artifact =
    artifacts.find((a) => a.id === selectedArtifactID) || latestArtifact;
  const dirtyArtifact =
    !!artifact &&
    (dirtyArtifactID === artifact.id || checksDirtyID === artifact.id);
  const quickCheck = pendingQuickCheck(session);
  const quickChecking =
    runPending ||
    !!autoCheckID ||
    !!(quickCheck && quickCheck.id !== blockedAutoCheckID && !dirtyArtifact);
  const evaluation = artifact ? editableEvaluation(artifact.blueprint) : null;
  const scenarioCount =
    artifact?.test_plan?.scenarios.length ||
    evaluation?.scenarios?.length ||
    evaluation?.examples.length;
  const emptyTrialKey = `new:${artifact?.id || ""}:${models.target}`;
  const trialDraftKey = threadID || emptyTrialKey;
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
    if (agent) setSelectedArtifactID(agent);
    const url = new URL(window.location.href);
    url.searchParams.set("view", next);
    if (agent) url.searchParams.set("agent", agent);
    else url.searchParams.delete("agent");
    if (thread) url.searchParams.set("thread", thread);
    else url.searchParams.delete("thread");
    window.history.replaceState(null, "", url.pathname + url.search);
  }
  function newTrial() {
    setThreadID("");
    setTrialBuffers((old) => ({ ...old, [emptyTrialKey]: "" }));
    navigate("try", artifact?.id, "");
  }
  function changeModels(next: Models) {
    if (next.target !== models.target) {
      setThreadID("");
      navigate(view, artifact?.id, "");
    }
    setModels(next);
  }
  const savedModels = session?.saved_models;
  // Canonical identity survives unknown legacy receipts or changed selections;
  // only the immutable model receipt can confirm that these choices were saved.
  const savedDraft =
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
  const token = useCallback(async () => {
    try {
      return await getAccessToken();
    } catch {
      return undefined;
    }
  }, [getAccessToken]);
  useEffect(() => {
    let current = true;
    void token()
      .then(async (auth) => {
        if (!auth) return;
        const items = await vibeFetch<SavedCheck[]>(
          `/saved-checks${workspace ? `?workspace=${workspace}` : ""}`,
          auth,
        );
        if (current) setSavedChecks(items);
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
    if (!id) return;
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
          setSession(v);
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
  }, [requestedSessionID, token, loadAttempt]);
  useEffect(() => {
    if (!sessionID) return;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    const connect = async () => {
      try {
        await watchVibe(sessionID, await token(), controller.signal, (v) => {
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
  }, [sessionID, token, loadAttempt]);
  const reload = async (id = sessionID) => {
    if (!id) return;
    const v = await vibeFetch<Session>(`/sessions/${id}`, await token());
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
    setSession(v);
    window.history.replaceState(
      null,
      "",
      `/vibe-evals?session=${id}${workspace ? `&workspace=${workspace}` : ""}`,
    );
    return v;
  }
  function exportConversation() {
    if (!session) return;
    const url = URL.createObjectURL(
      new Blob(
        [
          JSON.stringify(
            {
              format: "agentclash-vibe-conversation-v1",
              document: session.document,
              capabilities: config?.capabilities || [],
            },
            null,
            2,
          ),
        ],
        { type: "application/json" },
      ),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = "agentclash-vibe-conversation.json";
    a.click();
    URL.revokeObjectURL(url);
  }
  async function submit(
    kind = "message",
    text = content,
    baseline?: Operation,
    extra: {
      client_id?: string;
      quick_check?: boolean;
      instructions?: string;
      purpose?: "suggest_change";
      viewed_run_id?: string;
      viewed_case_key?: string;
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
      trialKey = previewThread;
      if (!threadID) {
        trialEdits.current[previewThread] =
          trialEdits.current[emptyTrialKey] || 0;
        setTrialBuffers((old) => ({
          ...old,
          [previewThread!]: text,
          [emptyTrialKey]: "",
        }));
        setThreadID(previewThread);
        navigate("try", artifact?.id, previewThread);
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
      setContent((current) => (current === text ? "" : current));
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
        composer: kind === "message" || kind === "playground" ? text : null,
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
          ...(artifact ? { artifact_id: artifact.id } : {}),
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
        kind === "message" &&
        !submission.current &&
        composerEdits.current === composerVersion
      ) {
        setContent((current) => current || text);
        setPendingMessage(undefined);
      } else if (kind === "message" && !submission.current) {
        retryUnsentMessage.current = () =>
          void submit(kind, text, baseline, extra);
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
    const requestKind = JSON.parse(request.body).kind;
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
      navigate("checks", admitted.source?.artifact_id);
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
    if (!session.operations.some(operation => operation.retry_of_operation_id === request.retryOperationID)) return;
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

  // Continue only a server-approved quick check, once per artifact. The stable
  // client ID protects a resumed page or a second tab from admitting it twice.
  // A rejected continuation stays reviewable and can be retried deliberately.
  useEffect(() => {
    if (
      !quickCheck ||
      busy ||
      dirtyArtifact ||
      !config ||
      sending.current ||
      submission.current ||
      quickCheckAttempts.current.has(quickCheck.id)
    )
      return;
    const current = quickCheck;
    quickCheckAttempts.current.add(current.id);
    setAutoCheckID(current.id);
    void (async () => {
      try {
        await submit("check", "", undefined, {
          client_id: quickCheckClientID(current.id),
          artifact_id: current.id,
          evidence_set_id: current.conversation_evaluation!.evidence_set_id,
        });
      } catch (error) {
        setError((error as Error).message);
      } finally {
        setAutoCheckID(undefined);
        setBlockedAutoCheckID(current.id);
      }
    })();
    // Eligibility and availability trigger this continuation; the artifact ID
    // guard prevents another request when snapshots or model settings change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [quickCheck, busy, dirtyArtifact, config]);
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
  async function retryOperation(id: string) {
    if (submission.current?.retryOperationID === id) {
      await retrySubmission();
      return;
    }
    if (!session || sending.current || busy || dirtyArtifact || submission.current) return;
    const operation = session.operations.find(item => item.id === id);
    if (!operation?.retryable || operation.completion_receipt) return;
    submission.current = {
      sessionID: session.id,
      retryOperationID: id,
      body: JSON.stringify({ client_id: crypto.randomUUID(), revision: session.revision }),
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
  async function attachEvidence(input: {
    content?: string;
    label?: string;
    parent_id?: string;
    roles?: Record<string, string>;
  }) {
    if (busy) return false;
    setPending(true);
    setPendingAction("Adding your answer…");
    setError("");
    try {
      const v = await ensureSession();
      const result = await vibeFetch<Session>(
        `/sessions/${v.id}/evidence`,
        await token(),
        {
          method: "POST",
          body: JSON.stringify({ revision: v.revision, ...input }),
        },
      );
      setSession(result);
      return true;
    } catch (e) {
      setError((e as Error).message);
      return false;
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
    setSaveTarget(selected?.kind === "test_suite" ? undefined : operation);
    setSavedCheck(undefined);
    setError("");
    setSaveOpen(true);
    const auth = await token();
    if (!auth) return;
    try {
      const me =
        await createApiClient(auth).get<UserMeResponse>("/v1/users/me");
      const available = me.organizations.flatMap((o) =>
        o.workspaces.filter(
          (w) =>
            ["workspace_admin", "workspace_member"].includes(w.role) ||
            o.role === "org_admin",
        ),
      );
      setWorkspaces(available);
      if (!workspace && available[0]) setWorkspace(available[0].id);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function save() {
    if (!session || !artifact || !workspace) return;
    setPending(true);
    setError("");
    try {
      const auth = await token();
      await vibeFetch(`/sessions/${session.id}/claim`, auth, {
        method: "POST",
        body: "{}",
      });
      const latest = await reload();
      if (!latest) return;
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
          models,
        }),
      });
      await reload();
      const kept = await vibeFetch<SavedCheck[]>("/saved-checks", auth);
      setSavedChecks(kept);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setPending(false);
    }
  }

  const composerIsTrial = view === "try";
  const setComposerText = (text: string) => {
    if (composerIsTrial) {
      const key = trialDraftKey;
      trialEdits.current[key] = (trialEdits.current[key] || 0) + 1;
      setTrialBuffers((old) => ({ ...old, [key]: text }));
    } else {
      composerEdits.current++;
      setContent(text);
    }
  };
  return (
    <main className="vibe-workspace dark flex h-dvh flex-col overflow-hidden font-sans">
      <EvaluationWorkspace
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
        onRetry={(id) => void retryOperation(id)}
        retryPendingOperationID={pending ? submission.current?.retryOperationID : undefined}
        retryUncertainOperationID={uncertain ? submission.current?.retryOperationID : undefined}
        checkingTestChanges={pendingEdit?.artifactID === artifact?.id && !!pendingEdit}
        pendingMessage={pendingMessage}
        requestedRunID={requestedRunID}
        quickChecking={quickChecking}
        pendingLabel={
          (loadingSession && !session ? "Loading your conversation…" : undefined) ||
          pendingAction ||
          (pending && !active
            ? runPending || autoCheckID
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
        onSend={(context) =>
          void submit("message", content, undefined, {
            ...context,
            ...(testJourney ? {} : { quick_check: !buildEvidence.current }),
          })
        }
        onMessage={(text, operation, instructions) => {
          navigate("build");
          void submit("message", text, undefined, {
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
        onRun={(baseline, evidenceID) => {
          void submit(baseline ? "retest" : "check", "", baseline, {
            ...(evidenceID ? { evidence_set_id: evidenceID } : {}),
            ...(baseline?.source?.kind === "provided_conversations"
              ? { artifact_id: baseline.source.artifact_id }
              : {}),
          });
        }}
        onAttach={attachEvidence}
        onEdit={edit}
        onDirty={(dirty) => {
          setChecksDirtyID(dirty ? artifact?.id || null : null);
          if (dirty && artifact) setSelectedArtifactID(artifact.id);
        }}
        onSave={openSave}
        onSettings={() => setSettingsOpen(true)}
        onImport={() => file.current?.click()}
        onInstructions={() => setInstructionsOpen((open) => !open)}
        loadEvidence={(id, key) =>
          token().then((auth) =>
            vibeFetch<CaseResult>(
              `/operations/${id}/case?key=${encodeURIComponent(key)}`,
              auth,
            ),
          )
        }
        onAction={operationAction}
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
                      ? "Your tests are saved. Find them in History whenever you update your agent."
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
        instructions={
          artifact &&
          !testJourney &&
          artifact.kind !== "conversation_evaluation" &&
          artifact.kind !== "test_plan" ? (
            <div hidden={!instructionsOpen}>
              <ArtifactPanel
                key={artifact.id}
                artifact={artifact}
                capabilities={config?.capabilities || []}
                requirements={session?.document.requirements || []}
                busy={busy || checksDirtyID === artifact.id}
                onEdit={(agent_prompt) =>
                  edit({ artifact_id: artifact.id, agent_prompt })
                }
                onDirtyChange={(dirty) => {
                  setDirtyArtifactID(dirty ? artifact.id : null);
                  if (dirty) setSelectedArtifactID(artifact.id);
                }}
                onRequirement={(requirement_id, status, statement) =>
                  edit({ requirement_id, status, statement })
                }
              />
            </div>
          ) : session?.document.requirements.length ? (
            <details>
              <summary className="cursor-pointer text-sm vibe-muted">
                Requirements and assumptions
              </summary>
              <Requirements
                requirements={session.document.requirements}
                busy={busy}
                onRequirement={(requirement_id, status, statement) =>
                  edit({ requirement_id, status, statement })
                }
              />
            </details>
          ) : null
        }
        preview={
          <section className="space-y-6">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <VibeButton variant="quiet" onClick={() => navigate("build")}>
                ← Back to Vibe Evals
              </VibeButton>
              <VibeButton disabled={busy} onClick={newTrial}>
                New conversation
              </VibeButton>
            </div>
            <div>
              <h1 className="text-2xl font-semibold">Try a message</h1>
              <p className="mt-2 text-sm vibe-muted">
                Talking to the agent from these instructions. Text only; live
                tools are not connected.
              </p>
            </div>
            {trialHistory.length > 0 && (
              <label className="block text-sm vibe-muted">
                Conversation{" "}
                <select
                  className="ml-2 rounded-lg border border-[var(--vibe-border)] bg-transparent p-2"
                  aria-label="Trial conversation"
                  value={
                    trialHistory.some((t) => t.id === threadID) ? threadID : ""
                  }
                  disabled={busy}
                  onChange={(e) => {
                    const next = trialHistory.find(
                      (t) => t.id === e.target.value,
                    );
                    setThreadID(e.target.value);
                    if (next?.model)
                      setModels((old) => ({ ...old, target: next.model! }));
                    navigate("try", artifact?.id, e.target.value);
                  }}
                >
                  <option value="">New conversation</option>
                  {trialHistory.map((t, i) => (
                    <option key={t.id} value={t.id}>
                      {t.legacy ? "Earlier trial" : "Conversation"} {i + 1}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <div
              role="log"
              aria-label="Trial conversation"
              className="space-y-6"
            >
              {trialMessages.map((m) => (
                <div
                  key={m.id}
                  className={
                    m.role === "user"
                      ? "vibe-message vibe-message-user"
                      : "vibe-message"
                  }
                >
                  <p className="mb-2 text-xs vibe-muted">
                    {m.role === "user" ? "You" : "Agent"}
                  </p>
                  <AgentReply>{m.content}</AgentReply>
                </div>
              ))}
            </div>
            {!trialMessages.length &&
              (evaluation?.scenarios?.[0]?.input ||
                evaluation?.examples[0]) && (
                <VibeButton
                  variant="quiet"
                  onClick={() =>
                    setComposerText(
                      evaluation?.scenarios?.[0]?.input ||
                        evaluation?.examples[0] ||
                        "",
                    )
                  }
                >
                  Use an example message
                </VibeButton>
              )}
            {legacyTrial ? (
              <p className="text-sm vibe-muted">
                Start a new conversation to try follow-ups. This earlier trial
                used one message.
              </p>
            ) : (
              <form
                className="vibe-composer"
                onSubmit={(e) => {
                  e.preventDefault();
                  if (trialText.trim()) void submit("playground", trialText);
                }}
              >
                <label
                  htmlFor="vibe-trial-message"
                  className="mb-2 block text-xs vibe-muted"
                >
                  Message your agent
                </label>
                <textarea
                  id="vibe-trial-message"
                  aria-label="Message your agent"
                  value={trialText}
                  onChange={(e) => setComposerText(e.target.value)}
                  placeholder="Send a customer message…"
                  onKeyDown={(e) =>
                    sendOnEnter(e, () => {
                      if (!busy && !dirtyArtifact && trialText.trim()) {
                        void submit("playground", trialText);
                      }
                    })
                  }
                />
                <div className="vibe-composer-actions">
                  <span className="text-xs vibe-muted">
                    Separate from your conversation with Vibe Evals
                  </span>
                  <VibeButton
                    variant="primary"
                    type="submit"
                    disabled={busy || dirtyArtifact || !trialText.trim()}
                  >
                    Send to agent
                    <ArrowUp />
                  </VibeButton>
                </div>
              </form>
            )}
          </section>
        }
      />
      <Dialog open={settingsOpen} onOpenChange={setSettingsOpen}>
        <DialogContent className="vibe-workspace max-h-[85vh] overflow-y-auto">
          <DialogTitle>Settings and details</DialogTitle>
          <DialogDescription>
            Choose models, review versions, or export your work.
          </DialogDescription>
          {workspace && <CreditsDialog workspace={workspace} />}
          {artifact &&
            artifact.kind !== "test_plan" &&
            artifact.kind !== "conversation_evaluation" && (
              <VibeButton
                disabled={busy || dirtyArtifact}
                onClick={() => {
                  setSettingsOpen(false);
                  void openSave();
                }}
              >
                {testJourney ? "Keep these tests" : "Save agent"}
              </VibeButton>
            )}
          {savedDraft && (
            <Link
              className="text-sm underline"
              href={`/workspaces/${savedDraft.workspace_id}/challenge-packs/builder/${savedDraft.draft_id}`}
            >
              Open saved evaluation
            </Link>
          )}
          {(
            [
              ["Assistant", "assistant"],
              ["Agent", "target"],
              ["Evaluator", "evaluator"],
            ] as const
          )
            .filter(
              ([, role]) =>
                role !== "target" ||
                artifact?.kind !== "conversation_evaluation",
            )
            .map(([label, role]) => (
              <div key={role}>
                <ModelSelect
                  label={label}
                  value={models[role]}
                  models={config?.models || []}
                  disabled={
                    busy ||
                    (role === "evaluator" && session?.anonymous !== false)
                  }
                  onChange={(value) =>
                    changeModels({ ...models, [role]: value })
                  }
                />
                <p className="mt-1 text-xs text-builder-fg-muted">
                  {role === "assistant"
                    ? "Plans the check and helps explain results."
                    : role === "target"
                      ? "Generates replies when testing instructions here. Changing it starts a fresh preview conversation."
                      : "Grades replies against your expectations. The free trial keeps this fixed."}
                </p>
              </div>
            ))}
          {artifacts.length > 1 && (
            <label className="text-sm">
              Agent version
              <select
                aria-label="Agent version"
                value={artifact?.id}
                disabled={busy || dirtyArtifact}
                className="mt-2 w-full rounded-lg border bg-background p-2"
                onChange={(e) => {
                  setSelectedArtifactID(e.target.value);
                  setThreadID("");
                  navigate(view, e.target.value, "");
                }}
              >
                {artifacts.map((a, i) => (
                  <option key={a.id} value={a.id}>
                    Version {i + 1}: {a.title}
                  </option>
                ))}
              </select>
            </label>
          )}
          <Button
            variant="outline"
            disabled={busy || dirtyArtifact}
            onClick={() => file.current?.click()}
          >
            <Paperclip size={16} /> Import an evaluation
          </Button>
          {artifact &&
            artifact.kind !== "test_plan" &&
            artifact.kind !== "conversation_evaluation" && (
              <Button
                variant="outline"
                onClick={() => exportAgent(artifact, models)}
              >
                {testJourney
                  ? "Export tests and agent instructions"
                  : "Export agent and checks"}
              </Button>
            )}
          {session && (
            <Button variant="ghost" onClick={exportConversation}>
              Export conversation
            </Button>
          )}
          <details className="text-sm">
            <summary className="cursor-pointer">
              Available capabilities and documentation
            </summary>
            {config?.capabilities?.map((c) => (
              <p key={c.id} className="mt-3 text-xs leading-5">
                <strong>{c.label}:</strong> {c.description}{" "}
                {c.url && (
                  <a
                    href={c.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline"
                  >
                    Documentation ↗
                  </a>
                )}
              </p>
            ))}
          </details>
        </DialogContent>
      </Dialog>
      <input
        ref={file}
        type="file"
        accept=".json,.yaml,.yml"
        className="hidden"
        aria-label="Evaluation file"
        onChange={(e) => upload(e.target.files?.[0])}
      />
      <Dialog open={saveOpen} onOpenChange={setSaveOpen}>
        <DialogContent className="vibe-workspace">
          <DialogTitle>
            {testJourney
              ? "Keep these tests"
              : saveTarget
                ? "Save this check"
                : "Save agent and checks"}
          </DialogTitle>
          <DialogDescription>
            {testJourney
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
          {workspaces.length ? (
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
                  (saveTarget ? !!savedCheck : saved)
                }
              >
                {saveTarget
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
              href={`/auth/login?returnTo=${encodeURIComponent(`/vibe-evals?session=${sessionID || ""}`)}`}
              className="rounded-lg bg-primary px-4 py-3 text-center text-sm text-primary-foreground"
            >
              Sign in to save your work
            </Link>
          )}
          {savedCheck && (
            <p role="status" className="text-sm">
              Saved. Find it in History whenever you need it.
            </p>
          )}
          {!saveTarget && savedDraft && (
            <>
              {!saved && (
                <p className="text-xs text-builder-fg-muted">
                  {savedModelNotice}
                </p>
              )}
              <Link
                href={
                  testJourney
                    ? `/vibe-evals?session=${sessionID}&agent=${session?.saved_artifact_id || artifact?.id}`
                    : `/workspaces/${savedDraft.workspace_id}/challenge-packs/builder/${savedDraft.draft_id}`
                }
                className="text-sm underline"
              >
                {testJourney ? "Open saved tests" : "Open your evaluation"}
              </Link>
            </>
          )}
        </DialogContent>
      </Dialog>
    </main>
  );
}
