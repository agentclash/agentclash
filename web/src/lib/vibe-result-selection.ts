import type { ResultRef } from "./vibe-result-evidence";
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
export function parseResultSelection(value: string | null | undefined): ResultRef | undefined {
  if (!value) return;
  if (uuid.test(value)) return { kind: "vibe_operation", id: value.toLowerCase() };
  const match = /^(sdk_report|run|eval_session|vibe_operation):(.+)$/.exec(value);
  if (match && uuid.test(match[2])) return { kind: match[1] as ResultRef["kind"], id: match[2].toLowerCase() };
}
export function formatResultSelection(ref: ResultRef): string {
  if (!uuid.test(ref.id) || !["sdk_report", "run", "eval_session", "vibe_operation"].includes(ref.kind)) throw Error("Invalid result reference.");
  return ref.kind === "vibe_operation" ? ref.id.toLowerCase() : `${ref.kind}:${ref.id.toLowerCase()}`;
}
