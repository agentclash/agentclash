import { describe, expect, it } from "vitest";

import { isCanonicalResultActive } from "./vibe-canonical";

describe("isCanonicalResultActive", () => {
  it("polls a run only while the run is active", () => {
    for (const status of ["queued", "provisioning", "running", "scoring"]) {
      expect(isCanonicalResultActive("run", status)).toBe(true);
    }
    for (const status of ["draft", "completed", "failed", "cancelled"]) {
      expect(isCanonicalResultActive("run", status)).toBe(false);
    }
  });

  it("polls an eval session only while the session is active", () => {
    for (const status of ["queued", "running", "aggregating"]) {
      expect(isCanonicalResultActive("eval_session", status)).toBe(true);
    }
    for (const status of ["completed", "failed", "cancelled"]) {
      expect(isCanonicalResultActive("eval_session", status)).toBe(false);
    }
  });

  it("ignores statuses that the resource never sends", () => {
    // Neither runs nor eval sessions send "pending".
    expect(isCanonicalResultActive("run", "pending")).toBe(false);
    expect(isCanonicalResultActive("eval_session", "pending")).toBe(false);
    expect(isCanonicalResultActive("run", "aggregating")).toBe(false);
    expect(isCanonicalResultActive("eval_session", "provisioning")).toBe(false);
    expect(isCanonicalResultActive("eval_session", "scoring")).toBe(false);
  });

  it("never polls other result kinds", () => {
    expect(isCanonicalResultActive("sdk_report", "queued")).toBe(false);
    expect(isCanonicalResultActive("vibe_operation", "running")).toBe(false);
  });
});
