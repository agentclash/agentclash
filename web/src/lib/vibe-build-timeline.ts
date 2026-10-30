import { terminal, type Artifact, type Operation, type Session } from "./vibe";
import { testCount } from "./vibe-presentation";

export function workingBuildArtifact(session: Session): Artifact | undefined {
  const id = session.document.active_artifact_id || session.document.build?.artifact_id;
  return session.document.artifacts.find(a => a.id === id && !!a.agent_prompt);
}

// Reuse the terminal parent result only for an instruction-only proposal.
// Request admission still validates the selected artifact and frozen grading.
export function instructionChangeBaseline(session?: Session | null, artifact?: Artifact): Operation | undefined {
  const parent = session?.document.artifacts.find(item => item.id === artifact?.parent_id);
  if (!session || !artifact || !parent?.agent_prompt.trim() || !artifact.agent_prompt ||
      parent.agent_prompt === artifact.agent_prompt ||
      JSON.stringify(parent.blueprint) !== JSON.stringify(artifact.blueprint)) return undefined;
  return session.operations.findLast(operation =>
    (operation.kind === "check" || operation.kind === "retest") &&
    operation.results.some(result => result.version === parent.id) && terminal(operation.state));
}

export function buildVersion(session: Session, artifact: Artifact) {
  return session.document.artifacts.filter(a => a.agent_prompt || a.conversation_evaluation).findIndex(a => a.id === artifact.id) + 1;
}

export function currentBuildAction(session: Session): { artifact: Artifact; operation?: Operation; hasOutput: boolean } | undefined {
  const artifact = workingBuildArtifact(session) || session.document.artifacts.filter(a => a.kind === "task_brief").at(-1);
  if (!artifact) return undefined;
  const operation = session.operations.filter(o => (o.kind === "check" || o.kind === "retest")
    && (o.source?.artifact_id || o.results[0]?.version) === artifact.id).at(-1);
  const hasOutput = session.document.messages.some(m => m.origin === "playground" && m.role === "assistant" && m.artifact_id === artifact.id);
  return { artifact, operation, hasOutput };
}

export type BuildMessage = Session["document"]["messages"][number];
export type BuildEntry =
  | { id: string; kind: "message"; message: BuildMessage }
  | { id: string; kind: "prototype"; artifact: Artifact; version: number }
  | { id: string; kind: "run"; operation: Operation; artifact?: Artifact; historical: boolean };

// Display-only projection. These relationships never authorize a run or become
// prompt context. Operation order is the server's persisted journal order.
export function buildTimeline(session: Session): BuildEntry[] {
  const messages = session.document.messages;
  const messageIndex = new Map(messages.map((m, i) => [m.id, i]));
  const operationIndex = new Map(session.operations.map((o, i) => [o.id, i]));
  const artifacts = new Map(session.document.artifacts.map(a => [a.id, a]));
  const groups = new Map<number, BuildEntry[]>();
  const add = (anchor: number, entry: BuildEntry) => {
    const group = groups.get(anchor) || [];
    if (!group.some(e => e.id === entry.id)) group.push(entry);
    groups.set(anchor, group);
  };
  const artifactAnchor = (artifact: Artifact) => messageIndex.get(artifact.proposal_message_id || "")
    ?? messages.findIndex(m => m.role === "assistant" && m.artifact_id === artifact.id);
  session.document.artifacts.forEach((artifact) => {
    if (!artifact.agent_prompt && artifact.kind !== "task_brief" && (artifact.kind !== "test_suite" || artifact.dismissed)) return;
    const anchor = artifactAnchor(artifact);
    add(anchor < 0 ? messages.length : anchor, { id: `prototype:${artifact.id}`, kind: "prototype", artifact, version: buildVersion(session, artifact) });
  });
  session.operations.forEach((operation, index) => {
    if (operation.kind !== "check" && operation.kind !== "retest") return;
    const artifact = artifacts.get(operation.source?.artifact_id || operation.results[0]?.version || "");
    let anchor = -1;
    messages.forEach((message, i) => {
      const rank = operationIndex.get(message.operation_id || "");
      if (rank !== undefined && rank <= index) anchor = i;
    });
    if (artifact) anchor = Math.max(anchor, artifactAnchor(artifact));
    add(anchor < 0 ? messages.length : anchor, { id: `run:${operation.id}`, kind: "run", operation, artifact, historical: anchor < 0 });
  });
  const entries: BuildEntry[] = [];
  messages.forEach((message, i) => {
    if (message.content.trim() || message.cards?.length || message.role === "user")
      entries.push({ id: `message:${message.id}`, kind: "message", message });
    entries.push(...groups.get(i) || []);
  });
  entries.push(...groups.get(messages.length) || []);
  return entries;
}

