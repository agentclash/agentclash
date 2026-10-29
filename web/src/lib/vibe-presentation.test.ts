import { expect, it } from "vitest";
import { runStateLabel, testCount } from "./vibe-presentation";

it.each([
  ["COMPLETED", "Completed"], ["RUNNING", "Running"], ["QUEUED", "Queued"], ["FAILED", "Failed"],
  ["CANCELLED", "Stopped"], ["CANCELLING", "Stopping"], ["AWAITING_APPROVAL", "Needs approval"], ["AWAITING_INPUT", "Needs one detail"],
])("labels the %s run state in plain words", (state, label) => {
  expect(runStateLabel(state)).toBe(label);
});

it("uses sentence case for states it does not know", () => {
  expect(runStateLabel("PARTIAL")).toBe("Partial");
  expect(runStateLabel("WAITING_FOR_PROVIDER")).toBe("Waiting for provider");
  expect(runStateLabel("supported")).toBe("Supported");
  expect(runStateLabel("")).toBe("Unknown");
});

it("counts tests with one noun and the right plural", () => {
  expect(testCount(1)).toBe("1 test");
  expect(testCount(3)).toBe("3 tests");
  expect(testCount(0)).toBe("0 tests");
  expect(testCount(1, "new")).toBe("1 new test");
  expect(testCount(2, "existing")).toBe("2 existing tests");
});
