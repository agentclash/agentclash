import { testCount } from "@/lib/vibe-presentation";

export function resultHeadline(passed: number, total: number) {
  return passed === total ? `Your agent passed all ${testCount(total)}` : `Your agent passed ${passed} of ${testCount(total)}`;
}