export function buildProgress(operation: Operation) {
  const message = operation.kind === "message" || operation.kind === "build";
  const intent = operation.conversation_decision?.intent;
  if (operation.state === "CANCELLING") return "Stopping…";
  if (operation.state === "AWAITING_APPROVAL") return "Review the cost before continuing.";
  if (operation.state === "AWAITING_INPUT") return "One detail is needed to continue.";
  if (operation.state === "QUEUED") return message ? "Thinking…" : "Waiting to start…";
  if (operation.state === "FINALIZING") return message ? "Saving your reply…" : "Saving your results…";
  if (terminal(operation.state)) return undefined;
  if (operation.kind === "playground") return "Your agent is replying…";
  if (operation.kind === "check" || operation.kind === "retest") {
    const { completed_cases = 0, total_cases = 0 } = operation.progress || {};
    return completed_cases > 0 && total_cases > 0
      ? `${completed_cases} of ${testCount(total_cases)} finished…`
      : total_cases > 0 ? `Running ${testCount(total_cases)}…` : "Running your tests…";
  }
  if (operation.progress?.phase === "switching_assistant") return "Trying another model…";
  if (intent === "suggest_fix") return "Preparing a suggested improvement…";
  if (intent === "prepare_tests") return buildStageLabel(operation);
  if (intent === "chat") return "Replying…";
  if (intent === "explain_results") return "Reading your results…";
  return "Thinking…";
}

export function buildRunSummary(operation: Operation) {
  const score = operation.scorecard;
  if (operation.state === "CANCELLED") return "Stopped. Your completed results are saved.";
  if (!terminal(operation.state)) return buildProgress(operation) || "Running your tests…";
  if (operation.error || operation.state !== "COMPLETED") return "This run couldn’t finish. Your work is saved.";
  if (!score || score.total === 0) return "No tests were run.";
  if (operation.source?.comparison === "regraded") return "Saved replies regraded. The agent hasn’t changed.";
  if (score.failed) return `${score.failed === 1 ? "One test needs" : `${testCount(score.failed)} need`} attention.`;
  if (score.unknown || score.evaluated !== score.total || score.incomplete_cases || score.passed !== score.total || operation.results.length !== score.total || operation.results.some(c => c.verdict !== "PASS"))
    return "Some tests could not be fully graded.";
  return score.total === 1 ? "It passed this test." : `It passed all ${testCount(score.total)}.`;
}

export const buildStages = ["Understand the task", "Write the instructions", "Write 3 tests", "Run tests"] as const;

export function buildStageIndex(operation?: Operation, phase?: string) {
  if (phase === "checking" || phase === "results" || operation?.kind === "check" || operation?.kind === "retest") return 3;
  if (phase === "ready") return 2;
  if (operation?.progress?.phase === "reviewing" || operation?.progress?.phase === "repairing") return 2;
  if (operation?.progress?.phase === "preparing") return 1;
  return 0;
}

export function buildStageLabel(operation?: Operation, phase?: string) {
  if (phase === "clarifying" || operation?.state === "AWAITING_INPUT") return "Needs your answer";
  const stage = buildStageIndex(operation, phase);
  return stage === 3 ? `Run tests (${operation?.progress?.completed_cases || 0} of ${operation?.progress?.total_cases || 3})` : buildStages[stage];
}
