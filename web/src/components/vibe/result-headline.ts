export function resultHeadline(passed: number, total: number) {
  return passed === total ? `Your agent passed all ${total} tests` : `Your agent passed ${passed} of ${total} tests`;
}
