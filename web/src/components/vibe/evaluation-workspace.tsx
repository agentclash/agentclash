"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import {
  ArrowRight,
  ArrowDown,
  ArrowUp,
  ArrowLeft,
  CircleHelp,
  FileUp,
  FileText,
  History,
  Plus,
  Settings,
  Square,
} from "lucide-react";
import { Tabs } from "@base-ui/react/tabs";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { ClashMark } from "@/components/marketing/clash-mark";
import {
  terminal,
  type Artifact,
  type CaseResult,
  type Operation,
  type SavedCheck,
  type Session,
} from "@/lib/vibe";
import { SavedWorkDialog } from "./saved-work-dialog";
import type { ConversationAction } from "@/lib/vibe-conversation";
import { hasConversationChoice, primarySurface } from "@/lib/vibe-presentation";
import { ConversationActions } from "./conversation-actions";
import { ConversationGuidance, ExampleComparison } from "./conversation-guidance";
import { EvaluationOutcome } from "./evaluation-outcome";
import { CoverageNote } from "./coverage-note";
import { OperationFeedback } from "./operation-feedback";
import { VibeButton } from "./vibe-button";
import { SafeMarkdown } from "./safe-markdown";


import { PromptChange } from "./prompt-change";
import { ActivityStatus, conversationActivity } from "./activity-status";
import { useComposerAutosize } from "./use-composer-autosize";
import { sendOnEnter } from "./composer-keyboard";
import { VibeScorecard } from "./scorecard";
import { TestSuitePanel, exampleAgentDescription } from "./test-suite-panel";
import { EvaluationEntry } from "./evaluation-entry";
import { evaluationIdentity } from "./evaluation-navigation";
import { BuildConversation } from "./build-conversation";
import { buildProgress } from "@/lib/vibe-build-timeline";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent } from "@/lib/vibe-build-analytics";
import "./workspace.css";

type View = "build" | "try" | "checks";
const scrollPositions = new Map<string, number>();

export const exampleAnswer = `Check this answer from my trip-planning app. It should respect the total budget, including every listed cost.

User: Plan a one-day Jaipur trip for two people with a total budget of ₹5,000, including transport, food, and activities.
Assistant: Here’s your plan: transport ₹2,000, food ₹2,000, and activities ₹4,000. Total: ₹8,000. This fits within your ₹5,000 budget.`;

export type EvaluationWorkspaceProps = {
  twoDoor?: boolean;
  onDoor?: (door: "build" | "test") => void;
  contextControl?: ReactNode;
  navigationToggle?: ReactNode;
  newEvaluation?: boolean;
  contextNavigationBlocked?: boolean;
  onCancelNew?: () => void;
  canChangeDoor?: boolean;
  onChangeDoor?: () => void;
  savedWorkOpen?: boolean;
  onSavedWorkOpenChange?: (open: boolean) => void;
  buildStart?: boolean;
  onSample?: () => void;
  onDemo?: (id: string) => void;
  onTougher?: (text: string, count: number, artifactID: string) => void;
  sendBlocked?: boolean;
  sendError?: string;
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
  onNavigate: (view: View, artifactID?: string) => void;
  onRun: (baseline?: Operation, evidenceID?: string) => void;
  onEdit: (fields: Record<string, unknown>) => Promise<boolean>;
  onDirty: (dirty: boolean) => void;
  onSave: (operation?: Operation) => void;
  onImport: () => void;
  onSettings: () => void;
  loadEvidence: (operation: string, key: string) => Promise<CaseResult>;
  onAction: (id: string, action: "stop" | "approve") => void;
  onRetry?: (id: string, assistantModel?: string) => void;
  retryModels?: { id: string; name: string }[];
  retryPendingOperationID?: string;
  retryUncertainOperationID?: string;
  checkingTestChanges?: boolean;
  onRegrade?: (operation: Operation) => void;
  onDispute: (rule: string, result: CaseResult, operation: Operation) => void;
  notice: ReactNode;
  preview: ReactNode;
  savedChecks: SavedCheck[];
};

