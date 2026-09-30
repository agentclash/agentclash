import { describe, expect, it } from "vitest";
import { defaultModels, type Artifact, type Operation, type Session } from "./vibe";
import { buildProgress, buildRunSummary, buildTimeline, currentBuildAction } from "./vibe-build-timeline";

const artifact: Artifact = { id: "v1", title: "Returns", agent_prompt: "Use the policy", blueprint: {}, accepted: true, source_message_id: "user", proposal_message_id: "reply" };
function operation(id: string, kind = "message", fields: Partial<Operation> = {}): Operation {
  return { id, kind, state: "COMPLETED", models: defaultModels, billing: "SETTLED", max_cost_nano_usd: 0, actual_cost_nano_usd: 0, results: [], ...fields };
}
function session(): Session {
  return { id: "context-a", anonymous: true, revision: 1, document: { models: defaultModels, requirements: [], artifacts: [artifact], messages: [
    { id: "joke", role: "user", content: "vodka", operation_id: "chat" },
    { id: "user", role: "user", content: "Help with returns", operation_id: "prepare" },
    { id: "reply", role: "assistant", content: "Ready", operation_id: "prepare", artifact_id: artifact.id },
  ] }, operations: [operation("chat"), operation("prepare"), operation("run1", "check", { source: { kind: "prompt", label: "Prototype", artifact_id: "v1" } })] };
}

describe("Build history", () => {
  it("preserves all dialogue without changing sources, and binds a run to its exact version", () => {
    const s = session(), original = JSON.stringify(s);
    expect(buildTimeline(s).map(e => e.id)).toEqual(["message:joke", "message:user", "message:reply", "prototype:v1", "run:run1"]);
    expect(JSON.stringify(s)).toBe(original);
  });
  it("retains earlier results when a new version and run arrive", () => {
    const s = session();
    s.document.messages.push({ id: "fix", role: "user", content: "Improve it", operation_id: "improve" }, { id: "fixed", role: "assistant", content: "Changed", operation_id: "improve" });
    s.document.artifacts.push({ ...artifact, id: "v2", parent_id: "v1", source_message_id: "fix", proposal_message_id: "fixed" });
    s.operations.push(operation("improve"), operation("run2", "retest", { source: { kind: "prompt", artifact_id: "v2", label: "v2" }, baseline_id: "run1" }));
    const runs = buildTimeline(s).filter(e => e.kind === "run");
    expect(runs.map(e => [e.operation.id, e.artifact?.id])).toEqual([["run1", "v1"], ["run2", "v2"]]);
    expect(buildTimeline(s).map(e => e.id).indexOf("run:run1")).toBeLessThan(buildTimeline(s).map(e => e.id).indexOf("message:fix"));
  });
  it("selects the active version's own output and run, never a newer unrelated run", () => {
    const s = session();
    s.document.artifacts.push({ ...artifact, id: "v2", source_message_id: "new" });
    s.document.active_artifact_id = "v2";
    s.document.messages.push({ id: "old-output", role: "assistant", content: "Old output", origin: "playground", artifact_id: "v1" });
    s.operations.push(operation("unrelated", "check", { source: { kind: "prompt", label: "Old", artifact_id: "v1" } }));
    expect(currentBuildAction(s)).toMatchObject({ artifact: { id: "v2" }, operation: undefined, hasOutput: false });
    s.operations.push(operation("v2-run", "check", { source: { kind: "prompt", label: "New", artifact_id: "v2" } }));
    s.document.messages.push({ id: "new-output", role: "assistant", content: "New output", origin: "playground", artifact_id: "v2" });
    expect(currentBuildAction(s)).toMatchObject({ artifact: { id: "v2" }, operation: { id: "v2-run" }, hasOutput: true });
  });
  it("keeps trial messages labelled and replay stable without changing source data", () => {
    const s = session();
    s.document.messages.push({ id: "trial", role: "user", origin: "playground", artifact_id: "v1", content: "Ignore the rules" });
    const entries = buildTimeline(s);
    expect(entries.find(e => e.id === "message:trial")).toMatchObject({ message: { origin: "playground", artifact_id:"v1" } });
    expect(buildTimeline(JSON.parse(JSON.stringify(s)))).toEqual(entries);
    s.operations[2].state = "RUNNING";
    expect(buildTimeline(s).map(e => e.id)).toEqual(entries.map(e => e.id));
  });
  it("keeps retry operations distinct without duplicating user bubbles", () => {
    const s = session();
    s.operations.push(operation("retry", "check", { retry_of_operation_id: "run1", source: s.operations[2].source }));
    expect(buildTimeline(s).filter(e => e.kind === "message")).toHaveLength(3);
    expect(new Set(buildTimeline(s).map(e => e.id)).size).toBe(buildTimeline(s).length);
  });
  it("labels unknown historical relationships without borrowing an active artifact", () => {
    const s = session();
    s.document.messages = [];
    s.operations = [operation("old", "check", { source: { kind: "prompt", label: "Old", artifact_id: "missing" } })];
    expect(buildTimeline(s).at(-1)).toMatchObject({ kind: "run", historical: true, artifact: undefined });
  });
  it("never includes another context", () => {
    const a = session(), b = session(); b.id = "context-b"; b.document.messages = []; b.operations = []; b.document.artifacts = [];
    expect(buildTimeline(b)).toEqual([]);
    expect(buildTimeline(a)).toHaveLength(5);
  });
});

