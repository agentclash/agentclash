"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import {
  ArrowRight,
  ArrowDown,
  ArrowUp,
  History,
  Plus,
  Settings,
  Square,
} from "lucide-react";
import { Tabs } from "@base-ui/react/tabs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ClashMark } from "@/components/marketing/clash-mark";
import {
  completionAcknowledgement,
  terminal,
  type Artifact,
  type CaseResult,
  type Operation,
  type SavedCheck,
  type Session,
} from "@/lib/vibe";
import type { ConversationAction } from "@/lib/vibe-conversation";
import { hasConversationChoice, primarySurface } from "@/lib/vibe-presentation";
import { ConversationActions } from "./conversation-actions";
import { ConversationGuidance, ExampleComparison } from "./conversation-guidance";
import { VibeButton } from "./vibe-button";
import { SafeMarkdown } from "./safe-markdown";
import { EvidenceIntake, exampleBrief } from "./evidence-intake";
import { ConversationProposal } from "./conversation-proposal";
import { PromptChange } from "./prompt-change";
import { ActivityStatus, conversationActivity } from "./activity-status";
import { useComposerAutosize } from "./use-composer-autosize";
import { sendOnEnter } from "./composer-keyboard";
import { VibeScorecard } from "./scorecard";
import { TestSuitePanel, exampleAgentDescription } from "./test-suite-panel";
import "./workspace.css";

type View = "build" | "try" | "checks";

export const exampleAnswer = `Check this answer from my trip-planning app. It should respect the total budget, including every listed cost.

User: Plan a one-day Jaipur trip for two people with a total budget of ₹5,000, including transport, food, and activities.
Assistant: Here’s your plan: transport ₹2,000, food ₹2,000, and activities ₹4,000. Total: ₹8,000. This fits within your ₹5,000 budget.`;

export type EvaluationWorkspaceProps = {
  interactionActions?: boolean;
  onChoice?: (action: ConversationAction) => Promise<void>;
  onReloadChoices?: () => Promise<void>;
  testJourney?: boolean;
  modelLabel?: string;
  session: Session | null;
  artifact?: Artifact;
  view: View;
  busy: boolean;
  pendingMessage?: { id: string; content: string };
  pendingLabel?: string;
  quickChecking?: boolean;
  requestedRunID?: string;
  dirty: boolean;
  content: string;
  onContent: (value: string) => void;
  onSend: (context?: { viewed_run_id: string }) => void;
  onMessage: (
    text: string,
    operation?: Operation,
    instructions?: string,
  ) => void;
  onNavigate: (view: View) => void;
  onRun: (baseline?: Operation, evidenceID?: string) => void;
  onAttach: (input: {
    content?: string;
    label?: string;
    parent_id?: string;
    roles?: Record<string, string>;
  }) => Promise<boolean>;
  onEdit: (fields: Record<string, unknown>) => Promise<boolean>;
  onDirty: (dirty: boolean) => void;
  onSave: (operation?: Operation) => void;
  onImport: () => void;
  onSettings: () => void;
  onInstructions: () => void;
  loadEvidence: (operation: string, key: string) => Promise<CaseResult>;
  onAction: (id: string, action: "stop" | "approve") => void;
  onRetry?: (id: string) => void;
  retryPendingOperationID?: string;
  retryUncertainOperationID?: string;
  checkingTestChanges?: boolean;
  onDispute: (rule: string, result: CaseResult, operation: Operation) => void;
  notice: ReactNode;
  preview: ReactNode;
  instructions: ReactNode;
  savedChecks: SavedCheck[];
};

