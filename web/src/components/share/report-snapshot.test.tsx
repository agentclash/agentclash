import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
vi.mock("@workos-inc/authkit-nextjs/components", () => ({ useAccessToken: () => ({ getAccessToken: async () => "token" }), useAuth: () => ({ user: null }) }));
vi.mock("@workos-inc/authkit-nextjs", () => ({}));
import { PublicShareRenderer } from "./public-share-renderers";
it("renders the copied report with exact scalar text and explicit subset scope", async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    const node = document.createElement("div"), root = createRoot(node);
    document.body.append(node);
    try {
        await act(async () => root.render(<PublicShareRenderer resource={{ type: "report_snapshot", content: { origin: "imported", status: "failed", omitted_cases: 2, omissions: ["Source labels and full messages omitted."], cases: [{ entry_id: "case", label: "Case 1", status: "failed", capture_assurance: "imported", capture_scope: { messages: { scope: "complete", instrumentation: "instrumented" }, tools: { scope: "unknown", instrumentation: "unknown" }, retrieval: { scope: "unknown", instrumentation: "unknown" } }, omitted_entries: 3, entries: [{ entry_id: "metric", kind: "grade", label: "Grade 1", facts: [{ label: "score", value: "0.12345678901234567890123456789" }], text: [], text_selected: false }] }] } }}/>));
        expect(node.textContent).toContain("0.12345678901234567890123456789");
        expect(node.textContent).toContain("2 cases omitted");
        expect(node.textContent).toContain("Original source capture");
        expect(node.textContent).not.toContain("observed none");
        expect(node.querySelector("form")).toBeNull();
    }
    finally {
        await act(async () => root.unmount());
        node.remove();
        vi.unstubAllGlobals();
    }
});