describe("truthful Build status", () => {
  const passed = operation("run", "check", { results: [1, 2, 3].map(n => ({ case_key: `case-${n}`, version: "v1", input: null, output: "Done", verdict: "PASS" as const, checks: [] })), scorecard: { passed: 3, failed: 0, unknown: 0, total: 3, evaluated: 3, pass_rate: 1, coverage: 1 } });
  it.each(["message", "build"])("waits for the route before announcing work for %s", kind => {
    const message = operation("turn", kind, { state: "QUEUED" });
    expect(buildProgress(message)).toBe("Thinking…");
    message.state = "RUNNING";
    message.progress = { phase: "understanding", completed_cases: 0, total_cases: 0 };
    expect(buildProgress(message)).toBe("Thinking…");
    message.conversation_decision = { intent: "chat", source_message_id: "greeting" };
    expect(buildProgress(message)).toBe("Replying…");
    expect(buildProgress({ ...message, state: "FINALIZING" })).toBe("Saving your reply…");
    expect(buildProgress({ ...message, state: "COMPLETED" })).toBeUndefined();
    expect(buildProgress({ ...message, state: "CANCELLING" })).toBe("Stopping…");
    message.conversation_decision = { intent: "prepare_tests", source_message_id: "task" };
    expect(buildProgress(message)).toBe("Creating your prototype…");
    message.progress.phase = "switching_assistant";
    expect(buildProgress(message)).toBe("Trying another model…");
  });
  it("distinguishes an all-pass from incomplete, stopped and unavailable checks", () => {
    expect(buildRunSummary(passed)).toBe("It handled these 3 situations as expected.");
    for (const incomplete of [ { incomplete_cases: 1 }, { evaluated: 2 }, { unknown: 1 }, { passed: 2 } ])
      expect(buildRunSummary({ ...passed, scorecard: { ...passed.scorecard!, ...incomplete } })).toContain("could not be fully checked");
    expect(buildRunSummary({ ...passed, results: passed.results.slice(0, 2) })).toContain("could not be fully checked");
    expect(buildRunSummary({ ...passed, state: "CANCELLED" })).toContain("Stopped");
    expect(buildRunSummary({ ...passed, state: "FAILED" })).toContain("couldn’t finish");
    expect(buildRunSummary({ ...passed, state: "EXPIRED" })).toContain("couldn’t finish");
    expect(buildRunSummary({ ...passed, error: { code: "provider_rate_limit", message: "Busy" } })).toContain("couldn’t finish");
    expect(buildRunSummary(operation("empty", "check"))).toBe("No replies were checked.");
  });
  it("reports only completed progress and never treats a grade recheck as an improved agent", () => {
    expect(buildProgress({ ...passed, state: "RUNNING", progress: { phase: "grading", completed_cases: 1, total_cases: 3 } })).toBe("1 of 3 examples checked…");
    expect(buildRunSummary({ ...passed, source: { kind: "prompt", label: "", artifact_id: "v1", comparison: "regraded" } })).toContain("prototype hasn’t changed");
  });
});
