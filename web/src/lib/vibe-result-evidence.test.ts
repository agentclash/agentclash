import { expect, it } from "vitest";
import type { CaseResult } from "./vibe";
import { resultSlotStatus } from "./vibe-result-evidence";

const pass: CaseResult = {
  case_key: "pass", version: "v1", input: null, output: "Reply", verdict: "PASS",
  checks: [{ key: "policy", verdict: "PASS", evidence: "Follows the policy." }],
};
const fail: CaseResult = { ...pass, case_key: "fail", verdict: "FAIL", checks: [{ key: "policy", verdict: "FAIL", evidence: "Violates the policy." }] };
const incomplete: CaseResult = { ...pass, case_key: "unfinished", expected_checks: 2 };

it.each([
  { name: "all pass", results: [pass, { ...pass, case_key: "second" }], total: 2, slots: ["pass", "pass"], counts: { pass: 2, fail: 0, unresolved: 0 } },
  { name: "one fail", results: [pass, fail], total: 2, slots: ["pass", "fail"], counts: { pass: 1, fail: 1, unresolved: 0 } },
  { name: "one incomplete PASS", results: [pass, incomplete], total: 2, slots: ["pass", "unresolved"], counts: { pass: 1, fail: 0, unresolved: 1 } },
  { name: "one missing result", results: [pass], total: 2, slots: ["pass", "unresolved"], counts: { pass: 1, fail: 0, unresolved: 1 } },
  { name: "one incomplete and one missing", results: [pass, incomplete], total: 3, slots: ["pass", "unresolved", "unresolved"], counts: { pass: 1, fail: 0, unresolved: 2 } },
])("derives exclusive slot statuses and counts for $name", ({ results, total, slots, counts }) => {
  expect(resultSlotStatus(results, total)).toEqual({ slots, counts });
});
