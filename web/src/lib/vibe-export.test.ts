import { describe, expect, it, vi } from "vitest";
import { exportRuns } from "./vibe-export";
import type { Session, CaseResult } from "./vibe";

const session = {operations: [{id: "run-a", results: [{case_key: "one"}, {case_key: "two"}]}]} as Session;

describe("saved evidence export", () => {
  it("retrieves the original bodies without modifying the redacted snapshot", async () => {
    const load = vi.fn(async (_: string, key: string) => ({case_key: key, input: "actual input", output: "actual reply"}) as CaseResult);
    const runs = await exportRuns(session, load);
    expect(load).toHaveBeenCalledWith("run-a", "one");
    expect(load).toHaveBeenCalledWith("run-a", "two");
    expect(runs[0].results.map(r => r.output)).toEqual(["actual reply", "actual reply"]);
    expect(session.operations[0].results[0].output).toBeUndefined();
  });

  it("does not present an incomplete archive as a successful full export", async () => {
    await expect(exportRuns(session, async () => {throw new Error("Unavailable");})).rejects.toThrow("Unavailable");
  });
});
