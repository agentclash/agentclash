import { expect, it } from "vitest";
import { resultHeadline } from "./result-headline";

it.each([
  [0, 1, "Your agent passed 0 of 1 test"],
  [1, 1, "Your agent passed all 1 test"],
  [0, 3, "Your agent passed 0 of 3 tests"],
  [2, 3, "Your agent passed 2 of 3 tests"],
  [3, 3, "Your agent passed all 3 tests"],
])("reports %i passed out of %i with the correct test noun", (passed, total, headline) => {
  expect(resultHeadline(passed, total)).toBe(headline);
});