export function EvaluationWorkspace(p: EvaluationWorkspaceProps) {
  const params = useSearchParams();
  const door = p.session?.document.evaluation?.door;
  const buildJourney = door === "build";
  const previewOpen = buildJourney && p.view === "try";
  const [referencedRun, setReferencedRun] = useState<Operation>();
  const entry = !!p.twoDoor && !door && !p.session?.document.messages.length && !p.session?.document.artifacts.length;

  const awaitingBuildAnswer = p.session?.document.build?.phase === "clarifying";
  const pendingDemo = p.session?.document.build?.phase === "waiting" ? p.session.document.conversation_state?.pending_demo : undefined;
  const activeBuildQuestion = awaitingBuildAnswer && p.session?.document.conversation_state?.pending_question?.status === "active" ? p.session.document.conversation_state.pending_question : undefined;
  useEffect(() => {
    if (buildJourney && p.session && activeBuildQuestion)
      captureBuildEvent(WEB_EVENTS.VIBE_BUILD_CLARIFICATION_VIEWED, { session_id: p.session.id, question_id: activeBuildQuestion.id, clarification_count: p.session.document.build?.clarifications_used || 0 }, `${p.session.id}:${activeBuildQuestion.id}`);
  }, [buildJourney, p.session, activeBuildQuestion]);
  const reduced = useReducedMotion();
  const [intake, setIntake] = useState<"instructions" | null>(null);
  const [instructionText, setInstructionText] = useState("");
  const [welcomeExample, setWelcomeExample] = useState(false);
  const [localHistoryOpen, setLocalHistoryOpen] = useState(false);
  const historyOpen = p.savedWorkOpen ?? localHistoryOpen;
  const setHistoryOpen = p.onSavedWorkOpenChange ?? setLocalHistoryOpen;
  const newEntry = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (p.newEvaluation) newEntry.current?.focus();
  }, [p.newEvaluation]);
  const [isNearBottom, setIsNearBottom] = useState(true);
  const [runSelection, setRunSelection] = useState({
    id: params.get("run") || undefined,
    request: p.requestedRunID,
  });
  const end = useRef<HTMLDivElement>(null);
  const scrollRegion = useRef<HTMLDivElement>(null);
  const composer = useRef<HTMLTextAreaElement>(null);
  const nearBottom = useRef(true);
  const scrollKey = `${p.session?.id || "entry"}:${p.view}`;
  useLayoutEffect(() => {
    const region = scrollRegion.current;
    if (region && buildJourney) {
      region.scrollTop = scrollPositions.get(scrollKey) || 0;
      nearBottom.current = region.scrollHeight - region.scrollTop - region.clientHeight < 120;
      setIsNearBottom(nearBottom.current);
    }
  }, [scrollKey, buildJourney]);
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
  const currentPane = p.view === "try" ? "build" : p.view;
  const hasResults = runs.some((run) => terminal(run.state));
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
      ? buildJourney ? buildProgress(active) : conversationActivity(active, p.testJourney)
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
      if (buildJourney) {
        // Reveal the beginning of the newest turn/card, not the tail of a long
        // response. Subsequent evidence expansion keeps the browser's anchor.
        const region = scrollRegion.current;
        const entries = region?.querySelectorAll<HTMLElement>("[data-build-entry], [data-pending-message]");
        const newest = entries?.[entries.length - 1];
        if (region && newest) {
          const top = newest.getBoundingClientRect().top - region.getBoundingClientRect().top + region.scrollTop;
          region.scrollTo?.({ top: Math.min(top, region.scrollHeight - region.clientHeight), behavior: reduced ? "auto" : "smooth" });
        }
        return;
      }
      end.current?.scrollIntoView?.({
        behavior: reduced ? "auto" : "smooth",
        block: "nearest",
      });
    }
  }, [contentStamp, p.session?.id, reduced, buildJourney]);
  const switchView = (view: View) => {
    if (buildJourney && scrollRegion.current) scrollPositions.set(scrollKey, scrollRegion.current.scrollTop);
    setIntake(null);
    markRead();
    p.onNavigate(view);
    if (!buildJourney) scrollRegion.current?.scrollTo?.({ top: 0, behavior: "auto" });
  };
  const startRun = (baseline?: Operation, id?: string) => {
    setRunSelection({ id: undefined, request: p.requestedRunID });
    setIntake(null);
    p.onRun(baseline, id);
  };
  const changeSource = () => {
    setIntake("instructions");
    p.onNavigate("build");
  };
  const send = () => {
    if (p.busy || p.dirty || p.sendBlocked || !p.content.trim()) return;
    nearBottom.current = true;
    setIsNearBottom(true);
    if (p.view !== "build") p.onNavigate("build");
    const context = buildJourney ? referencedRun : result;
    p.onSend(context && terminal(context.state) ? { viewed_run_id: context.id } : undefined);
    setReferencedRun(undefined);
  };
  const showComposer = (p.view !== "try" || buildJourney) && !intake && !entry;
  const compactComposer = true;
  useComposerAutosize(composer, p.content, compactComposer, showComposer && !p.newEvaluation, `${p.view}:${welcome}`);
  const primary = primarySurface({ busy: p.busy, dirty: p.dirty, typing: !!p.content.trim(),
    choice: !!p.interactionActions && hasConversationChoice(p.session), build: buildJourney,
    results: showResult || (buildJourney && !!latest && latest.source?.artifact_id === p.artifact?.id && terminal(latest.state)),
    tests: !!p.artifact && (!proposalMessage || proposalMessage.id === messages.at(-1)?.id),
    recovery: !!operations.at(-1)?.retryable && !operations.at(-1)?.completion_receipt });
  const buildPrimary = primary === "results";
  function askAboutRun(operation: Operation) {
    setReferencedRun(operation);
    p.onNavigate("build");
    requestAnimationFrame(() => composer.current?.focus({ preventScroll: true }));
  }
  function inspectRun(operation: Operation) {
    if (scrollRegion.current) scrollPositions.set(scrollKey, scrollRegion.current.scrollTop);
    setRunSelection({ id: operation.id, request: p.requestedRunID });
    p.onNavigate("checks");
    const url = new URL(window.location.href);
    url.searchParams.set("run", operation.id);
    window.history.replaceState(null, "", url.pathname + url.search);
  }
  const suiteContents = p.artifact?.kind === "test_suite" ? (
    <div className="space-y-5" data-proposal-id={p.artifact.id}>
      {promptChanged && <PromptChange key={`instructions:${p.artifact.id}`} before={parent!.agent_prompt} after={p.artifact.agent_prompt} defaultOpen={buildJourney} />}
      <TestSuitePanel primary={primary === "tests"} key={`tests:${p.artifact.id}`} artifact={p.artifact} busy={p.busy} blocked={p.dirty}
        buildJourney={buildJourney}
        parent={parent}
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
  const composerDock = showComposer && !p.newEvaluation && (
    <div className="vibe-composer-dock" data-testid="vibe-composer-dock">
      {(hasNewResponse || (buildJourney && !isNearBottom)) && (
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
            {hasNewResponse ? "New response" : "Latest"}
            <ArrowDown />
          </VibeButton>
        </div>
      )}
      <div className="vibe-column vibe-composer-dock-inner">
        {p.contextControl && <div className="vibe-active-context">{p.contextControl}</div>}
        {previewOpen ? p.preview : <>
        <div>
          {referencedRun && <div className="vibe-run-reference"><span>Asking about this saved result · {referencedRun.scorecard?.passed ?? 0} passed</span><button type="button" aria-label="Remove result reference" onClick={() => setReferencedRun(undefined)}>×</button></div>}
          {!buildJourney && !welcome && !showResult && (
            <label htmlFor="vibe-message" className="mb-2 block text-xs vibe-muted">
              Your next message
            </label>
          )}
          <form
            className={`vibe-composer vibe-composer-compact vibe-chat-composer ${p.buildStart ? "vibe-composer-build-start" : ""}`}
            onSubmit={(e) => {
              e.preventDefault();
              send();
            }}
          >
            <label htmlFor="vibe-message" className="sr-only">Message Vibe Evals</label>
            <textarea
              ref={composer}
              id="vibe-message"
              aria-label="Message Vibe Evals"
              value={p.content}
              onChange={(e) => p.onContent(e.target.value)}
              maxLength={p.session?.anonymous === false ? 65536 : 16384}
              placeholder={
                welcome
                  ? door === "test" ? "Or describe your agent and what you want to check…" : door === "build" ? "We spend hours answering customer questions about returns…" : p.testJourney
                    ? "My agent helps customers with returns…"
                    : "I made an AI trip planner…"
                  : referencedRun
                    ? "Ask about this saved result…"
                    : showResult && !buildJourney
                    ? "Ask about these results…"
                    : buildJourney ? "Message Vibe Evals…" : p.testJourney
                      ? "Add a rule, ask a question, or change a test…"
                      : "Paste the question and your app’s answer, or keep talking…"
              }
              rows={1}
              onKeyDown={(e) => sendOnEnter(e, send)}
            />
            <div className="vibe-composer-actions">
              <span className="vibe-composer-hint vibe-muted">
                {welcome || showResult
                  ? ""
                  : p.busy
                    ? "You can keep typing while this runs."
                    : "Shift + Enter for a new line"}
              </span>
              <VibeButton
                variant={p.content.trim() && !p.busy && !p.dirty ? "primary" : "quiet"}
                type="submit"
                aria-label="Send message"
                disabled={p.busy || p.dirty || p.sendBlocked || !p.content.trim()}
              >
                {p.buildStart ? "Build and try 3 examples" : <span className="sr-only">Send</span>}
                <ArrowUp />
              </VibeButton>
            </div>
          </form>
          {p.sendError && <p className="mt-2 text-xs vibe-muted" role="alert">{p.sendError}</p>}
          {welcome && !entry && (
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
        </>}
      </div>
    </div>
  );

  return (
    <>
      <header className="vibe-header">
        {p.twoDoor ? <div className="vibe-header-identity">
          {p.navigationToggle}
          <span className="vibe-header-title">{p.newEvaluation || entry ? "New agent" : p.session ? evaluationIdentity(p.session).title : "Vibe Evals"}</span>
        </div> : (
        <Link
          href="/vibe-evals"
          className="flex w-fit items-center gap-2.5 text-sm font-semibold"
        >
          <ClashMark className="size-5 vibe-muted" />
          Vibe Evals
        </Link>
        )}
        <div className="vibe-header-nav justify-self-center">
          <AnimatePresence initial={false}>
            {hasResults && !p.newEvaluation && (
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
        {!p.twoDoor && <div className="vibe-header-actions flex items-center justify-self-end gap-1">
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
        </div>}
      </header>
      {p.newEvaluation && <div className="vibe-scroll-region min-h-0 flex-1 overflow-y-auto" ref={newEntry} tabIndex={-1}
        aria-label="New agent choices" onKeyDown={event => { if (event.key === "Escape" && !p.contextNavigationBlocked) { p.onCancelNew?.(); requestAnimationFrame(() => composer.current?.focus({ preventScroll: true })); } }}>
        <div className="vibe-column vibe-entry-page">
          <VibeButton variant="quiet" className="vibe-back-button" disabled={p.contextNavigationBlocked} onClick={() => {
            p.onCancelNew?.(); requestAnimationFrame(() => composer.current?.focus({ preventScroll: true }));
          }}><ArrowLeft />{p.session?.document.evaluation ? "Back to your evaluation" : "Back"}</VibeButton>
          <EvaluationEntry busy={!!p.contextNavigationBlocked} onDoor={p.onDoor} />
          {p.pendingLabel && <ActivityStatus label={p.pendingLabel} working />}
          {p.notice}
        </div>
      </div>}
      <div
        ref={scrollRegion}
        hidden={p.newEvaluation}
        className="vibe-scroll-region min-h-0 flex-1 overflow-y-auto"
        id="vibe-scroll-region"
        onScroll={(event) => {
          const region = event.currentTarget;
          if (buildJourney) {
            scrollPositions.set(scrollKey, region.scrollTop);
            if (scrollPositions.size > 100) scrollPositions.delete(scrollPositions.keys().next().value!);
          }
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
            className={`vibe-column ${welcome ? "vibe-entry-page" : "py-8 sm:py-12"}`}
          >
            {welcome && !intake && (entry ? <EvaluationEntry busy={p.busy} onDoor={p.onDoor} /> : <div className="vibe-welcome mb-7">
              <h1 className="vibe-entry-title">{door === "build" ? "What would you like help with?" : door === "test" ? "Improve an agent you already have." : p.testJourney ? "What should your agent do?" : "Check the AI in your app."}</h1>
              <p className="mt-4 max-w-[560px] vibe-muted">{door === "build" ? "Describe a repetitive task. We’ll make a prototype and try three situations to see what works." : door === "test" ? "Bring your agent’s instructions or a saved test pack." : p.testJourney ? "Describe its job and rules. We’ll check what works and what needs fixing." : "Tell us what it does, or paste an answer you want checked."}</p>
              {door === "test" && <><div className="vibe-source-options">
                <div className="vibe-source-option">
                  <VibeButton disabled={p.busy} onClick={p.onImport}><FileUp />Import a test pack</VibeButton>
                  <TooltipProvider delay={250}>
                    <Tooltip>
                      <TooltipTrigger aria-label="What is a test pack?" className="vibe-source-tooltip"><CircleHelp aria-hidden="true" /></TooltipTrigger>
                      <TooltipContent side="top" className="max-w-[280px] leading-relaxed">A test pack is a saved set of situations to try and what a good response should do. Import one only if you already have it.</TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </div>
                <VibeButton disabled={p.busy} onClick={changeSource}><FileText />Paste agent instructions</VibeButton>
              </div><p className="vibe-source-note">Live connections aren’t available here yet. Instructions recreate behavior here; a pack supplies tests, not an agent.</p>
              <details className="vibe-source-help"><summary>What’s a test pack?</summary><p>A saved set of situations to try and what a good response should do. It is also called a challenge pack. Import one only if you already have it.</p></details>
              {p.canChangeDoor && <button type="button" className="vibe-door-switch" onClick={p.onChangeDoor}>Actually, I want to build an agent.</button>}</>}
              {door === "build" && p.canChangeDoor && <button type="button" className="vibe-door-switch" onClick={p.onChangeDoor}>Already have an agent? Improve it instead.</button>}
              {!door && p.testJourney && <p className="mt-3 text-sm vibe-muted">Already have an agent or challenge pack? Describe it here or <button type="button" className="underline underline-offset-4" disabled={p.busy} onClick={p.onImport}>import your pack</button>.</p>}
            </div>)}
            {door && !buildJourney && p.artifact?.agent_prompt && <section aria-label="Prototype scope" className="mb-5 text-sm">
              <div className="flex flex-wrap items-center justify-between gap-2"><p className="font-medium">Interactive prototype{p.artifact.sample ? " · Sample demonstration" : ""}</p><VibeButton variant="quiet" onClick={() => p.onNavigate("try")}>Talk to this prototype</VibeButton></div>
              <p className="vibe-muted">{p.artifact.scope_note || "Runs here using what you supplied; your business systems aren’t connected."}</p>
              {p.artifact.sample && <p className="mt-1 vibe-muted">These results check the sample, not your business policy.</p>}
            </section>}
            {buildJourney && p.session && !welcome && <div hidden={(p.view !== "build" && p.view !== "try") || !!intake}>
              <BuildConversation session={p.session} artifact={p.artifact} busy={p.busy || p.dirty} primary={buildPrimary}
                previewOpen={previewOpen} onPreviewOpen={(open, id) => p.onNavigate(open ? "try" : "build", id || p.artifact?.id)} onReviewArtifact={id => p.onNavigate("build", id)} preview={p.view === "try" ? null : p.preview}
                proposal={active ? null : suiteCard}
                pending={pendingMessage && <div data-pending-message><Message message={pendingMessage} pending /></div>}
                renderMessage={m => <><Message message={m} animate={false} /><ConversationGuidance cards={m.cards} scope={p.session?.document.conversation_state?.brief.scope_id} message={m.id} />

                  {m.role === "user" && m.operation_id && <OperationFeedback serverTime={p.session?.server_time}
                    original={operationByID.get(m.operation_id)} operation={latestOperations.get(m.operation_id)}
                    primary={primary === "recovery" && latestOperations.get(m.operation_id)?.id === operations.at(-1)?.id}
                    busy={p.busy || p.dirty} pendingID={p.retryPendingOperationID} uncertainID={p.retryUncertainOperationID}
                    onRetry={p.onRetry} retryModels={p.retryModels} />}
                </>}
                onDetails={inspectRun}
                onGuide={() => { p.onNavigate("build"); requestAnimationFrame(() => composer.current?.focus({ preventScroll: true })); }}
                onImprove={operation => {
                  nearBottom.current = true;
                  setIsNearBottom(true);
                  p.onMessage("Suggest a focused improvement to these agent instructions using the failed examples. Preserve the checks and expected behavior.", operation);
                }}
                onSave={p.onSave} onTougher={p.onTougher} loadEvidence={p.loadEvidence} />
            </div>}
            {p.view === "try" ? (
              buildJourney ? null : p.preview
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
                      onClick={() => setInstructionText(exampleAgentDescription)}
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
                            {r.source?.comparison === "regraded" ? " · grades rechecked" : r.kind === "retest" ? " · comparison" : ""}
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>
                )}
                <VibeScorecard
                  findingFirst={!!door}
                  testJourney={p.testJourney}
                  primary={primary === "results"}
                  operation={result}
                  baseline={runs.find((r) => r.id === result.baseline_id)}
                  eventCursor={p.session?.event_cursor}
                  loadEvidence={(key) => p.loadEvidence(result.id, key)}
                  busy={p.busy || p.dirty || (buildJourney && result.source?.artifact_id !== p.artifact?.id)}
                  onImprove={() => {
                    p.onNavigate("build");
                    p.onMessage(
                      result.source?.kind === "provided_conversations"
                        ? "Suggest a concrete instruction change to address the observed failures. Keep the expectations fixed. Explain what to copy into my agent; do not claim to apply it."
                        : "Suggest a focused improvement to these agent instructions using the failed examples. Preserve the checks and expected behavior.",
                      result,
                    );
                  }}
                  onRetest={() => startRun(result)}
                  onDispute={(rule, evidence) => {
                    p.onDispute(rule, evidence, result);
                  }}
                  onNewCheck={() => startRun(undefined, result.source?.evidence_set_id)}
                  onRegrade={p.onRegrade ? () => p.onRegrade?.(result) : undefined}
                  onSave={() => p.onSave(result)}
                  onRecheck={
                    result.source?.kind === "provided_conversations"
                      ? () => startRun(result, result.source?.evidence_set_id)
                      : undefined
                  }
                />
                {buildJourney && <VibeButton variant="quiet" onClick={() => askAboutRun(result)}>Ask about this result</VibeButton>}
                {door && p.session && p.artifact && terminal(result.state) && <EvaluationOutcome key={`outcome:${result.id}`} session={p.session} artifact={p.session.document.artifacts.find(a => a.id === result.source?.artifact_id) || p.artifact} operation={result} busy={p.busy || p.dirty} loadEvidence={p.loadEvidence} onTougher={buildJourney && result.source?.artifact_id !== p.artifact.id ? undefined : p.onTougher} />}
                {result.results.length > 0 && terminal(result.state) && (
                  <CoverageNote key={result.id} rows={p.session?.rule_coverage?.[result.source?.artifact_id || result.results[0]?.version] || []}
                    busy={p.busy || p.dirty || !!p.content.trim() || (buildJourney && result.source?.artifact_id !== p.artifact?.id)} onSuggest={rule => {
                      p.onNavigate("build", result.source?.artifact_id || result.results[0]?.version);
                      p.onContent(`I’d like one additional test for this rule: ${rule}. Keep every existing test and its grading unchanged. Prepare the extra test for review; do not run it.`);
                    }} />
                )}
              </motion.div>
            ) : buildJourney && p.session && !welcome ? null : waitingForResult ? null : (
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
                          <Message message={m} animate={false} /><ConversationGuidance cards={m.cards} scope={p.session?.document.conversation_state?.brief.scope_id} message={m.id} />

                          {m.role === "user" && m.operation_id && (
                            <OperationFeedback serverTime={p.session?.server_time} original={operationByID.get(m.operation_id)} operation={latestOperations.get(m.operation_id)}
                              primary={primary === "recovery" && latestOperations.get(m.operation_id)?.id === operations.at(-1)?.id} busy={p.busy || p.dirty} pendingID={p.retryPendingOperationID} uncertainID={p.retryUncertainOperationID} onRetry={p.onRetry} retryModels={p.retryModels} />
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
                        <Message message={m} /><ConversationGuidance cards={m.cards} scope={p.session?.document.conversation_state?.brief.scope_id} message={m.id} />

                        {m.role === "user" && m.operation_id && (
                          <OperationFeedback serverTime={p.session?.server_time} original={operationByID.get(m.operation_id)} operation={latestOperations.get(m.operation_id)}
                            primary={primary === "recovery" && latestOperations.get(m.operation_id)?.id === operations.at(-1)?.id} busy={p.busy || p.dirty} pendingID={p.retryPendingOperationID} uncertainID={p.retryUncertainOperationID} onRetry={p.onRetry} retryModels={p.retryModels} />
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
                  ) : null)}
              </div>
            )}
            {p.view === "build" && !intake && p.session && !awaitingBuildAnswer && p.interactionActions && p.onChoice && p.onReloadChoices && (
              <div className="mt-4"><ConversationActions key={p.session.id} session={p.session} busy={p.busy || p.dirty} primary={primary === "choices"} onAction={p.onChoice} onReload={p.onReloadChoices} /></div>
            )}
            {!buildJourney && p.artifact?.agent_prompt && p.view === "build" && !activityLabel && <VibeButton disabled={p.busy || p.dirty} onClick={() => switchView("try")}>Try it yourself</VibeButton>}
            {buildJourney && !p.artifact && active?.state === "RUNNING" && active.conversation_decision?.intent === "prepare_tests" && <p className="vibe-build-intro">I’m making a first version, then I’ll try three situations and check its replies. Your business systems won’t be connected.</p>}
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
            {p.session?.document.build?.phase === "error" && !activityLabel && !operations.some(operation => operation.error && operation.id === p.session?.operations.at(-1)?.id) && (
              <p role="alert" className="mt-4 text-sm text-builder-warn">
                {p.session.document.build.error?.message || "This Build cycle stopped. Your work is saved."}{" "}
                {p.session.document.artifacts.length ? "You can edit this prototype and use Run tests for a fresh estimate." : "Use New agent to start another prototype."}
              </p>
            )}
            {awaitingBuildAnswer && p.session?.document.conversation_state?.pending_question?.purpose !== "clarify_job" && !p.busy && <div className="mt-4"><VibeButton onClick={p.onSample}>Use a sample policy</VibeButton><p className="mt-2 text-xs vibe-muted">A labelled demonstration. Your real policy stays unspecified.</p></div>}
            {pendingDemo && !p.busy && <div className="mt-4"><VibeButton onClick={() => p.onDemo?.(pendingDemo.id)}>Try a sample email assistant</VibeButton><p className="mt-2 text-xs vibe-muted">Uses fictional messages. You can describe your own task instead.</p></div>}
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
                      onClick={changeSource}
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
      {composerDock}
      <SavedWorkDialog open={historyOpen} onOpenChange={setHistoryOpen} testJourney={p.testJourney}
        savedChecks={p.savedChecks} hasResults={runs.length > 0} onResults={() => switchView("checks")} />
    </>
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
