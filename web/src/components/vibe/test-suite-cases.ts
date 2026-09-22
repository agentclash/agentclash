import { caseInput } from "@/lib/vibe";

export type SuiteCase = { key: string; input: string; expected: string; editable: boolean };

// Display every retained case, including custom imported contracts. Editing is
// narrower: only explicit text fields that the existing judge actually reads.
export function suiteCases(blueprint: unknown): SuiteCase[] {
  if (!blueprint || typeof blueprint !== "object") return [];
  const b = blueprint as { cases?: unknown[]; input_sets?: { cases?: unknown[] }[]; judges?: { context_from?: string[] }[] };
  const cases = b.cases || b.input_sets?.flatMap(set => set.cases || []);
  if (!Array.isArray(cases)) return [];
  const readsExpected = b.judges?.some(j => j.context_from?.includes("case.expectations.expected_behavior"));
  return cases.map((value, i) => {
    const c = value as { key?: string; payload?: { question?: unknown }; expectations?: { key?: string; kind?: string; value?: unknown }[] };
    const expected = c.expectations?.find(e => e.key === "expected_behavior" && e.kind === "text" && typeof e.value === "string");
    return {
      key: c.key || `imported-${i + 1}`,
      input: typeof c.payload?.question === "string" ? c.payload.question : caseInput(c.payload),
      expected: expected ? String(expected.value) : "Uses the grading rules in your imported pack.",
      editable: !!c.key && typeof c.payload?.question === "string" && !!expected && !!readsExpected,
    };
  });
}

export function changedCases(original: SuiteCase[], draft: SuiteCase[]) {
  return draft.flatMap(row => {
    const previous = original.find(c => c.key === row.key);
    if (!previous || !row.editable) return [];
    const input = previous.input !== row.input ? row.input : undefined;
    const expected = previous.expected !== row.expected ? row.expected : undefined;
    return input === undefined && expected === undefined ? [] : [{ action: "update", case_key: row.key, ...(input !== undefined ? { input } : {}), ...(expected !== undefined ? { expected } : {}) }];
  });
}
