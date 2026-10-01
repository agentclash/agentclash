import { VibeError } from "./vibe";

// All mutation callers use the same acknowledgement rule. Once a reply is lost,
// a later rejection describes only that retry, not the original submission.
export type RequestRecovery = { readonly body: string; uncertain: boolean };
export function recoverRequest(request: RequestRecovery, error: unknown): "rejected" | "uncertain" {
  if (!request.uncertain && error instanceof VibeError && error.admission === "rejected" && error.code !== "idempotency_conflict") return "rejected";
  request.uncertain = true;
  return "uncertain";
}
