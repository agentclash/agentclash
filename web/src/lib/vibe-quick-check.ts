import { terminal, type Artifact, type Session } from "./vibe";

// The backend grants this signal only for a requested check of real, usable
// evidence. Never infer permission to run from keywords or a draft alone.
export function pendingQuickCheck(
  session: Session | null,
): Artifact | undefined {
  if (
    !session ||
    session.operations.some((operation) => !terminal(operation.state))
  )
    return;
  const artifact = session.document.artifacts.at(-1);
  if (
    !artifact?.quick_check ||
    artifact.accepted ||
    artifact.kind !== "conversation_evaluation" ||
    !artifact.conversation_evaluation?.expectations.length
  )
    return;
  const lastUser = session.document.messages.findLast(
    (message) => message.role === "user" && message.origin !== "playground",
  );
  if (lastUser?.id !== artifact.source_message_id) return;
  const evidenceID = artifact.conversation_evaluation.evidence_set_id;
  if (session.document.active_evidence_id !== evidenceID) return;
  const evidence = session.document.evidence_sets?.find(
    (item) => item.id === evidenceID,
  );
  if (
    !evidence?.conversations.length ||
    evidence.conversations.some(
      (chat) =>
        chat.messages.some((message) => message.role === "unknown") ||
        !chat.messages.some((message) => message.role === "assistant"),
    )
  )
    return;
  if (
    session.operations.some(
      (operation) =>
        (operation.kind === "check" || operation.kind === "retest") &&
        (operation.source?.artifact_id === artifact.id ||
          operation.results.some((result) => result.version === artifact.id)),
    )
  )
    return;
  return artifact;
}

// Artifact IDs are server-generated UUIDs. A reversible namespace transform
// keeps the auto-start identity stable across refreshes and multiple tabs.
export function quickCheckClientID(artifactID: string): string {
  if (
    !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
      artifactID,
    )
  )
    throw new Error(
      "This check has an invalid saved identity. Reload the conversation.",
    );
  const prefix = ((parseInt(artifactID.slice(0, 8), 16) ^ 0x7169636b) >>> 0)
    .toString(16)
    .padStart(8, "0");
  return prefix + artifactID.slice(8);
}
