import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import type { AgentTryout } from "@/lib/api/types";
import { SaveTryout } from "./save-tryout";
const auth = vi.hoisted(() => ({ getAccessToken: async () => "token", user: { id: "user" } }));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({ useAuth: () => ({ user: auth.user, loading: false }), useAccessToken: () => ({ getAccessToken: auth.getAccessToken }) }));
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });
const result = { id: "result", status: "completed", save_eligible: true } as unknown as AgentTryout;
async function mount(tryout = result, status = 200) {
 vi.stubEnv("NEXT_PUBLIC_API_URL", "https://api.example.test"); vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
 const claims: RequestInit[] = [];
 vi.stubGlobal("fetch", async (url: string, init: RequestInit) => {
  if (url.endsWith("/users/me")) return Response.json({ organizations: [{ role: "org_member", workspaces: [{ id: "workspace", name: "My workspace", role: "workspace_member" }, { id: "viewer", name: "Read only", role: "workspace_viewer" }] }] });
  if (url.endsWith("/agent-tryout-templates")) return Response.json({ items: [], creator_access_protocol: "cookie-v1" });
  claims.push(init);
  return Response.json(status === 200 ? { ...result, workspace_id: "workspace", claimed_by_user_id: "user", claimed_at: "2026-01-01T00:00:00Z" } : { error: { code: "forbidden", message: "Workspace access denied" } }, { status });
 });
 const container = document.createElement("div"); document.body.append(container); const root = createRoot(container); const saved = vi.fn();
 await act(async () => root.render(<SaveTryout tryout={tryout} loginHref="/auth/login" onSaved={saved} />));
 return { container, root, saved, claims, close: async () => { await act(async () => root.unmount()); container.remove(); } };
}
it("requires explicit authorized workspace selection and a real save receipt", async () => {
 const ui = await mount();
 try {
  const select = ui.container.querySelector("select")!;
  expect(select).not.toBeNull(); expect(select.textContent).not.toContain("Read only"); expect(ui.claims).toEqual([]);
  const button = ui.container.querySelector("button")!; expect(button.disabled).toBe(true);
  await act(async () => { select.value = "workspace"; select.dispatchEvent(new Event("change", { bubbles: true })); });
  await act(async () => button.click());
  expect(ui.claims).toHaveLength(1); expect(JSON.parse(ui.claims[0].body as string)).toEqual({ workspace_id: "workspace" }); expect(ui.claims[0].credentials).toBe("include");
  expect(ui.saved).toHaveBeenCalledWith(expect.objectContaining({ id: "result", workspace_id: "workspace" }));
 } finally { await ui.close(); }
});
it("does not enable an old server's ID-only claim", async () => {
 const ui = await mount({ ...result, save_eligible: undefined } as AgentTryout);
 try { expect(ui.container.querySelector('[data-save-state="unavailable"]')).not.toBeNull(); expect(ui.container.querySelector("button")?.disabled ?? true).toBe(true); expect(ui.claims).toEqual([]); } finally { await ui.close(); }
});
it("keeps forbidden save unsaved and recoverable", async () => {
 const ui = await mount(result, 403);
 try {
  const select = ui.container.querySelector("select")!;
  await act(async () => { select.value = "workspace"; select.dispatchEvent(new Event("change", { bubbles: true })); });
  await act(async () => ui.container.querySelector("button")!.click());
  expect(ui.saved).not.toHaveBeenCalled(); expect(ui.container.querySelector('[role="alert"]')).not.toBeNull(); expect(ui.container.querySelector("button")!.disabled).toBe(false);
 } finally { await ui.close(); }
});

for (const status of ["queued", "running"] as const) {
 it(`defers ${status} save even if an older capability says true`, async () => {
  const ui = await mount({ ...result, status });
  try {
   expect(ui.container.querySelector("select")).toBeNull();
   expect(ui.container.querySelector("button")).toBeNull();
   expect(ui.container.querySelector('[data-save-state="active"]')).not.toBeNull();
   expect(ui.claims).toEqual([]);
  } finally { await ui.close(); }
 });
}
it("names the saved workspace and retains exact identities without a UUID banner", async () => {
 const id = "c8000000-0000-4000-8000-000000000001";
 const ui = await mount({ ...result, id, workspace_id: "workspace", save_eligible: false });
 try {
  const receipt = ui.container.querySelector('[data-save-state="saved"]')!;
  expect(receipt?.getAttribute("data-tryout-id")).toBe(id);
  expect(receipt?.getAttribute("data-workspace-id")).toBe("workspace");
  const destination = receipt?.querySelector<HTMLAnchorElement>('a[href="/workspaces/workspace"]');
  expect(destination?.textContent).toBe("My workspace");
  expect(receipt?.querySelector('[role="status"]')?.textContent).not.toContain(id);
  expect(receipt?.querySelector("button")).toBeNull();
  expect(ui.claims).toEqual([]);
 } finally { await ui.close(); }
});
