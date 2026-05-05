import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CISetupClient } from "./ci-setup-client";

const {
  mockGetAccessToken,
  mockCreateApiClient,
  mockListResponse,
  mockPost,
  mockRunsResponse,
  toast,
} = vi.hoisted(() => ({
  mockGetAccessToken: vi.fn(),
  mockCreateApiClient: vi.fn(),
  mockListResponse: vi.fn(),
  mockPost: vi.fn(),
  mockRunsResponse: vi.fn(),
  toast: Object.assign(vi.fn(), {
    success: vi.fn(),
    error: vi.fn(),
  }),
}));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAccessToken: () => ({ getAccessToken: mockGetAccessToken }),
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: (...args: unknown[]) => mockCreateApiClient(...args),
}));

vi.mock("@/lib/api/swr", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api/swr")>();
  return {
    ...actual,
    useApiListQuery: (path: string) => ({
      data: { items: mockListResponse(path) },
      isLoading: false,
      error: null,
    }),
    usePaginatedApiQuery: () => ({
      data: mockRunsResponse(),
      isLoading: false,
      error: null,
    }),
  };
});

vi.mock("sonner", () => ({ toast }));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement> & { href: string }) =>
    React.createElement("a", { href, ...props }, children),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement>) =>
    React.createElement("button", props, children),
}));

vi.mock("@/components/ui/badge", () => ({
  Badge: ({
    children,
    ...props
  }: React.HTMLAttributes<HTMLSpanElement>) =>
    React.createElement("span", props, children),
}));

vi.mock("@/components/ui/input", () => ({
  Input: (props: React.InputHTMLAttributes<HTMLInputElement>) =>
    React.createElement("input", props),
}));

vi.mock("@/components/ui/tabs", () => ({
  Tabs: ({ children }: { children: React.ReactNode }) =>
    React.createElement("div", null, children),
  TabsList: ({ children }: { children: React.ReactNode }) =>
    React.createElement("div", null, children),
  TabsTrigger: ({ children }: { children: React.ReactNode }) =>
    React.createElement("button", null, children),
  TabsContent: ({ children }: { children: React.ReactNode }) =>
    React.createElement("div", null, children),
}));

vi.mock("lucide-react", () => {
  const icon = () => React.createElement("span", null);
  return {
    CheckCircle2: icon,
    Clipboard: icon,
    Download: icon,
    FileCode2: icon,
    GitBranch: icon,
    Github: icon,
    Loader2: icon,
    Play: icon,
    ShieldCheck: icon,
    TriangleAlert: icon,
  };
});

const cleanups: Array<() => void> = [];

beforeEach(() => {
  document.body.innerHTML = "";
  mockPost.mockReset();
  mockGetAccessToken.mockResolvedValue("token");
  mockCreateApiClient.mockReturnValue({
    get: vi.fn(async (path: string) => {
      if (path.includes("/input-sets")) {
        return {
          items: [
            {
              id: "input-set-1",
              challenge_pack_version_id: "pack-version-1",
              input_key: "default",
              name: "Default",
            },
          ],
        };
      }
      if (path.includes("/agents")) {
        return {
          items: [
            {
              id: "baseline-agent-1",
              run_id: "baseline-run-1",
              lane_index: 0,
              label: "production",
              agent_deployment_id: "deployment-1",
              agent_deployment_snapshot_id: "snapshot-1",
              status: "completed",
              created_at: "2026-05-05T00:00:00Z",
              updated_at: "2026-05-05T00:00:00Z",
            },
          ],
        };
      }
      return { items: [] };
    }),
    post: mockPost,
  });
  mockPost.mockResolvedValue({
    pull_request: {
      number: 7,
      html_url: "https://github.com/acme/support-agent/pull/7",
      state: "open",
      draft: true,
    },
    branch: "agentclash/ci-setup/abc12345",
    base_branch: "main",
    files: [
      { path: ".agentclash/ci.yaml" },
      { path: ".github/workflows/agentclash.yml" },
    ],
  });
  mockListResponse.mockImplementation(listResponse);
  mockRunsResponse.mockReturnValue({
    items: [
      {
        id: "baseline-run-1",
        workspace_id: "ws-1",
        challenge_pack_version_id: "pack-version-1",
        name: "Production baseline",
        status: "completed",
        execution_mode: "single_agent",
        race_context: false,
        official_pack_mode: "full",
        created_at: "2026-05-05T00:00:00Z",
        updated_at: "2026-05-05T00:00:00Z",
        links: { self: "", agents: "" },
      },
    ],
    total: 1,
    limit: 20,
    offset: 0,
  });
  Object.assign(navigator, {
    clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
  });
});

afterEach(() => {
  while (cleanups.length > 0) {
    cleanups.pop()?.();
  }
});

