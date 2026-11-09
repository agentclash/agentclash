import type { RuleCoverage } from "./vibe";

export function coverageProposal(rows: RuleCoverage[]) {
  const gaps = rows.filter(row => !row.case_keys.length).slice(0, 3);
  if (gaps.length) return gaps.map(row => `One test covering: ${row.statement}`);
  return rows.slice(0, 2).map(row => `A boundary or missing-information test for: ${row.statement}`);
}
