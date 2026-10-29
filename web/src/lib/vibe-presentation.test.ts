import { expect, it } from "vitest";
import { runStateLabel } from "./vibe-presentation";

it.each([
  ["COMPLETED", "Completed"], ["RUNNING", "Running"], ["QUEUED", "Queued"], ["FAILED", "Failed"],
  ["CANCELLED", "Stopped"], ["CANCELLING", "Stopping"], ["AWAITING_APPROVAL", "Needs approval"], ["AWAITING_INPUT", "Needs one detail"],
])("labels the %s run state in plain words", (state, label) => {
  expect(runStateLabel(state)).toBe(label);
});

it("maps the later backend states and uses sentence case for others", () => {
  expect(runStateLabel("PARTIAL")).toBe("Partly complete");
  expect(runStateLabel("RESERVED")).toBe("Preparing");
  expect(runStateLabel("WAITING_FOR_PROVIDER")).toBe("Waiting for provider");
  expect(runStateLabel("supported")).toBe("Supported");
  expect(runStateLabel("")).toBe("Unknown");
});
