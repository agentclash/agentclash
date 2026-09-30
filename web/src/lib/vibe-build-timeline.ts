import { terminal, type Artifact, type Operation, type Session } from "./vibe";

export function workingBuildArtifact(session: Session): Artifact | undefined {
  const id = session.document.active_artifact_id || session.document.build?.artifact_id;
  return session.document.artifacts.find(a => a.id === id && !!a.agent_prompt);
}

export function buildVersion(session: Session, artifact: Artifact) {
  return session.document.artifacts.filter(a => a.agent_prompt).findIndex(a => a.id === artifact.id) + 1;
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
  if (operation.kind === "playground") return "Your prototype is replying…";
  if (operation.kind === "check" || operation.kind === "retest") {
    const { completed_cases = 0, total_cases = 0 } = operation.progress || {};
    return completed_cases > 0 && total_cases > 0
      ? `${completed_cases} of ${total_cases} examples checked…`
      : total_cases > 0 ? `Trying ${total_cases} examples…` : "Trying your examples…";
  }
  if (operation.progress?.phase === "switching_assistant") return "Trying another model…";
  if (intent === "suggest_fix") return "Preparing a suggested improvement…";
  if (intent === "prepare_tests") return "Creating your prototype…";
  if (intent === "chat") return "Replying…";
  if (intent === "explain_results") return "Reading your results…";
  return "Thinking…";
}

export function buildRunSummary(operation: Operation) {
  const score = operation.scorecard;
  if (operation.state === "CANCELLED") return "Stopped. Your completed examples are saved.";
  if (!terminal(operation.state)) return buildProgress(operation) || "Trying your examples…";
  if (operation.error || operation.state !== "COMPLETED") return "This check couldn’t finish. Your work is saved.";
  if (!score || score.total === 0) return "No replies were checked.";
  if (operation.source?.comparison === "regraded") return "Saved replies regraded. The prototype hasn’t changed.";
  if (score.failed) return `${score.failed === 1 ? "One example needs" : `${score.failed} examples need`} attention.`;
  if (score.unknown || score.evaluated !== score.total || score.incomplete_cases || score.passed !== score.total || operation.results.length !== score.total || operation.results.some(c => c.verdict !== "PASS"))
    return "Some examples could not be fully checked.";
  return score.total === 1 ? "It handled this situation as expected." : `It handled these ${score.total} situations as expected.`;
}
