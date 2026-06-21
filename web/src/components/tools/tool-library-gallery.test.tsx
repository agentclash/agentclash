import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ToolLibraryGallery } from "./tool-library-gallery";

const {
  mockGetAccessToken,
  mockMutateMany,
  mockPost,
  mockPush,
  mockUseApiListQuery,
  toast,
} = vi.hoisted(() => ({
  mockGetAccessToken: vi.fn(),
  mockMutateMany: vi.fn(),
  mockPost: vi.fn(),
  mockPush: vi.fn(),
  mockUseApiListQuery: vi.fn(),
  toast: { success: vi.fn(), info: vi.fn(), error: vi.fn() },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a href={String(href)} {...props}>{children}</a>
  ),
}));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: () => ({ post: mockPost }),
}));

vi.mock("@/lib/api/swr", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/swr")>();
  return {
    ...actual,
    useApiListQuery: (...args: unknown[]) => mockUseApiListQuery(...args),
    useApiMutator: () => ({ mutateMany: mockMutateMany }),
  };
});

vi.mock("sonner", () => ({ toast }));

vi.mock("lucide-react", () => ({
  Check: () => <span>check</span>,
  FlaskConical: () => <span>mock</span>,
  KeyRound: () => <span>key</span>,
  Loader2: () => <span>loading</span>,
  PencilRuler: () => <span>custom</span>,
  Plus: () => <span>plus</span>,
  Search: () => <span>search</span>,
  Zap: () => <span>live</span>,
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({ render, children, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { render?: React.ReactElement }) =>
    render
      ? React.cloneElement(render, props as React.HTMLAttributes<HTMLElement>, children)
      : <button {...props}>{children}</button>,
}));

vi.mock("@/components/ui/badge", () => ({
  Badge: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
}));

vi.mock("@/components/ui/page-header", () => ({
  PageHeader: ({ title, description, actions }: { title: string; description: string; actions: React.ReactNode }) => (
    <header><h1>{title}</h1><p>{description}</p>{actions}</header>
  ),
}));

vi.mock("@/components/app-shell/workspace-loading", () => ({
  WorkspaceListLoading: () => <div>loading</div>,
}));

const entry = {
  slug: "web-search",
  name: "Search the web",
  category: "Web & data",
  description: "Search the web.",
  tags: ["search"],
  tool_kind: "primitive" as const,
  delivery: "live" as const,
  has_live: false,
  definition: {},
};

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
  });
}

describe("ToolLibraryGallery", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
    mockGetAccessToken.mockReset().mockResolvedValue("token");
    mockMutateMany.mockReset().mockResolvedValue(undefined);
    mockPost.mockReset();
    mockPush.mockReset();
    mockUseApiListQuery.mockReset();
    toast.success.mockReset();
    toast.info.mockReset();
    toast.error.mockReset();
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it("marks a renamed workspace tool as added by its stable slug", () => {
    mockUseApiListQuery.mockImplementation((path: string) => path === "/v1/tool-library"
      ? { data: { items: [entry] }, error: undefined, isLoading: false }
      : { data: { items: [{ slug: "search-the-web", name: "Renamed search" }] } });

    act(() => root.render(<ToolLibraryGallery workspaceId="ws-1" />));

    const addedButton = [...container.querySelectorAll("button")].find((button) => button.textContent?.includes("Added"));
    expect(addedButton).toBeDefined();
    expect(addedButton?.disabled).toBe(true);
  });

  it("posts the catalog slug and navigates to the created tool", async () => {
    mockUseApiListQuery.mockImplementation((path: string) => path === "/v1/tool-library"
      ? { data: { items: [entry] }, error: undefined, isLoading: false }
      : { data: { items: [] } });
    mockPost.mockResolvedValue({
      items: [{ id: "tool-1", name: entry.name, slug: "search-the-web" }],
      skipped: [],
    });

    act(() => root.render(<ToolLibraryGallery workspaceId="ws-1" />));
    const addButton = [...container.querySelectorAll("button")].find((button) => button.textContent?.includes("Add to workspace"));
    expect(addButton).toBeDefined();
    act(() => addButton?.click());
    await flushPromises();

    expect(mockPost).toHaveBeenCalledWith(
      "/v1/workspaces/ws-1/tools/from-library",
      { entries: [{ slug: "web-search" }] },
    );
    expect(mockMutateMany).toHaveBeenCalledOnce();
    expect(mockPush).toHaveBeenCalledWith("/workspaces/ws-1/tools/tool-1");
  });
});
