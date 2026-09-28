"use client";
import { completionAcknowledgement, terminal, type Operation } from "@/lib/vibe";
import { RetryControl } from "./retry-control";
import { RetryModelControl } from "./retry-model-control";
import { recoveryMessage } from "./recovery-message";

export function OperationFeedback({ original, operation, busy, pendingID, uncertainID, onRetry, primary, serverTime, retryModels = [] }: {
  serverTime?: string;
  primary?: boolean;
  original?: Operation;
  operation?: Operation;
  busy: boolean;
  pendingID?: string;
  uncertainID?: string;
  onRetry?: (id: string, assistantModel?: string) => void;
  retryModels?: { id: string; name: string }[];
}) {
  if (!operation || !original?.error) return null;
  const acknowledgement = completionAcknowledgement(operation);
  if (operation.completion_receipt)
    return <p role="status" className="mt-3 text-sm vibe-muted">{operation.id !== original.id ? "Completed on retry." : acknowledgement || "This request completed."}</p>;
  const uncertain = uncertainID === operation.id;
  const retrying = pendingID === operation.id || !terminal(operation.state);
	const rateLimited = operation.error?.code === "provider_rate_limit" || original.error.code === "provider_rate_limit";
  const message = recoveryMessage(operation) || original.error.message;
  return (
    <div className="mt-3 space-y-2 text-sm">
      <p role="alert" className="text-builder-warn">{message}</p>
      {uncertain && <p className="vibe-muted">The retry acknowledgement wasn’t confirmed. Retry to recover its saved status.</p>}
      {retrying && <p role="status" className="vibe-muted">Retrying this request…</p>}
      {onRetry && (operation.retryable || uncertain) && (
        <div className="flex flex-wrap items-start gap-2">
        <RetryControl key={`${operation.id}:${uncertain ? "recover" : operation.error?.retry_available_at || "ready"}`} availableAt={uncertain ? undefined : operation.error?.retry_available_at} serverTime={serverTime}
          primary={primary} disabled={retrying || (busy && !uncertain)} onRetry={() => onRetry(operation.id)}
          label={rateLimited ? "Try again" : "Retry"} />
        {!uncertain && <RetryModelControl key={operation.id} disabled={retrying || busy}
          models={retryModels.filter(model => model.id !== operation.models.assistant)}
          onRetry={model => onRetry(operation.id, model)} />}
        </div>
      )}
    </div>
  );
}
