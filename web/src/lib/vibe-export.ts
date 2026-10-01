import type { CaseResult, Session } from "./vibe";

// Session snapshots intentionally omit full case bodies. Exports must retrieve
// the authorized evidence instead of silently exporting only the scorecard.
export async function exportRuns(session: Session, load: (id: string, key: string) => Promise<CaseResult>) {
  return Promise.all(session.operations.map(async operation => ({
    ...operation,
    results: await Promise.all(operation.results.map(result => load(operation.id, result.case_key))),
  })));
}

export function downloadJSON(name: string, value: unknown) {
  const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], { type: "application/json" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