export function EvaluationWorkspace(p: EvaluationWorkspaceProps) {
  const params = useSearchParams();
  const reduced = useReducedMotion();
  const [intake, setIntake] = useState<"chats" | "instructions" | null>(null);
  const [compare, setCompare] = useState<Operation>();
  const [intakeStartedWith, setIntakeStartedWith] = useState<string>();
  const [freshAnswer, setFreshAnswer] = useState(false);
  const [fromGenerated, setFromGenerated] = useState(false);
  const [instructionText, setInstructionText] = useState("");
  const [welcomeExample, setWelcomeExample] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [isNearBottom, setIsNearBottom] = useState(true);
  const [runSelection, setRunSelection] = useState({
    id: params.get("run") || undefined,
    request: p.requestedRunID,
  });
  const end = useRef<HTMLDivElement>(null);
  const scrollRegion = useRef<HTMLDivElement>(null);
  const composer = useRef<HTMLTextAreaElement>(null);
  const nearBottom = useRef(true);
  const parent = p.session?.document.artifacts.find(
    (a) => a.id === p.artifact?.parent_id,
  );
  const promptChanged =
    !!parent &&
    !!parent.agent_prompt.trim() &&
    !!p.artifact?.agent_prompt &&
    parent.agent_prompt !== p.artifact.agent_prompt;
  const changeBaseline =
    promptChanged &&
    JSON.stringify(parent.blueprint) === JSON.stringify(p.artifact?.blueprint)
      ? p.session?.operations.findLast(
          (o) =>
            (o.kind === "check" || o.kind === "retest") &&
            o.results.some((c) => c.version === parent.id) &&
            terminal(o.state),
        )
      : undefined;
  const runs =
    p.session?.operations.filter(
      (o) => o.kind === "check" || o.kind === "retest",
    ) || [];
  const latest = runs.at(-1);
  const runID =
    runSelection.request === p.requestedRunID
      ? runSelection.id || p.requestedRunID
      : p.requestedRunID;
  const result = runID ? runs.find((o) => o.id === runID) : latest;
  const active = p.session?.operations.find((o) => !terminal(o.state));
  const messages =
    p.session?.document.messages.filter((m) => m.origin !== "playground") || [];
  const operations = p.session?.operations || [];
  const operationByID = new Map(operations.map(operation => [operation.id, operation]));
  const latestOperations = new Map(operationByID);
  for (let index = operations.length - 1; index >= 0; index--) {
    const operation = operations[index];
    if (operation.retry_of_operation_id)
      latestOperations.set(operation.retry_of_operation_id, latestOperations.get(operation.id)!);
  }
  const pendingMessage =
    p.pendingMessage &&
    !messages.some((message) => message.id === p.pendingMessage!.id)
      ? { ...p.pendingMessage, role: "user" }
      : undefined;
  const welcome =
    !p.pendingLabel &&
    messages.length === 0 &&
    !p.artifact &&
    !pendingMessage &&
    runs.length === 0;
  const evidence = p.session?.document.evidence_sets?.find(
    (e) => e.id === p.session?.document.active_evidence_id,
  );
  const artifactEvidence = p.session?.document.evidence_sets?.find(
    (e) => e.id === p.artifact?.conversation_evaluation?.evidence_set_id,
  );
  const currentPane = p.view === "try" ? "build" : p.view;
  const hasResults = runs.some((run) => terminal(run.state));
  const artifactHasRun = runs.some(
    (r) =>
      r.source?.artifact_id === p.artifact?.id ||
      r.results.some((c) => c.version === p.artifact?.id),
  );
  const proposalMessage = messages.find(
    (m) => m.role === "assistant" && (p.artifact?.proposal_message_id ? m.id === p.artifact.proposal_message_id : m.artifact_id === p.artifact?.id),
  );
  const defaultRecentStart = p.quickChecking
    ? Math.max(
        0,
        messages.findLastIndex((message) => message.role === "user"),
      )
    : p.testJourney
      ? Math.max(0, messages.length - (messages.at(-1)?.role === "user" ? 3 : 4))
      : p.artifact && proposalMessage
        ? messages.findIndex((m) => m.id === proposalMessage.id)
        : 0;
  const recoveryTurn = messages.findIndex(message => {
    if (message.role !== "user" || !message.operation_id) return false;
    const original = operationByID.get(message.operation_id);
    const latest = latestOperations.get(message.operation_id);
    const unresolved = !latest?.completion_receipt && latest?.state !== "COMPLETED";
    const justRetried = !!latest?.retry_of_operation_id && latest.id === operations.at(-1)?.id;
    return !!original?.error && (unresolved || justRetried);
  });
  const recentStart = recoveryTurn < 0
    ? defaultRecentStart
    : Math.min(defaultRecentStart, recoveryTurn);
  const visibleMessages = messages
    .slice(Math.max(0, recentStart))
    .filter(
      (message) =>
        !p.quickChecking || message.role === "user" || !message.artifact_id,
    );
  const waitingForResult =
    (!!p.quickChecking && hasResults) ||
    (p.view === "checks" &&
      (!!p.quickChecking ||
        (!!p.pendingLabel && !active) ||
        (!!runID && !result)));
  const showResult = p.view === "checks" && !!result && !waitingForResult;
  const activityLabel =
    p.pendingLabel ||
    (active
      ? conversationActivity(active, p.testJourney)
      : p.quickChecking
        ? "Preparing the check…"
        : waitingForResult
          ? "Waiting for your check…"
          : undefined);
  const contentStamp = [
    messages.at(-1)?.id,
    pendingMessage?.id,
    p.artifact?.id,
    result?.id,
    result?.state,
    ...operations.filter(operation => operation.error || operation.retry_of_operation_id).map(operation => `${operation.id}:${operation.state}:${!!operation.completion_receipt}`),
  ].join(":");
  const [readPosition, setReadPosition] = useState({
    session: p.session?.id,
    stamp: contentStamp,
  });
  const hasNewResponse =
    !isNearBottom &&
    readPosition.session === p.session?.id &&
    readPosition.stamp !== contentStamp;
  const markRead = () =>
    setReadPosition((previous) =>
      previous.session === p.session?.id && previous.stamp === contentStamp
        ? previous
        : { session: p.session?.id, stamp: contentStamp },
    );
  const previousContent = useRef({
    session: p.session?.id,
    stamp: contentStamp,
  });
  useEffect(() => {
    const previous = previousContent.current;
    previousContent.current = { session: p.session?.id, stamp: contentStamp };
    if (previous.session !== p.session?.id || previous.stamp === contentStamp)
      return;
    if (nearBottom.current) {
      end.current?.scrollIntoView?.({
        behavior: reduced ? "auto" : "smooth",
        block: "nearest",
      });
    }
  }, [contentStamp, p.session?.id, reduced]);
  const switchView = (view: View) => {
    setIntake(null);
    markRead();
    p.onNavigate(view);
    scrollRegion.current?.scrollTo?.({ top: 0, behavior: "auto" });
  };
  const startRun = (baseline?: Operation, id?: string) => {
    setRunSelection({ id: undefined, request: p.requestedRunID });
    setIntake(null);
    p.onRun(baseline, id);
  };
  const changeSource = (mode: "chats" | "instructions") => {
    setCompare(undefined);
    setFreshAnswer(false);
    setFromGenerated(false);
    setIntake(mode);
    p.onNavigate("build");
  };
  const checkNewAnswer = () => {
    if (!result) return;
    const provided = result.source?.kind === "provided_conversations";
    setCompare(provided ? result : undefined);
    setFreshAnswer(true);
    setFromGenerated(!provided);
    setIntakeStartedWith(evidence?.id);
    setIntake("chats");
    p.onNavigate("build");
  };
  const send = () => {
    if (p.busy || p.dirty || !p.content.trim()) return;
    if (p.view !== "build") p.onNavigate("build");
    p.onSend(result && terminal(result.state) ? { viewed_run_id: result.id } : undefined);
  };
  const showComposer = p.view !== "try" && !intake;
  const compactComposer = showResult || (!!p.testJourney && !!p.artifact);
  useComposerAutosize(composer, p.content, compactComposer, showComposer, `${p.view}:${welcome}`);
  const primary = primarySurface({ busy: p.busy, dirty: p.dirty, typing: !!p.content.trim(),
    choice: !!p.interactionActions && hasConversationChoice(p.session), results: showResult,
    tests: !!p.artifact && (!proposalMessage || proposalMessage.id === messages.at(-1)?.id),
    recovery: !!operations.at(-1)?.retryable && !operations.at(-1)?.completion_receipt });
  const suiteContents = p.artifact?.kind === "test_suite" ? (
    <div className="space-y-5" data-proposal-id={p.artifact.id}>
      {promptChanged && <PromptChange key={`instructions:${p.artifact.id}`} before={parent!.agent_prompt} after={p.artifact.agent_prompt} />}
      <TestSuitePanel primary={primary === "tests"} key={`tests:${p.artifact.id}`} artifact={p.artifact} busy={p.busy} blocked={p.dirty}
        comparison={!!changeBaseline} onRun={() => startRun(changeBaseline)} onEdit={p.onEdit}
        onDirty={p.onDirty} onSave={() => p.onSave()} onSettings={p.onSettings} modelLabel={p.modelLabel}
        checkingChanges={p.checkingTestChanges}
        rules={p.session?.document.policies?.find(policy => policy.id === p.artifact!.policy_id && policy.source_version === "spec-sources-v1")?.rules}
        pendingPolicy={p.session?.document.pending_policy_changes?.some(change => change.status === "pending" && (!change.artifact_id || change.artifact_id === p.artifact!.id))} />
      {promptChanged && <VibeButton variant="quiet" disabled={p.busy || p.dirty}
        onClick={() => void p.onEdit({artifact_id: p.artifact!.id, dismissed: true})}>Dismiss suggestion</VibeButton>}
    </div>
  ) : null;
  const suiteCard = suiteContents && (
    p.artifact?.dismissed ? <p className="text-sm vibe-muted py-3">Suggestion dismissed. <button type="button"
      className="underline underline-offset-4" disabled={p.busy || p.dirty}
      onClick={() => void p.onEdit({artifact_id: p.artifact!.id, dismissed: false})}>Show suggestion</button></p>
    : <details className={proposalMessage?.id === messages.at(-1)?.id ? undefined : "vibe-pending-proposal"} key={p.artifact?.id}
        open={proposalMessage?.id === messages.at(-1)?.id || p.dirty || p.checkingTestChanges || undefined}>
        <summary hidden={proposalMessage?.id === messages.at(-1)?.id}>{promptChanged ? "Review the suggested fix" : "Review your tests"}</summary>{suiteContents}
      </details>
  );

  return (
    <>
      <header className="vibe-header">
        <Link
          href="/vibe-evals"
          className="flex w-fit items-center gap-2.5 text-sm font-semibold"
        >
          <ClashMark className="size-5 vibe-muted" />
          Vibe Evals
        </Link>
        <div className="vibe-header-nav justify-self-center">
          <AnimatePresence initial={false}>
            {hasResults && (
              <motion.div
                layout="position"
                key="navigation"
                initial={reduced ? false : { opacity: 0, y: -3 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{
                  duration: reduced ? 0 : 0.15,
                  layout: { duration: reduced ? 0 : 0.22 },
                }}
              >
                <Tabs.Root
                  value={currentPane}
                  onValueChange={(v) => switchView(v as View)}
                >
                  <TooltipProvider delay={350}>
                    <Tabs.List
                      aria-label="Workspace views"
                      className="vibe-nav"
                    >
                      {(
                        [
                          [
                            "build",
                            "Conversation",
                            "Discuss what matters, ask about findings, or change the rules.",
                          ],
                          [
                            "checks",
                            "Results",
                            "See the saved findings and the messages behind each one.",
                          ],
                        ] as const
                      ).map(([value, label, help]) => (
                        <Tooltip key={value}>
                          <Tabs.Tab
                            id={`vibe-tab-${value}`}
                            aria-controls="vibe-workspace-panel"
                            value={value}
                            className="vibe-button vibe-button-quiet relative"
                            render={<TooltipTrigger />}
                          >
                            {currentPane === value && (
                              <motion.span
                                layoutId="vibe-current-view"
                                className="absolute inset-0 -z-10 rounded-[10px] bg-[#22211d]"
                                transition={{ duration: reduced ? 0 : 0.22 }}
                              />
                            )}
                            {label}
                          </Tabs.Tab>
                          <TooltipContent>{help}</TooltipContent>
                        </Tooltip>
                      ))}
                    </Tabs.List>
                  </TooltipProvider>
                </Tabs.Root>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
        <div className="vibe-header-actions flex items-center justify-self-end gap-1">
          <TooltipProvider delay={350}>
            {(
              [
                [
                  "New check",
                  Plus,
                  () => {
                    window.location.href = "/vibe-evals";
                  },
                ],
                ["History", History, () => setHistoryOpen(true)],
                ["Settings", Settings, p.onSettings],
              ] as const
            )
              .filter(
                ([label]) =>
                  label !== "History" || hasResults || p.savedChecks.length > 0,
              )
              .map(([label, Icon, action]) => (
                <Tooltip key={label}>
                  <TooltipTrigger
                    render={
                      <VibeButton
                        variant="quiet"
                        className="!px-2.5"
                        aria-label={label}
                        onClick={action}
                      />
                    }
                  >
                    <Icon />
                  </TooltipTrigger>
                  <TooltipContent>{label}</TooltipContent>
                </Tooltip>
              ))}
          </TooltipProvider>
        </div>
      </header>
      <div
        ref={scrollRegion}
        className="vibe-scroll-region min-h-0 flex-1 overflow-y-auto"
        id="vibe-scroll-region"
        onScroll={(event) => {
          const region = event.currentTarget;
          const wasNearBottom = nearBottom.current;
          nearBottom.current =
            region.scrollHeight - region.scrollTop - region.clientHeight < 120;
          setIsNearBottom(nearBottom.current);
          if (nearBottom.current || wasNearBottom) markRead();
        }}
      >
        <div
          id="vibe-workspace-panel"
          role={hasResults ? "tabpanel" : undefined}
          aria-labelledby={hasResults ? `vibe-tab-${currentPane}` : undefined}
        >
          <div
            className={`vibe-column ${welcome ? "pb-12 pt-[clamp(3rem,15vh,9rem)]" : "py-8 sm:py-12"}`}
          >
            {welcome && !intake && (
              <div className="vibe-welcome mb-7">
                <h1 className="text-[30px] font-semibold leading-[1.2] tracking-tight sm:text-[36px]">
                  {p.testJourney
                    ? "What should your agent do?"
                    : "Check the AI in your app."}
                </h1>
                <p className="mt-4 max-w-[520px] vibe-muted">
                  {p.testJourney
                    ? "Describe its job and rules. We’ll check what works and what needs fixing."
                    : "Tell us what it does, or paste an answer you want checked."}
                </p>
                {p.testJourney && (
                  <p className="mt-3 !text-sm vibe-muted">
                    Already have an agent or challenge pack? Describe it here or{" "}
                    <button
                      type="button"
                      disabled={p.busy || p.dirty}
                      onClick={p.onImport}
                      className="cursor-pointer rounded-sm underline underline-offset-4 hover:text-[var(--vibe-fg)] disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      import your pack
                    </button>
                    .
                  </p>
                )}
              </div>
            )}
            {p.view === "try" ? (
              p.preview
            ) : intake === "chats" ? (
              <section className="space-y-5">
                {fromGenerated && (
                  <p className="text-sm vibe-muted">
                    The earlier answers were generated here. This will check
                    your app’s actual answer as a new check.
                  </p>
                )}
                <EvidenceIntake
                  key={compare?.id || "new"}
                  evidence={
                    freshAnswer && evidence?.id === intakeStartedWith
                      ? undefined
                      : evidence
                  }
                  busy={p.busy}
                  comparing={!!compare}
                  autoPrepare={freshAnswer}
                  onAttach={p.onAttach}
                  onCancel={() => {
                    setIntake(null);
                    if (freshAnswer) p.onNavigate("checks");
                  }}
                  onPrepare={(evidenceID) => {
                    if (compare) {
                      startRun(compare, evidenceID || evidence?.id);
                      return;
                    }
                    setIntake(null);
                    p.onMessage(
                      "Prepare expectations for the complete chats I supplied, using the job and rules we discussed.",
                    );
                  }}
                />
              </section>
            ) : intake === "instructions" ? (
              <section className="space-y-5">
                <div>
                  <h2 className="text-xl font-semibold">
                    Try your app’s instructions
                  </h2>
                  <p className="mt-2 text-sm vibe-muted">
                    Paste the instructions your app gives its AI. We’ll use them
                    to generate example answers here.
                  </p>
                </div>
                <form
                  className="vibe-composer"
                  onSubmit={(e) => {
                    e.preventDefault();
                    if (!instructionText.trim() || p.busy) return;
                    setIntake(null);
                    p.onMessage(
                      "Prepare three useful examples to check these instructions against the rules we discussed.",
                      undefined,
                      instructionText,
                    );
                  }}
                >
                  <label className="sr-only" htmlFor="vibe-instructions-source">
                    Agent instructions to test
                  </label>
                  <textarea
                    id="vibe-instructions-source"
                    value={instructionText}
                    onChange={(e) => setInstructionText(e.target.value)}
                    placeholder="You help customers understand our return policy…"
                    style={{ minHeight: 180 }}
                  />
                  <div className="vibe-composer-actions">
                    <VibeButton
                      variant="quiet"
                      type="button"
                      onClick={() => setInstructionText(exampleBrief)}
                    >
                      Use sample instructions
                    </VibeButton>
                    <VibeButton
                      variant="primary"
                      type="submit"
                      disabled={p.busy || !instructionText.trim()}
                    >
                      Prepare examples
                      <ArrowRight />
                    </VibeButton>
                  </div>
                </form>
                <VibeButton variant="quiet" onClick={() => setIntake(null)}>
                  Back to conversation
                </VibeButton>
              </section>
            ) : showResult && result ? (
              <motion.div
                key={result.id}
                initial={reduced ? false : { opacity: 0, y: 6 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: reduced ? 0 : 0.2 }}
              >
                {runs.length > 1 && (
                  <div className="mb-5 flex justify-end">
                    <label className="text-xs vibe-muted">
                      Result{" "}
                      <select
                        aria-label="Result history"
                        value={result.id}
                        onChange={(e) =>
                          setRunSelection({
                            id: e.target.value,
                            request: p.requestedRunID,
                          })
                        }
                        className="ml-2 rounded-lg border border-[var(--vibe-border)] bg-transparent p-2"
                      >
                        {runs.map((r, i) => (
                          <option key={r.id} value={r.id}>
                            {i === runs.length - 1 ? "Latest · " : ""}Check{" "}
                            {i + 1}
                            {r.kind === "retest" ? " · comparison" : ""}
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>
                )}
                <VibeScorecard
                  testJourney={p.testJourney}
                  primary={primary === "results"}
                  operation={result}
                  baseline={runs.find((r) => r.id === result.baseline_id)}
                  eventCursor={p.session?.event_cursor}
                  loadEvidence={(key) => p.loadEvidence(result.id, key)}
                  busy={p.busy || p.dirty}
                  onImprove={() => {
                    p.onNavigate("build");
                    p.onMessage(
                      result.source?.kind === "provided_conversations"
                        ? "Suggest a concrete instruction change to address the observed failures. Keep the expectations fixed. Explain what to copy into my agent; do not claim to apply it."
                        : "Suggest a focused improvement to these agent instructions using the failed examples. Preserve the checks and expected behavior.",
                      result,
                    );
                  }}
                  onRetest={() => {
                    if (result.source?.kind === "provided_conversations") {
                      checkNewAnswer();
                    } else startRun(result);
                  }}
                  onNewReplies={p.testJourney ? undefined : checkNewAnswer}
                  onDispute={(rule, evidence) => {
                    p.onDispute(rule, evidence, result);
                  }}
                  onSave={() => p.onSave(result)}
                  onRecheck={
                    result.source?.kind === "provided_conversations"
                      ? () => startRun(result, result.source?.evidence_set_id)
                      : undefined
                  }
                />
              </motion.div>
            ) : waitingForResult ? null : (
              <div className={welcome ? "" : "space-y-7"}>
                {hasResults && !activityLabel && (
                  <VibeButton
                    variant="quiet"
                    onClick={() => switchView("checks")}
                  >
                    View latest results
                    <ArrowRight />
                  </VibeButton>
                )}
                {recentStart > 0 && (
                  <details className="text-sm vibe-muted">
                    <summary className="cursor-pointer">
                      Earlier messages
                    </summary>
                    <div className="mt-5 space-y-6">
                      {messages.slice(0, recentStart).map((m) => (
                        <div key={m.id} data-message-id={m.id}>
                          <Message message={m} animate={false} />
                          <ConversationGuidance cards={m.cards} scope={p.session?.document.conversation_state?.brief.scope_id} message={m.id} />
                          {m.role === "user" && m.operation_id && (
                            <OperationFeedback original={operationByID.get(m.operation_id)} operation={latestOperations.get(m.operation_id)}
                              primary={primary === "recovery" && latestOperations.get(m.operation_id)?.id === operations.at(-1)?.id} busy={p.busy || p.dirty} pendingID={p.retryPendingOperationID} uncertainID={p.retryUncertainOperationID} onRetry={p.onRetry} />
                          )}
                        </div>
                      ))}
                    </div>
                  </details>
                )}
                <div
                  key={p.session?.id || "new-conversation"}
                  role="log"
                  aria-label="Conversation with Vibe Evals"
                  aria-relevant="additions"
                  className="space-y-6"
                >
                  <AnimatePresence initial={false}>
                    {visibleMessages.map((m) => (
                      <div key={m.id} data-message-id={m.id}>
                        <Message message={m} />
                        <ConversationGuidance cards={m.cards} scope={p.session?.document.conversation_state?.brief.scope_id} message={m.id} />
                        {m.role === "user" && m.operation_id && (
                          <OperationFeedback original={operationByID.get(m.operation_id)} operation={latestOperations.get(m.operation_id)}
                            primary={primary === "recovery" && latestOperations.get(m.operation_id)?.id === operations.at(-1)?.id} busy={p.busy || p.dirty} pendingID={p.retryPendingOperationID} uncertainID={p.retryUncertainOperationID} onRetry={p.onRetry} />
                        )}
                        {m.id === proposalMessage?.id && suiteCard && <div className="mt-5">{suiteCard}</div>}
                      </div>
                    ))}
                    {pendingMessage && (
                      <Message
                        key={pendingMessage.id}
                        message={pendingMessage}
                        pending
                      />
                    )}
                  </AnimatePresence>
                </div>
                {!p.quickChecking &&
                  !activityLabel &&
                  promptChanged &&
                  p.artifact && p.artifact.kind !== "test_suite" && (
                    <PromptChange
                      before={parent!.agent_prompt}
                      after={p.artifact.agent_prompt}
                    />
                  )}
                {!p.quickChecking &&
                  (!activityLabel || (p.artifact?.kind === "test_suite" && p.dirty)) &&
                  (p.artifact?.kind === "test_suite" ? (
                    !visibleMessages.some(message => message.id === proposalMessage?.id) ? suiteCard : null
                  ) : p.artifact?.kind === "test_plan" ? (
                    <div className="vibe-panel p-5">
                      <h2 className="font-semibold">{p.artifact.title}</h2>
                      <p className="mt-2 text-sm vibe-muted">
                        Add an answer from your app to see what it gets right
                        and what needs attention.
                      </p>
                      <div className="mt-4 flex flex-wrap gap-2">
                        <VibeButton onClick={() => changeSource("chats")}>
                          Add an answer
                        </VibeButton>
                        <VibeButton
                          onClick={() => changeSource("instructions")}
                        >
                          Try instructions
                        </VibeButton>
                      </div>
                      <details className="mt-4">
                        <summary>View the plan</summary>
                        <pre className="vibe-transcript p-4 text-sm">
                          {JSON.stringify(p.artifact.test_plan, null, 2)}
                        </pre>
                      </details>
                    </div>
                  ) : (
                    p.artifact && (
                      <details
                        open={!artifactHasRun || undefined}
                        className="text-sm"
                      >
                        <summary
                          hidden={!artifactHasRun}
                          className="mb-4 cursor-pointer py-2 vibe-muted"
                        >
                          What was checked
                        </summary>
                        <ConversationProposal
                          key={p.artifact.id}
                          artifact={p.artifact}
                          evidence={artifactEvidence}
                          busy={p.busy}
                          blocked={p.dirty}
                          comparison={!!changeBaseline}
                          onRun={() =>
                            startRun(changeBaseline, artifactEvidence?.id)
                          }
                          onEdit={p.onEdit}
                          onDirty={p.onDirty}
                          onInstructions={p.onInstructions}
                          onPreview={() => p.onNavigate("try")}
                        />
                      </details>
                    )
                  ))}
              </div>
            )}
            {p.view === "build" && !intake && p.session && p.interactionActions && p.onChoice && p.onReloadChoices && (
              <div className="mt-4"><ConversationActions key={p.session.id} session={p.session} busy={p.busy || p.dirty} primary={primary === "choices"} onAction={p.onChoice} onReload={p.onReloadChoices} /></div>
            )}
            {p.instructions &&
              !welcome &&
              !p.quickChecking &&
              !activityLabel && (
                <div hidden={p.view !== "build" || !!intake} className="mt-5">
                  {p.instructions}
                </div>
              )}
            <ActivityStatus label={activityLabel} working={active?.state === "RUNNING" && !p.pendingLabel} />
            <AnimatePresence initial={false}>
              {activityLabel && active && (
                <motion.div
                  key="activity"
                  className="vibe-activity"
                  initial={reduced ? false : { opacity: 0, y: 4 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: reduced ? 0 : 0.18 }}
                >
                  {active && (
                    <div className="flex gap-2">
                      {active.state === "AWAITING_APPROVAL" && (
                        <VibeButton
                          disabled={!!p.pendingLabel}
                          onClick={() => p.onAction(active.id, "approve")}
                        >
                          Approve run
                        </VibeButton>
                      )}
                      <VibeButton
                        variant="quiet"
                        disabled={!!p.pendingLabel}
                        onClick={() => p.onAction(active.id, "stop")}
                      >
                        <Square />
                        Stop
                      </VibeButton>
                    </div>
                  )}
                </motion.div>
              )}
            </AnimatePresence>
            <div className={`vibe-notice ${welcome ? "" : "mt-7"}`}>
              {p.notice}
            </div>
            {showComposer && (
              <div className={welcome ? "" : "mt-7"}>
                {!welcome && !showResult && (
                  <label
                    htmlFor="vibe-message"
                    className="mb-2 block text-xs vibe-muted"
                  >
                    Your next message
                  </label>
                )}
                <form
                  className={`vibe-composer ${compactComposer ? "vibe-composer-compact" : ""}`}
                  onSubmit={(e) => {
                    e.preventDefault();
                    send();
                  }}
                >
                  <label htmlFor="vibe-message" className="sr-only">
                    Message Vibe Evals
                  </label>
                  <textarea
                    ref={composer}
                    id="vibe-message"
                    aria-label="Message Vibe Evals"
                    value={p.content}
                    onChange={(e) => p.onContent(e.target.value)}
                    maxLength={p.session?.anonymous === false ? 65536 : 16384}
                    placeholder={
                      welcome
                        ? p.testJourney
                          ? "My agent helps customers with returns…"
                          : "I made an AI trip planner…"
                        : showResult
                          ? "Ask about these results…"
                          : p.testJourney
                            ? "Add a rule, ask a question, or change a test…"
                            : "Paste the question and your app’s answer, or keep talking…"
                    }
                    rows={compactComposer ? 1 : 3}
                    onKeyDown={(e) => sendOnEnter(e, send)}
                  />
                  <div className="vibe-composer-actions">
                    <span className="vibe-composer-hint text-xs vibe-muted">
                      {welcome || showResult
                        ? ""
                        : p.busy
                          ? "You can keep typing while this runs."
                          : "Shift + Enter for a new line"}
                    </span>
                    <VibeButton
                      variant={
                        primary !== "composer"
                          ? "quiet"
                          : "primary"
                      }
                      type="submit"
                      aria-label="Send message"
                      disabled={p.busy || p.dirty || !p.content.trim()}
                    >
                      {!showResult && "Send"}
                      <ArrowUp />
                    </VibeButton>
                  </div>
                </form>
                {welcome && (
                  <VibeButton
                    variant="quiet"
                    className="mt-3 !px-0"
                    aria-expanded={welcomeExample}
                    onClick={() => setWelcomeExample(!welcomeExample)}
                  >
                    {welcomeExample ? "Hide example" : "See an example"}
                  </VibeButton>
                )}
              </div>
            )}
            {welcome && welcomeExample && <div className="mt-3 space-y-3">
              <p className="text-sm vibe-muted">A test pairs an example task with what should happen.</p>
              <ExampleComparison input="Can I return an unopened item bought 10 days ago?" expected="With a 30-day policy, this item is eligible. No refund is processed." />
              <VibeButton variant="quiet" onClick={() => { p.onContent(p.testJourney ? exampleAgentDescription : exampleAnswer); composer.current?.focus({ preventScroll: true }); }}>Use this description</VibeButton>
            </div>}
            {!p.testJourney &&
              !welcome &&
              !intake &&
              p.view !== "try" &&
              !activityLabel && (
                <details className="vibe-context-options mt-5 text-sm vibe-muted">
                  <summary className="w-fit cursor-pointer py-2">
                    More options
                  </summary>
                  <div className="mt-2 flex flex-wrap gap-2">
                    <VibeButton
                      variant="quiet"
                      onClick={() => changeSource("chats")}
                    >
                      {evidence
                        ? "View or edit source messages"
                        : "Add a conversation or file"}
                    </VibeButton>
                    <VibeButton
                      variant="quiet"
                      onClick={() => changeSource("instructions")}
                    >
                      Try instructions instead
                    </VibeButton>
                    {showResult && p.artifact && (
                      <VibeButton
                        variant="quiet"
                        onClick={() => switchView("build")}
                      >
                        Review or change the rules
                      </VibeButton>
                    )}
                  </div>
                </details>
              )}
            <div ref={end} />
          </div>
        </div>
      </div>
      {hasNewResponse && (
        <div className="vibe-new-response">
          <VibeButton
            onClick={() => {
              nearBottom.current = true;
              setIsNearBottom(true);
              markRead();
              end.current?.scrollIntoView?.({
                behavior: reduced ? "auto" : "smooth",
                block: "nearest",
              });
              composer.current?.focus({ preventScroll: true });
            }}
          >
            New response
            <ArrowDown />
          </VibeButton>
        </div>
      )}
      <Dialog open={historyOpen} onOpenChange={setHistoryOpen}>
        <DialogContent className="vibe-workspace max-h-[85vh] overflow-y-auto">
          <DialogTitle>
            {p.testJourney ? "Your saved tests" : "Your saved checks"}
          </DialogTitle>
          <DialogDescription>
            {p.testJourney
              ? "Tests you can return to after your next change."
              : "Private checks you can return to and repeat."}
          </DialogDescription>
          {p.savedChecks.length ? (
            p.savedChecks.map((c) => (
              <Link
                key={c.id}
                className="rounded-lg border p-3 text-sm"
                href={`/vibe-evals?session=${c.session_id}&agent=${c.artifact_id}${c.baseline_operation_id && c.baseline_operation_id !== "00000000-0000-0000-0000-000000000000" ? `&view=checks&run=${c.baseline_operation_id}` : ""}`}
              >
                {c.title}
                <span className="mt-1 block text-xs text-muted-foreground">
                  {c.draft_id
                    ? "Saved tests"
                    : c.source?.kind === "provided_conversations"
                      ? "Provided chats"
                      : "Text test"}
                </span>
              </Link>
            ))
          ) : (
            <p className="text-sm text-muted-foreground">
              {p.testJourney
                ? "Choose “Keep these tests” to find them here next time."
                : "After your first result, choose “Save this check” to keep it here."}
            </p>
          )}
          {runs.length > 0 && (
            <VibeButton
              onClick={() => {
                setHistoryOpen(false);
                switchView("checks");
              }}
            >
              Results in this conversation
              <ArrowRight />
            </VibeButton>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}

function OperationFeedback({ original, operation, busy, pendingID, uncertainID, onRetry, primary }: {
  primary?: boolean;
  original?: Operation;
  operation?: Operation;
  busy: boolean;
  pendingID?: string;
  uncertainID?: string;
  onRetry?: (id: string) => void;
}) {
  if (!operation || !original?.error) return null;
  const acknowledgement = completionAcknowledgement(operation);
  if (operation.completion_receipt)
    return <p role="status" className="mt-3 text-sm vibe-muted">{operation.id !== original.id ? "Completed on retry." : acknowledgement || "This request completed."}</p>;
  const uncertain = uncertainID === operation.id;
  const retrying = pendingID === operation.id || !terminal(operation.state);
	const rateLimited = operation.error?.code === "provider_rate_limit" || original.error.code === "provider_rate_limit";
  // Historical generic failures sometimes claimed that nonexistent tests had
  // been saved. A missing receipt cannot substantiate that claim.
  const message = operation.error?.code === "invalid_response" && operation.error.message.includes("tests are saved")
    ? "I couldn’t complete that response. Your request is still here."
    : operation.error?.message || original.error.message;
  return (
    <div className="mt-3 space-y-2 text-sm">
      <p role="alert" className="text-builder-warn">{message}</p>
      {uncertain && <p className="vibe-muted">The retry acknowledgement wasn’t confirmed. Retry to recover its saved status.</p>}
      {retrying && <p role="status" className="vibe-muted">Retrying this request…</p>}
      {onRetry && (operation.retryable || uncertain) && (
        <VibeButton variant={primary ? "primary" : "secondary"} disabled={retrying || (busy && !uncertain)} onClick={() => onRetry(operation.id)}>{rateLimited ? "Try again" : "Retry"}</VibeButton>
      )}
    </div>
  );
}

function Message({
  message,
  pending = false,
  animate = true,
}: {
  message: Session["document"]["messages"][number];
  pending?: boolean;
  animate?: boolean;
}) {
  const reduced = useReducedMotion();
  return (
    <motion.div
      className={`vibe-message ${message.role === "user" ? "vibe-message-user" : ""}`}
      data-pending={pending || undefined}
      initial={!animate || reduced ? false : { opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: reduced ? 0 : 0.18 }}
    >
      {message.role !== "user" && (
        <p className="mb-2 flex items-center gap-2 text-xs vibe-muted">
          <ClashMark className="size-4" />
          Vibe Evals
        </p>
      )}
      <SafeMarkdown>{message.content}</SafeMarkdown>
    </motion.div>
  );
}
