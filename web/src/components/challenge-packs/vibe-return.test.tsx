import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, expect, it, vi } from "vitest";
import { VibeReturn } from "./vibe-return";

const auth = vi.hoisted(() => ({ user: { id: "actor-A" }, token: vi.fn(async () => "token") }));
const network = vi.hoisted(() => ({ metadata: vi.fn(), bytes: vi.fn() }));
vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAuth: () => ({ user: auth.user }),
  useAccessToken: () => ({ getAccessToken: auth.token }),
}));
vi.mock("@/lib/vibe", () => ({ vibeFetch: network.metadata, vibeBytes: network.bytes }));
afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

it.each(["unmount", "actor change", "mounted"])(
  "retained builder original observes its %s lifetime",
  async (lifetime) => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    vi.stubGlobal(
      "URL",
      Object.assign(URL, {
        createObjectURL: vi.fn(() => "blob:original"),
        revokeObjectURL: vi.fn(),
      }),
    );
    auth.user = { id: "actor-A" };
    network.metadata.mockResolvedValue({ filename: "exact.json", media_type: "application/json" });
    let resolve!: (bytes: Uint8Array) => void;
    network.bytes.mockReturnValue(
      new Promise<Uint8Array>((r) => {
        resolve = r;
      }),
    );
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    const node = document.createElement("div");
    document.body.append(node);
    const root = createRoot(node);
    const render = () =>
      root.render(
        <React.StrictMode>
          <VibeReturn draft="saved-draft" path="/vibe-evals?session=project" />
        </React.StrictMode>,
      );
    try {
      await act(async () => render());
      await act(async () => node.querySelector("button")!.click());
      expect(network.bytes).toHaveBeenCalledWith("/saved-originals/saved-draft", "token");
      if (lifetime === "unmount") await act(async () => root.render(null));
      if (lifetime === "actor change") {
        auth.user = { id: "actor-B" };
        await act(async () => render());
      }
      await act(async () => resolve(new Uint8Array([0xef, 0xbb, 0xbf, 123, 125])));
      expect(click).toHaveBeenCalledTimes(lifetime === "mounted" ? 1 : 0);
    } finally {
      await act(async () => root.unmount());
      node.remove();
      network.bytes.mockReset();
    }
  },
);
