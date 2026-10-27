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
afterEach(() => { vi.clearAllMocks(); vi.unstubAllGlobals(); });
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
