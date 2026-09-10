"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  ArrowUp,
  Loader2,
  MessageSquarePlus,
  Paperclip,
  PanelRight,
  Square,
} from "lucide-react";
import { ClashMark } from "@/components/marketing/clash-mark";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { CreditsDialog } from "@/components/vibe/credits-dialog";
import { TestPlanPanel } from "@/components/vibe/test-plan-panel";
import { ArtifactPanel, ModelSelect } from "@/components/vibe/artifact-panel";
import { SafeMarkdown } from "@/components/vibe/safe-markdown";
import { VibeScorecard } from "@/components/vibe/scorecard";
import { Requirements } from "@/components/vibe/requirements";
import { createApiClient } from "@/lib/api/client";
import type { UserMeResponse } from "@/lib/api/types";
import {
  defaultModels,
  dollars,
  terminal,
  VibeError,
  vibeFetch,
  watchVibe,
  type CaseResult,
  type Models,
  type Operation,
  type Session,
  type VibeConfig,
} from "@/lib/vibe";

// POST /messages faults from api/vibe.go, Service.Prepare and Store.Submit:
// these exact code/status pairs reject before admission or roll back its transaction.
// Unknown responses (including generic 503s) cannot prove non-admission.
const submissionRejections: Partial<Record<number, readonly string[]>> = {
  400: [
    "invalid_request",
    "invalid_message",
    "invalid_operation",
    "invalid_import",
    "import_limit",
    "unsupported_schema",
    "invalid_encoding",
    "free_model_required",
    "unsupported_model",
    "evaluator_pinned",
    "artifact_required",
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

const starters = [
  "I have an agent that needs testing",
  "Help me build an agent",
  "I’m figuring out what AI could do for us",
];
export function VibeClient() {
  const params = useSearchParams();
  const requestedSessionID = params.get("session");
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [loadingSession, setLoadingSession] = useState(!!requestedSessionID);
  const { getAccessToken } = useAccessToken();
  const [session, setSession] = useState<Session | null>(null);
  const [config, setConfig] = useState<VibeConfig | null>(null);
  const [models, setModels] = useState<Models>(defaultModels);
  const [content, setContent] = useState("");
  const [journeyChoice, setJourneyChoice] = useState<
    "idea" | "existing" | "exploring"
  >();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [connection, setConnection] = useState("");
  const [panel, setPanel] = useState(false);
  const [dirtyArtifactID, setDirtyArtifactID] = useState<string | null>(null);
  const [saveOpen, setSaveOpen] = useState(false);
  const [workspaces, setWorkspaces] = useState<{ id: string; name: string }[]>(
    [],
  );
  const [workspace, setWorkspace] = useState(params.get("workspace") || "");

  const sending = useRef(false);
  const submission = useRef<{
    sessionID: string;
    body: string;
    composer: string | null;
    composerVersion: number;
    uncertain: boolean;
  } | null>(null);
  const composerEdits = useRef(0);
  const [uncertain, setUncertain] = useState(false);
  const file = useRef<HTMLInputElement>(null);
  const scrollEnd = useRef<HTMLDivElement>(null);
  const active = session?.operations.find((o) => !terminal(o.state));
  const sessionUnavailable =
    !!requestedSessionID && session?.id !== requestedSessionID;
  const busy = pending || !!active || uncertain || sessionUnavailable;
  const journey = session?.document.journey;
  const artifact = session?.document.artifacts
    .filter(
      (a) =>
        journey?.mode !== "existing" ||
        journey.preview_consent ||
        a.kind === "test_plan",
    )
    .at(-1);
  const dirtyArtifact = panel && !!artifact && dirtyArtifactID === artifact.id;
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
    ? "Your current model choices differ from those recorded when this draft was saved."
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
    let alive = true;
    vibeFetch<VibeConfig>("/config")
      .then((c) => {
        if (alive) {
          setConfig(c);
          if (!params.get("session")) setModels(c.defaults);
        }
      })
      .catch(() => {
        if (alive) setError("Vibe is not connected to the local backend yet.");
      });
    return () => {
      alive = false;
    };
  }, [params]);
  useEffect(() => {
    const id = params.get("session");
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
          setModels(v.document.models);
        }
      } catch (e) {
        if (alive) setError((e as Error).message);
      } finally {
        if (alive) setLoadingSession(false);
      }
    })();
    return () => {
      alive = false;
    };
  }, [params, token, loadAttempt]);
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
      } catch {
        if (!controller.signal.aborted)
          setConnection("Reconnecting to saved progress…");
      }
      if (!controller.signal.aborted) timer = setTimeout(connect, 2000);
    };
    void connect();
    return () => {
      controller.abort();
      clearTimeout(timer);
    };
  }, [sessionID, token]);
  useEffect(() => {
    scrollEnd.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [session?.document.messages.length, active?.state]);
  const artifactID = artifact?.id;
  useEffect(() => {
    if (artifactID) setPanel(true);
  }, [artifactID]); // open new proposals, preserve user dismissal

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
    const v = await vibeFetch<Session>("/sessions", await token(), {
      method: "POST",
      body: JSON.stringify({
        id,
        ...(workspace ? { workspace_id: workspace } : {}),
      }),
    });
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
  ) {
    if (sending.current || busy || submission.current) return;
    if (kind !== "message" && (dirtyArtifact || !artifact?.accepted)) return;
    const composerVersion = composerEdits.current;
    sending.current = true;
    setPending(true);
    setError("");
    try {
      const v = await ensureSession();
      submission.current = {
        sessionID: v.id,
        uncertain: false,
        composer: kind === "message" ? text : null,
        composerVersion,
        body: JSON.stringify({
          client_id: crypto.randomUUID(),
          revision: v.revision,
          kind,
          ...(kind === "message" && !v.document.journey?.mode && journeyChoice
            ? { journey_mode: journeyChoice }
            : {}),
          content: text,
          models: baseline
            ? { ...models, evaluator: baseline.models.evaluator }
            : models,
          ...(artifact ? { artifact_id: artifact.id } : {}),
          ...(baseline ? { baseline_id: baseline.id } : {}),
        }),
      };
      await dispatchSubmission();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      sending.current = false;
      setPending(false);
    }
  }
  async function dispatchSubmission() {
    const request = submission.current;
    if (!request) return;
    try {
      await vibeFetch(
        `/sessions/${request.sessionID}/messages`,
        await token(),
        {
          method: "POST",
          body: request.body,
        },
      );
    } catch (e) {
      // Auth/rate/profile checks precede idempotency lookup. A later rejection
      // cannot disprove an earlier admission: retain every byte until acknowledged.
      const rejected =
        !request.uncertain &&
        e instanceof VibeError &&
        e.status !== undefined &&
        submissionRejections[e.status]?.includes(e.code) === true;
      if (rejected) submission.current = null;
      else request.uncertain = true;
      setUncertain(!rejected);
      if (e instanceof VibeError && e.code === "revision_conflict") {
        await reload(request.sessionID).catch(() => undefined);
      }
      throw e;
    }
    submission.current = null;
    setUncertain(false);
    if (
      request.composer !== null &&
      request.composerVersion === composerEdits.current
    ) {
      setContent((current) => (current === request.composer ? "" : current));
    }
    await reload(request.sessionID);
  }
  async function retrySubmission() {
    if (sending.current || pending || !submission.current) return;
    sending.current = true;
    setPending(true);
    setError("");
    try {
      await dispatchSubmission();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      sending.current = false;
      setPending(false);
    }
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
  async function openSave() {
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
      await vibeFetch<{
        draft_id: string;
        workspace_id: string;
      }>(`/sessions/${session.id}/save`, auth, {
        method: "POST",
        body: JSON.stringify({
          revision: latest.revision,
          artifact_id: artifact.id,
          workspace_id: workspace,
          models,
        }),
      });
      await reload();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="dark flex h-dvh overflow-hidden bg-background font-sans text-builder-fg">
      <nav
        aria-label="Vibe navigation"
        className="hidden w-56 shrink-0 flex-col border-r border-builder-border bg-sidebar px-4 py-6 md:flex"
      >
        <Link
          href="/"
          className="mb-8 flex items-center gap-2 text-sm font-semibold"
        >
          <ClashMark className="size-5" /> AgentClash
        </Link>
        <Button
          variant="outline"
          className="justify-start"
          onClick={() => {
            window.location.href = "/vibe-evals";
          }}
        >
          <MessageSquarePlus size={15} /> New conversation
        </Button>
        <p className="mt-8 px-2 font-mono text-[10px] uppercase tracking-widest text-builder-fg-subtle">
          Vibe Evals
        </p>
        <p className="mt-3 px-2 text-xs leading-6 text-builder-fg-muted">
          A conversation about making your agent better.
        </p>
        <div className="mt-auto space-y-3 border-t border-builder-border px-2 pt-4 text-xs text-builder-fg-muted">
          <p>Free trial includes a small check and one retest.</p>
          <Link href="/dashboard" className="block hover:text-builder-fg">
            Your workspace ↗
          </Link>
        </div>
      </nav>
      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-16 shrink-0 items-center justify-between border-b border-builder-border px-5">
          <Link href="/vibe-evals" className="text-sm font-medium">
            Vibe Evals{" "}
            <span className="ml-2 rounded border border-builder-border px-1.5 py-0.5 font-mono text-[9px] text-builder-fg-muted">
              PREVIEW
            </span>
          </Link>
          <div className="flex gap-2">
            {workspace && <CreditsDialog workspace={workspace} />}
            <Link
              href="/"
              className="p-2 text-xs text-builder-fg-muted md:hidden"
            >
              AgentClash
            </Link>
            {artifact && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setDirtyArtifactID(null);
                  setPanel(!panel);
                }}
              >
                <PanelRight size={15} />{" "}
                {artifact.kind === "test_plan" ? "Test plan" : "Your agent"}
              </Button>
            )}
          </div>
        </header>
        <div className="min-h-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-3xl px-5 py-8 sm:px-8">
            {!session?.document.messages.length && (
              <div className="pb-10 pt-[min(12vh,100px)]">
                <ClashMark className="mb-7 size-8 text-builder-fg-muted" />
                <h1 className="max-w-xl text-3xl font-semibold tracking-tight sm:text-4xl">
                  What are you working on?
                </h1>
                <p className="mt-4 max-w-xl text-sm leading-7 text-builder-fg-muted">
                  Tell me what your agent does, what you want to build, or what
                  isn’t working. We’ll figure out the next step together.
                </p>
                <div className="mt-8 flex flex-wrap gap-2">
                  {starters.map((starter, index) => (
                    <button
                      key={starter}
                      onClick={() => {
                        setContent(starter);
                        setJourneyChoice(
                          index === 0
                            ? "existing"
                            : index === 1
                              ? "idea"
                              : "exploring",
                        );
                      }}
                      className="rounded-xl border border-builder-border px-3 py-2.5 text-xs text-builder-fg-muted transition-colors hover:bg-builder-surface-hover hover:text-builder-fg"
                    >
                      {starter}
                    </button>
                  ))}
                </div>
              </div>
            )}
            <div className="space-y-8" role="log" aria-label="Conversation">
              {session?.document.messages.map((message) => (
                <div
                  key={message.id}
                  className={
                    message.role === "user"
                      ? "ml-auto max-w-[88%] rounded-2xl bg-builder-fg px-4 py-2 text-background"
                      : "pr-4"
                  }
                >
                  {message.role !== "user" && (
                    <p className="mb-2 flex items-center gap-2 text-xs font-medium">
                      <ClashMark className="size-4" />{" "}
                      {message.origin === "playground"
                        ? "Agent preview · Customer trial"
                        : "AgentClash · Design"}
                    </p>
                  )}
                  {message.role === "user" &&
                    message.origin === "playground" && (
                      <p className="mb-1 text-[10px]">Customer trial</p>
                    )}
                  <SafeMarkdown>{message.content}</SafeMarkdown>
                </div>
              ))}
              {!artifact && (
                <Requirements
                  requirements={session?.document.requirements || []}
                  busy={busy}
                  onRequirement={(requirement_id, status, statement) =>
                    edit({ requirement_id, status, statement })
                  }
                />
              )}
              {session?.operations.map((operation) => (
                <div key={operation.id}>
                  {operation.scorecard && operation.scorecard.total > 0 && (
                    <VibeScorecard
                      operation={operation}
                      eventCursor={session.event_cursor}
                      baseline={session?.operations.find(
                        (o) => o.id === operation.baseline_id,
                      )}
                      loadEvidence={async (key) =>
                        vibeFetch<CaseResult>(
                          `/operations/${operation.id}/case?key=${encodeURIComponent(key)}`,
                          await token(),
                        )
                      }
                      busy={
                        busy ||
                        dirtyArtifact ||
                        !artifact?.accepted ||
                        artifact.kind === "test_plan"
                      }
                      onImprove={() => {
                        setContent(
                          "Help me improve the accepted agent instructions while keeping the evaluation unchanged. Ask me for any missing policy facts.",
                        );
                      }}
                      onRetest={() => submit("retest", "", operation)}
                    />
                  )}
                  {operation.state === "AWAITING_APPROVAL" && (
                    <div className="rounded-xl border border-builder-border p-5">
                      <h2 className="text-sm font-semibold">
                        Ready when you are
                      </h2>
                      <p className="my-3 text-sm text-builder-fg-muted">
                        This operation will cost at most{" "}
                        {dollars(operation.max_cost_nano_usd)}. We’ll hold that
                        amount and settle the actual provider spend when it
                        finishes.
                      </p>
                      <Button
                        size="sm"
                        disabled={pending}
                        onClick={() => operationAction(operation.id, "approve")}
                      >
                        Run for up to {dollars(operation.max_cost_nano_usd)}
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={pending}
                        onClick={() => operationAction(operation.id, "stop")}
                      >
                        Dismiss
                      </Button>
                    </div>
                  )}
                  {operation.error && (
                    <p className="mt-3 text-xs leading-5 text-builder-warn">
                      {operation.id !== session.operations.at(-1)?.id && (
                        <span className="mr-1 font-medium">
                          Earlier{" "}
                          {operation.kind === "playground"
                            ? "customer trial"
                            : "design/evaluation request"}
                          :
                        </span>
                      )}
                      {operation.error.message}
                      {operation.error.context && (
                        <>
                          {artifact && (
                            <button
                              className="ml-2 underline"
                              onClick={() => setPanel(true)}
                            >
                              Review draft and requirements
                            </button>
                          )}
                          <button
                            className="ml-2 underline"
                            onClick={exportConversation}
                          >
                            Export preserved conversation
                          </button>
                        </>
                      )}
                    </p>
                  )}
                  {operation.state === "CANCELLED" && (
                    <p className="mt-3 text-xs text-builder-fg-muted">
                      Execution: cancelled · Billing:{" "}
                      {operation.billing.toLowerCase()}.{" "}
                      {operation.billing === "RECONCILING" &&
                        "A request already sent to the provider may still be billed; its reservation stays held."}
                    </p>
                  )}
                </div>
              ))}
              {(pending ||
                (active && active.state !== "AWAITING_APPROVAL")) && (
                <div
                  role="status"
                  className="flex items-center gap-3 text-sm text-builder-fg-muted"
                >
                  <Loader2 className="size-4 animate-spin" />
                  {active?.kind === "check" || active?.kind === "retest"
                    ? "Running the examples and checking the evidence…"
                    : "Working on your next step…"}
                  {active && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => operationAction(active.id, "stop")}
                      disabled={pending}
                    >
                      <Square size={11} /> Stop
                    </Button>
                  )}
                </div>
              )}
              {savedDraft && (
                <p className="rounded-xl border border-builder-border p-4 text-sm">
                  {saved ? "Your evaluation is saved." : savedModelNotice}{" "}
                  <Link
                    href={`/workspaces/${savedDraft.workspace_id}/challenge-packs/builder/${savedDraft.draft_id}`}
                    className="underline underline-offset-4"
                  >
                    Open it in your workspace
                  </Link>{" "}
                  to expand the tests and run your connected agent. Production
                  monitoring is a separate setup.
                </p>
              )}
            </div>
            <div ref={scrollEnd} />
          </div>
        </div>
        <div className="mx-auto w-full max-w-3xl shrink-0 px-5 pb-5 pt-3 sm:px-8">
          {sessionUnavailable && (
            <div className="mb-3 text-xs text-builder-fg-muted">
              {loadingSession ? (
                <p role="status">Loading your conversation…</p>
              ) : (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setError("");
                    setLoadAttempt((attempt) => attempt + 1);
                  }}
                >
                  Retry loading conversation
                </Button>
              )}
            </div>
          )}
          {error && (
            <p
              role="alert"
              className="mb-3 text-xs leading-5 text-builder-warn"
            >
              {error}
            </p>
          )}
          {connection && (
            <p role="status" className="mb-2 text-xs text-builder-fg-muted">
              {connection}
            </p>
          )}
          {uncertain && (
            <div className="mb-3 text-xs text-builder-fg-muted">
              <p>
                The submission acknowledgement was not confirmed. Retry the same
                submission to recover its saved status. Your next message stays
                here.
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={pending}
                onClick={retrySubmission}
              >
                Retry submission
              </Button>
            </div>
          )}
          <p className="mb-2 text-xs font-medium text-builder-fg-muted">
            Design · Discuss or revise your agent and tests
          </p>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void submit();
            }}
            className="rounded-2xl border border-builder-border bg-builder-surface p-3 focus-within:border-builder-border-strong"
          >
            <textarea
              aria-label="Message Vibe Evals"
              placeholder="Describe your agent, share an idea, or ask a question…"
              value={content}
              onChange={(e) => {
                composerEdits.current++;
                setContent(e.target.value);
              }}
              onKeyDown={(e) => {
                if (
                  e.key === "Enter" &&
                  !e.shiftKey &&
                  !e.nativeEvent.isComposing
                ) {
                  e.preventDefault();
                  if (content.trim()) void submit();
                }
              }}
              className="max-h-48 min-h-16 w-full resize-y bg-transparent px-1 py-1 text-sm leading-6 outline-none placeholder:text-builder-fg-subtle"
              maxLength={session?.anonymous === false ? 65536 : 16384}
            />
            <div className="mt-2 flex items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label="Import an evaluation"
                  disabled={busy}
                  onClick={() => file.current?.click()}
                >
                  <Paperclip size={16} />
                </Button>
                <input
                  ref={file}
                  type="file"
                  accept=".json,.yaml,.yml"
                  className="hidden"
                  aria-label="Evaluation file"
                  onChange={(e) => upload(e.target.files?.[0])}
                />
                <ModelSelect
                  label="Assistant"
                  value={models.assistant}
                  models={config?.models || []}
                  onChange={(assistant) => setModels({ ...models, assistant })}
                  disabled={busy}
                />
              </div>
              <Button
                type="submit"
                size="icon"
                aria-label="Send message"
                disabled={busy || !content.trim()}
                className="rounded-full"
              >
                <ArrowUp size={18} />
              </Button>
            </div>
          </form>
          <p className="mt-3 text-center text-[10px] text-builder-fg-subtle">
            Private by default. You review drafts; AgentClash counts the
            results.
          </p>
        </div>
      </div>
      {artifact &&
        panel &&
        (artifact.kind === "test_plan" ? (
          <TestPlanPanel
            artifact={artifact}
            journey={journey}
            capabilities={config?.capabilities || []}
            requirements={session?.document.requirements || []}
            busy={busy}
            onClose={() => setPanel(false)}
            onRequirement={(requirement_id, status, statement) =>
              edit({ requirement_id, status, statement })
            }
            onPreview={async () => {
              if (await edit({ preview_consent: true }))
                setContent(
                  "Create a prompt-only surrogate for a text preview. I understand this does not test my connected agent.",
                );
            }}
          />
        ) : (
          <ArtifactPanel
            key={artifact.id}
            artifact={artifact}
            requirements={session?.document.requirements || []}
            capabilities={config?.capabilities || []}
            models={models}
            choices={config?.models || []}
            anonymous={session?.anonymous ?? true}
            busy={busy}
            onClose={() => {
              setDirtyArtifactID(null);
              setPanel(false);
            }}
            onAccept={() => edit({ artifact_id: artifact.id })}
            onEdit={(agent_prompt) =>
              edit({ artifact_id: artifact.id, agent_prompt })
            }
            onEvaluationEdit={(evaluation) =>
              edit({ artifact_id: artifact.id, evaluation })
            }
            onDirtyChange={(dirty) =>
              setDirtyArtifactID(dirty ? artifact.id : null)
            }
            onRequirement={(requirement_id, status, statement) =>
              edit({ requirement_id, status, statement })
            }
            onModels={setModels}
            onCheck={() => submit("check", "")}
            onPlay={(text) => submit("playground", text)}
            onSave={openSave}
          />
        ))}
      <Dialog open={saveOpen} onOpenChange={setSaveOpen}>
        <DialogContent>
          <DialogTitle>Keep what worked</DialogTitle>
          <DialogDescription>
            Save your agent instructions and editable evaluation. Future checks
            in this conversation use your workspace’s AI credits. Your original
            evidence stays here.
          </DialogDescription>
          {error && (
            <p role="alert" className="text-xs leading-5 text-builder-warn">
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
                  busy || dirtyArtifact || !artifact?.accepted || !!saved
                }
              >
                {saved ? "Saved" : "Save to workspace"}
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
          {savedDraft && (
            <>
              {!saved && (
                <p className="text-xs leading-5 text-builder-fg-muted">
                  {savedModelNotice}
                </p>
              )}
              <Link
                href={`/workspaces/${savedDraft.workspace_id}/challenge-packs/builder/${savedDraft.draft_id}`}
                className="text-sm underline"
              >
                Open your evaluation
              </Link>
            </>
          )}
        </DialogContent>
      </Dialog>
    </main>
  );
}
