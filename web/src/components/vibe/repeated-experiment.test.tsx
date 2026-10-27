import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { RepeatedExperiment } from "./repeated-experiment";
import type { CanonicalResult } from "@/lib/vibe-canonical";
const calls = vi.hoisted(() => ({ post: vi.fn() }));
vi.mock("@/lib/api/client", () => ({ createApiClient: () => ({ post: calls.post }) }));
const source = { binding: { session_id: "project", artifact_id: "tests", challenge_pack_version_id: "pack", result_ref: { kind: "run", id: "run" } }, name: "Application", participants: [{ deployment_id: "deployment", snapshot_id: "snapshot", label: "Application", challenge_input_set_id: "inputs", cases: [{ source_case_id: "case", challenge_identity_id: "challenge" }] }] } as CanonicalResult;
const quote = { deployment_snapshot_ids: { deployment: "snapshot" }, controlled_settings: { snapshot: "hash" }, participants: [], case_ids: ["case"], child_runs: 3, participant_samples: 3, case_samples: 3, estimated_usd: null, price_assumptions: "Unknown external charges", limits: [], changed_variables: ["repetition"] };
afterEach(() => { vi.clearAllMocks(); vi.unstubAllGlobals(); localStorage.clear(); });
it("previews exact scope and recovers the same Fleet admission after lost response and remount", async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    const node = document.createElement("div");
    document.body.append(node);
    let root = createRoot(node);
    const created = vi.fn();
    const props = { source, workspaceId: "workspace", actorScope: "actor", canWrite: true, token: async () => "token", onCreated: created };
    const click = async (label: string) => { const button = Array.from(node.querySelectorAll("button")).find(v => v.textContent === label); expect(button).toBeTruthy(); await act(async () => button!.click()); };
    calls.post.mockResolvedValueOnce(quote).mockRejectedValueOnce(Error("lost acknowledgment"));
    try {
        await act(async () => root.render(<RepeatedExperiment {...props}/>));
        const mode = node.querySelector("select")!;
        await act(async () => { mode.value = "fleet"; mode.dispatchEvent(new Event("change", { bubbles: true })); });
        await click("Preview repeated batch");
        expect(calls.post.mock.calls[0][0]).toBe("/v1/eval-sets/controlled/preview");
        expect(calls.post.mock.calls[0][1]).toMatchObject({ challenge_pack_version_id: "pack", challenge_input_set_id: "inputs", challenge_identity_ids: ["challenge"], deployment_snapshot_ids: { deployment: "snapshot" }, eval_session: { repetitions: 3, aggregation: { report_variance: true }, routing_task_snapshot: { routing: { mode: "single_agent" }, task: { challenge_pack_version_id: "pack" } } } });
        await click("Run repeated batch");
        const original = calls.post.mock.calls[1][1];
        expect(original.admission_request_id).toBeTruthy();
        expect(original.controlled_settings).toEqual({ snapshot: "hash" });
        await act(async () => root.unmount());
        root = createRoot(node);
        await act(async () => root.render(<RepeatedExperiment {...props}/>));
        expect(calls.post).toHaveBeenCalledTimes(2);
        expect(node.textContent).toContain("Retry batch admission");
        calls.post.mockResolvedValueOnce({ eval_set: { id: "fleet" }, eval_session_ids: ["child"] });
        await click("Retry batch admission");
        expect(calls.post.mock.calls[2]).toEqual(["/v1/eval-sets/controlled", original]);
        expect(created).toHaveBeenCalledWith({ ...source.binding, result_ref: { kind: "eval_session", id: "child" } });
        expect(localStorage.length).toBe(0);
    }
    finally {
        await act(async () => root.unmount());
        node.remove();
    }
});
it("keeps an uncertain receipt through revoked access and recognizes a terminal Fleet with no children", async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    const node = document.createElement("div"), root = createRoot(node);
    document.body.append(node);
    const created = vi.fn();
    const request = { workspace_id: "workspace", admission_request_id: "fixed-request" };
    const key = "vibe-experiment:actor:project:tests:run:run";
    localStorage.setItem(key, JSON.stringify({ mode: "fleet", request, preview: quote }));
    const { ApiError } = await import("@/lib/api/errors");
    calls.post.mockRejectedValueOnce(new ApiError(403, "forbidden", "Access revoked"));
    try {
        await act(async () => root.render(<RepeatedExperiment source={source} workspaceId="workspace" actorScope="actor" canWrite token={async () => "token"} onCreated={created}/>));
        const retry = () => Array.from(node.querySelectorAll("button")).find(v => v.textContent === "Retry batch admission")!;
        await act(async () => retry().click());
        expect(localStorage.getItem(key)).not.toBeNull();
        expect(retry()).toBeTruthy();
        expect(created).not.toHaveBeenCalled();
        calls.post.mockRejectedValueOnce(new ApiError(409, "experiment_conflict", "Settings changed"));
        await act(async () => retry().click());
        expect(localStorage.getItem(key)).not.toBeNull();
        expect(created).not.toHaveBeenCalled();
        calls.post.mockResolvedValueOnce({ eval_set: { id: "outer", status: "failed" }, eval_session_ids: [] });
        await act(async () => retry().click());
        expect(localStorage.getItem(key)).toBeNull();
        expect(node.textContent).toContain("Fleet failed before a child was admitted");
        expect(created).not.toHaveBeenCalled();
    }
    finally {
        await act(async () => root.unmount());
        node.remove();
    }
});
