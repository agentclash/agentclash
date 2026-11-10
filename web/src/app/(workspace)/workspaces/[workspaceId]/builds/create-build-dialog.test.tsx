import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

import { CreateBuildDialog } from "./create-build-dialog";

const mocks = vi.hoisted(() => ({ post: vi.fn() }));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: vi.fn(async () => "token") }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/api/client", () => ({
  createApiClient: () => ({ post: mocks.post }),
}));
vi.mock("@/lib/api/swr", () => ({
  useApiMutator: () => ({ mutate: vi.fn() }),
}));
vi.mock("@/components/ui/dialog", () => {
  const passthrough = ({ children }: { children: React.ReactNode }) =>
    React.createElement("div", null, children);
  return {
    Dialog: passthrough,
    DialogTrigger: () => null,
    DialogContent: passthrough,
    DialogDescription: passthrough,
    DialogFooter: passthrough,
    DialogHeader: passthrough,
    DialogTitle: passthrough,
  };
});

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

it("sends one create request when Enter is pressed twice", async () => {
  // The request stays in flight for the whole test.
  mocks.post.mockReturnValue(new Promise(() => {}));
  await act(async () => root.render(<CreateBuildDialog workspaceId="ws-1" />));

  const input = container.querySelector<HTMLInputElement>('input[type="text"]')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(
      input,
      "Agent",
    );
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });

  const enter = () =>
    input.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
    );
  await act(async () => {
    enter();
    enter();
  });
  await act(async () => enter());

  expect(mocks.post).toHaveBeenCalledTimes(1);
});
