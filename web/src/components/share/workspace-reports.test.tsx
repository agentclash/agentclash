import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
const h = vi.hoisted(() => ({ user: "A", write: true, read: true, token: vi.fn(), fetch: vi.fn(), post: vi.fn() }));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({ useAuth: () => ({ user: { id: h.user }, loading: false }), useAccessToken: () => ({ getAccessToken: h.token }) }));
vi.mock("@/lib/use-vibe-workspace-access", () => ({ useVibeWorkspaceAccess: () => ({ access: { status: "ready", canRead: h.read, canWrite: h.write, canAdmin: false }, retry: vi.fn() }) }));
vi.mock("@/lib/vibe", () => ({ vibeFetch: h.fetch }));
vi.mock("@/lib/api/client", () => ({ createApiClient: () => ({ post: h.post }) }));
import { WorkspaceReports } from "./workspace-reports";
const report = { id: "copy", created_at: "2026-10-27T12:00:00Z", content: { status: "failed", origin: "imported", cases: [], omitted_cases: 1, omissions: ["Private source text omitted"], evidence_hash: "frozen" } };
afterEach(() => { vi.clearAllMocks(); vi.unstubAllGlobals(); h.user = "A"; h.write = true; h.read = true; });
it("uses only independent copy routes and revokes a pending public mutation on role change", async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    h.token.mockResolvedValue("auth");
    h.fetch.mockImplementation(async (path: string) => path.endsWith("/copy") ? report : []);
    const node = document.createElement("div"), root = createRoot(node);
    document.body.append(node);
    try {
        await act(async () => root.render(<WorkspaceReports workspaceId="ws" reportId="copy"/>));
        expect(node.textContent).toContain("Original grades are unchanged");
        expect(h.fetch.mock.calls.every(([path]) => path.startsWith("/workspaces/ws/report-snapshots/"))).toBe(true);
        let resolve!: (v: string) => void;
        h.token.mockReturnValueOnce(new Promise<string>(r => resolve = r));
        const create = Array.from(node.querySelectorAll("button")).find(b => b.textContent === "Create expiring public link")!;
        await act(async () => create.click());
        h.write = false;
        await act(async () => root.render(<WorkspaceReports workspaceId="ws" reportId="copy"/>));
        await act(async () => resolve("auth"));
        expect(h.post).not.toHaveBeenCalled();
        expect(node.textContent).not.toContain("Append annotation");
        expect(node.textContent).not.toContain("Create expiring public link");
        expect(node.textContent).toContain("Selected report");
    }
    finally {
        await act(async () => root.unmount());
        node.remove();
    }
});
it("discards a late recipient read through an A→B→A actor change", async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    h.token.mockResolvedValue("auth");
    let resolve!: (v: unknown) => void;
    h.fetch.mockImplementation((path: string) => path.endsWith("/copy") ? Promise.resolve({ ...report, content: { ...report.content, status: "current" } }) : Promise.resolve([]));
    h.fetch.mockImplementationOnce(() => new Promise(r => resolve = r));
    const node = document.createElement("div"), root = createRoot(node);
    document.body.append(node);
    try {
        await act(async () => root.render(<WorkspaceReports workspaceId="ws" reportId="copy"/>));
        h.user = "B";
        await act(async () => root.render(<WorkspaceReports workspaceId="ws" reportId="copy"/>));
        h.user = "A";
        await act(async () => root.render(<WorkspaceReports workspaceId="ws" reportId="copy"/>));
        await act(async () => resolve({ ...report, content: { ...report.content, status: "STALE_PRIVATE" } }));
        expect(node.textContent).not.toContain("STALE_PRIVATE");
        expect(node.textContent).toContain("current");
    }
    finally {
        await act(async () => root.unmount());
        node.remove();
    }
});
