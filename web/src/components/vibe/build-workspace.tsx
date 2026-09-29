"use client";

import type { ComponentProps } from "react";
import { BuildConversation } from "./build-conversation";
import { ConversationGuidance } from "./conversation-guidance";
import { Message } from "./vibe-message";
import { OperationFeedback } from "./operation-feedback";
import type { BuildMessage } from "@/lib/vibe-build-timeline";

type Props = Omit<ComponentProps<typeof BuildConversation>, "pending" | "renderMessage"> & {
  pendingMessage?: BuildMessage;
  recovery: Pick<ComponentProps<typeof OperationFeedback>, "primary" | "pendingID" | "uncertainID" | "onRetry" | "retryModels">;
};

// Build-only message projection. The shared shell owns scrolling and the
// composer; the client remains the only owner of session state and submissions.
export function BuildWorkspace({ pendingMessage, recovery, ...props }: Props) {
  const operations = props.session.operations;
  const originals = new Map(operations.map(o => [o.id, o]));
  const latest = new Map(originals);
  for (const operation of [...operations].reverse()) {
    if (operation.retry_of_operation_id)
      latest.set(operation.retry_of_operation_id, latest.get(operation.id)!);
  }
  return <BuildConversation {...props}
    pending={pendingMessage && <div data-pending-message><Message message={pendingMessage} pending /></div>}
    renderMessage={message => <>
      <Message message={message} animate={false} />
      <ConversationGuidance cards={message.cards} scope={props.session.document.conversation_state?.brief.scope_id} message={message.id} />
      {message.role === "user" && message.operation_id && <OperationFeedback {...recovery}
        serverTime={props.session.server_time} busy={props.busy}
        original={originals.get(message.operation_id)} operation={latest.get(message.operation_id)}
        primary={recovery.primary && latest.get(message.operation_id)?.id === operations.at(-1)?.id} />}
    </>} />;
}