describe("CISetupClient", () => {
  it("renders generated manifest and workflow previews from workspace resources", async () => {
    renderClient();
    await flushEffects();

    const text = document.body.textContent ?? "";
    expect(text).toContain("AgentClash GitHub Actions gate");
    expect(text).toContain('agent_build_id: "build-1"');
    expect(text).toContain('runtime_profile_id: "runtime-1"');
    expect(text).toContain('challenge_pack_version_id: "pack-version-1"');
    expect(text).toContain('run_id: "baseline-run-1"');
    expect(text).toContain(
      "uses: agentclash/agentclash/.github/actions/agentclash-ci@main",
    );
  });

  it("shows blockers when required resources are missing", async () => {
    mockListResponse.mockReturnValue([]);
    mockRunsResponse.mockReturnValue({ items: [], total: 0, limit: 20, offset: 0 });

    renderClient();
    await flushEffects();

    const text = document.body.textContent ?? "";
    expect(text).toContain("Setup needs attention");
    expect(text).toContain("Select an agent build for candidate versions.");
    expect(text).toContain("Select a locked baseline run.");
  });

  it("copies generated file bodies", async () => {
    renderClient();
    await flushEffects();

    const copyButton = Array.from(document.querySelectorAll("button")).find(
      (button) => button.textContent?.includes("Copy"),
    );
    if (!copyButton) throw new Error("Copy button not found");

    await act(async () => {
      copyButton.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
    });

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      expect.stringContaining('agent_build_id: "build-1"'),
    );
    expect(toast.success).toHaveBeenCalledWith("Manifest copied");
  });

  it("creates a GitHub setup pull request from the generated files", async () => {
    renderClient();
    await flushEffects();

    const button = Array.from(document.querySelectorAll("button")).find(
      (item) => item.textContent?.includes("Open setup PR"),
    );
    if (!button) throw new Error("Open setup PR button not found");
    expect(button).not.toHaveProperty("disabled", true);

    await act(async () => {
      button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(mockPost).toHaveBeenCalledWith(
      "/v1/workspaces/ws-1/github/ci-setup-pull-request",
      expect.objectContaining({
        github_repository_id: 456,
        github_installation_id: 123,
        base_branch: "main",
        files: [
          expect.objectContaining({
            path: ".agentclash/ci.yaml",
            content: expect.stringContaining('agent_build_id: "build-1"'),
          }),
          expect.objectContaining({
            path: ".github/workflows/agentclash.yml",
            content: expect.stringContaining("name: AgentClash CI"),
          }),
        ],
      }),
    );
    expect(document.body.textContent).toContain("Setup PR #7 created");
    expect(toast.success).toHaveBeenCalledWith("Created setup PR #7");
  });

  it("disables setup PR creation for manual repositories", async () => {
    mockListResponse.mockImplementation((path: string) =>
      path.includes("/github/repositories") ? [] : listResponse(path),
    );

    renderClient();
    await flushEffects();

    const button = Array.from(document.querySelectorAll("button")).find(
      (item) => item.textContent?.includes("Open setup PR"),
    );
    if (!button) throw new Error("Open setup PR button not found");
    expect(button).toHaveProperty("disabled", true);
  });
});

function renderClient() {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root: Root = createRoot(container);
  act(() => {
    root.render(React.createElement(CISetupClient, { workspaceId: "ws-1" }));
  });
  const rendered = {
    cleanup: () => {
      act(() => root.unmount());
      container.remove();
    },
  };
  cleanups.push(rendered.cleanup);
  return rendered;
}

async function flushEffects() {
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
}

function listResponse(path: string) {
  if (path.includes("/agent-builds")) {
    return [
      {
        id: "build-1",
        workspace_id: "ws-1",
        name: "Support Agent",
        slug: "support-agent",
        lifecycle_status: "active",
        created_at: "2026-05-05T00:00:00Z",
        updated_at: "2026-05-05T00:00:00Z",
      },
    ];
  }
  if (path.includes("/agent-deployments")) {
    return [
      {
        id: "deployment-1",
        organization_id: "org-1",
        workspace_id: "ws-1",
        current_build_version_id: "version-1",
        name: "Production",
        status: "active",
        created_at: "2026-05-05T00:00:00Z",
        updated_at: "2026-05-05T00:00:00Z",
      },
    ];
  }
  if (path.includes("/challenge-packs")) {
    return [
      {
        id: "pack-1",
        name: "Support Regression",
        slug: "support-regression",
        versions: [
          {
            id: "pack-version-1",
            challenge_pack_id: "pack-1",
            version_number: 1,
            lifecycle_status: "runnable",
            created_at: "2026-05-05T00:00:00Z",
            updated_at: "2026-05-05T00:00:00Z",
          },
        ],
        created_at: "2026-05-05T00:00:00Z",
        updated_at: "2026-05-05T00:00:00Z",
      },
    ];
  }
  if (path.includes("/runtime-profiles")) {
    return [
      {
        id: "runtime-1",
        workspace_id: "ws-1",
        name: "Default runtime",
        slug: "default-runtime",
        execution_target: "hosted",
        trace_mode: "full",
        created_at: "2026-05-05T00:00:00Z",
        updated_at: "2026-05-05T00:00:00Z",
      },
    ];
  }
  if (path.includes("/provider-accounts")) return [];
  if (path.includes("/model-aliases")) return [];
  if (path.includes("/regression-suites")) {
    return [
      {
        id: "suite-1",
        workspace_id: "ws-1",
        source_challenge_pack_id: "pack-1",
        name: "Refund regressions",
        description: "",
        status: "active",
        source_mode: "derived_only",
        default_gate_severity: "blocking",
        case_count: 1,
        created_by_user_id: "user-1",
        created_at: "2026-05-05T00:00:00Z",
        updated_at: "2026-05-05T00:00:00Z",
      },
    ];
  }
  if (path.includes("/github/repositories")) {
    return [
      {
        id: "repo-1",
        github_installation_id: 123,
        github_repository_id: 456,
        full_name: "acme/support-agent",
        owner_login: "acme",
        name: "support-agent",
        private: true,
        default_branch: "main",
        html_url: "https://github.com/acme/support-agent",
        clone_url: "https://github.com/acme/support-agent.git",
        permissions: {},
        last_synced_at: "2026-05-05T00:00:00Z",
      },
    ];
  }
  return [];
}
