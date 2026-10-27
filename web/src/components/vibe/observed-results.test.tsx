import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { ObservedResults } from "./observed-results";
import type { ObservedEvidence } from "@/lib/vibe-observed";
const mocks = vi.hoisted(() => ({ load: vi.fn() }));
vi.mock("@/lib/vibe-observed", async (original) => ({ ...await original<typeof import("@/lib/vibe-observed")>(), loadObservedEvidence: mocks.load }));
const binding = { session_id: "project", artifact_id: "artifact", result_ref: { kind: "sdk_report" as const, id: "report" } };
const unknown = { scope: "unknown" as const, instrumentation: "unknown" as const };
const evidence: ObservedEvidence = { binding, status: "imported", source_hash: "source", evidence_hash: "frozen", evidence_metadata: { result_ref: binding.result_ref, origin: "imported", fingerprints: { model: null, build: null, test: null, judge: null, config: null }, application_revision: null, capture_scope: { messages: unknown, tools: unknown, retrieval: unknown } }, omissions: [], cases: [{ case_key: "one", source_case_id: "private-source", entry_id: "entry-one", source_index: 0, status: "failed", capture_scope: { messages: unknown, tools: { scope: "complete", instrumentation: "instrumented" }, retrieval: unknown }, capture_assurance: "imported", entries: [{ entry_id: "grade", kind: "grade", source_path: "/cases/0/metrics/0", source_index: 0, score_text: "0.1234567890123456789", value_json: '{"score":0.1234567890123456789,"passed":false}', value: { score: 0.12345678901234568, passed: false } }, { entry_id: "proposal", kind: "tool_proposal", source_path: "/events/1/tool_calls/0", source_index: 0, value: { name: "lookup" } }] }] };
afterEach(() => { mocks.load.mockReset(); vi.clearAllMocks(); vi.useRealTimers(); vi.unstubAllGlobals(); });
it("shows original numeric grade, separates proposals, and only calls complete instrumented empty observed none", async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    mocks.load.mockResolvedValue(evidence);
    const node = document.createElement("div"), root = createRoot(node);
    document.body.append(node);
    try {
        await act(async () => root.render(<ObservedResults binding={binding} actorScope="actor" workspaceId="workspace" token={async () => "token"} access={{ status: "ready", canRead: true, canWrite: false, canAdmin: false }} onExport={vi.fn()}/>));
        expect(node.textContent).toContain("Score 0.1234567890123456789");
        expect(node.textContent).toContain("Tool proposals");
        expect(node.textContent).toContain("Tools: observed none");
        expect(node.textContent).toContain("Retrieval: unknown");
        expect(node.textContent).toContain("Imported");
    }
    finally {
        await act(async () => root.unmount());
        node.remove();
    }
});
it("discards a late source load after actor identity changes", async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    let resolve!: (v: ObservedEvidence) => void;
    mocks.load.mockReturnValueOnce(new Promise<ObservedEvidence>(r => { resolve = r; })).mockResolvedValue({ ...evidence, cases: [] });
    const node = document.createElement("div"), root = createRoot(node);
    document.body.append(node);
    const props = { binding, workspaceId: "workspace", token: async () => "token", access: { status: "ready" as const, canRead: true, canWrite: false, canAdmin: false }, onExport: vi.fn() };
    try {
        await act(async () => root.render(<ObservedResults {...props} actorScope="A"/>));
        await act(async () => root.render(<ObservedResults {...props} actorScope="B"/>));
        await act(async () => resolve(evidence));
        expect(node.textContent).not.toContain("private-source");
        expect(node.textContent).not.toContain("Score 0.1234567890123456789");
    }
    finally {
        await act(async () => root.unmount());
        node.remove();
    }
});

// Keep the same selected binding and real evidence-owner lease while the source
// advances. Every Vibe nonterminal state must refresh; every terminal must stop.
it.each([
    ["RUNNING", "COMPLETED"],
    ["FINALIZING", "PARTIAL"],
    ["CANCELLING", "CANCELLED"],
    ["CREATED", "FAILED"],
    ["VALIDATING", "EXPIRED"],
    ["AWAITING_INPUT", "COMPLETED"],
    ["AWAITING_APPROVAL", "COMPLETED"],
    ["RESERVED", "COMPLETED"],
    ["QUEUED", "COMPLETED"],
])("refreshes stable Vibe evidence from %s to %s then stops", async (pendingState, terminalState) => {
    vi.useFakeTimers();
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    const selected = {...binding, result_ref: {kind: "vibe_operation" as const, id: "operation"}};
    const pending: ObservedEvidence = {...evidence, binding: selected, status: pendingState, cases: [{...evidence.cases[0], status: "not_recorded", entries: []}]};
    const completed: ObservedEvidence = {...pending, status: terminalState, cases: [{...pending.cases[0], status: "PASS", entries: [
        {entry_id: "grade", kind: "grade", source_path: "/cases/0/grade", source_index: 0, value: {verdict: "PASS"}},
        {entry_id: "output", kind: "output", source_path: "/cases/0/output", source_index: 0, value: "Final retained answer"},
    ]}]};
    mocks.load.mockResolvedValueOnce(pending).mockResolvedValue(completed);
    const node = document.createElement("div"), root = createRoot(node);
    document.body.append(node);
    try {
        await act(async () => root.render(<ObservedResults binding={selected} actorScope="actor" workspaceId="workspace" token={async () => "token"} access={{status: "ready", canRead: true, canWrite: false, canAdmin: false}} onExport={vi.fn()}/>));
        expect(node.textContent).toContain(pendingState);
        expect(node.textContent).toContain("No retained observations");
        expect(mocks.load).toHaveBeenCalledTimes(1);
        await act(async () => { await vi.advanceTimersByTimeAsync(3000); });
        expect(mocks.load).toHaveBeenCalledTimes(2);
        expect(mocks.load.mock.calls[0]).toEqual(mocks.load.mock.calls[1]);
        expect(node.textContent).toContain(terminalState);
        expect(node.textContent).toContain("Original grades");
        expect(node.textContent).toContain("PASS");
        expect(node.textContent).toContain("Final retained answer");
        await act(async () => { await vi.advanceTimersByTimeAsync(30000); });
        expect(mocks.load).toHaveBeenCalledTimes(2);
    } finally {
        await act(async () => root.unmount());
        node.remove();
    }
});
