import { expect, it } from "vitest";
import { runStateLabel } from "./vibe-presentation";

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
