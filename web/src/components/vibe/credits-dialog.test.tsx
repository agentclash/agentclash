import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { CreditsDialog } from "./credits-dialog";

vi.mock("@workos-inc/authkit-nextjs/components", () => ({ useAccessToken: () => ({ getAccessToken: async () => "token" }) }));
afterEach(() => vi.unstubAllGlobals());

it("loads credits only while the controlled dialog is open", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const requests: string[] = [];
  vi.stubGlobal("fetch", vi.fn(async (url: string) => {
    requests.push(new URL(url).search);
    return new Response(JSON.stringify({ under_review: false, balance_nano_usd: 0, held_nano_usd: 0, available_nano_usd: 2_500_000_000, products: [] }), { headers: { "Content-Type": "application/json" } });
  }));
  const container = document.createElement("div");
  document.body.append(container);
  const root = createRoot(container);
  const onOpenChange = vi.fn();
  await act(async () => root.render(<CreditsDialog workspace="workspace one" open={false} onOpenChange={onOpenChange} />));
  expect(requests).toEqual([]);
  expect(document.querySelector('[role="dialog"]')).toBeNull();
  await act(async () => root.render(<CreditsDialog workspace="workspace one" open onOpenChange={onOpenChange} />));
  expect(requests).toEqual(["?workspace_id=workspace%20one"]);
  expect(document.querySelector('[role="dialog"]')?.textContent).toContain("$2.50");
  await act(async () => document.querySelector<HTMLButtonElement>('[role="dialog"] [data-slot="dialog-close"]')!.click());
  expect(onOpenChange).toHaveBeenCalledWith(false, expect.anything());
  await act(async () => root.unmount());
  container.remove();
});
