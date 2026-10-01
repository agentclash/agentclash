import type { CaseResult } from "./vibe";

// Snapshot rows omit reply bodies. Only the authorized case endpoint can
// provide a preview, and it must match the row whose verdict we display.
export function validCaseEvidence(summary: Pick<CaseResult, "case_key" | "version">, result: CaseResult): boolean {
  return result.case_key === summary.case_key &&
    result.version === summary.version &&
    Array.isArray(result.checks) &&
    typeof result.output === "string";
}
