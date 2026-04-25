import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockWithAuth, mockRedirect, mockCreateApiClient } = vi.hoisted(() => ({
  mockWithAuth: vi.fn(),
  mockRedirect: vi.fn(),
  mockCreateApiClient: vi.fn(),
}));

vi.mock("@workos-inc/authkit-nextjs", () => ({
  withAuth: () => mockWithAuth(),
}));

vi.mock("next/navigation", () => ({
  redirect: (url: string) => mockRedirect(url),
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: (...args: unknown[]) => mockCreateApiClient(...args),
}));

describe("getWorkspaceShellData", () => {
  beforeEach(() => {
    vi.resetModules();
    mockWithAuth.mockReset();
    mockRedirect.mockReset();
    mockCreateApiClient.mockReset();

    mockWithAuth.mockResolvedValue({
      user: {
        firstName: "Ayush",
        lastName: "Parihar",
        email: "ayush@example.com",
        profilePictureUrl: null,
      },
      accessToken: "token",
    });
  });

  it("redirects to login when workspace shell auth fetches fail", async () => {
    const redirectError = new Error("redirect:/auth/login");
    mockRedirect.mockImplementation(() => {
      throw redirectError;
    });
    mockCreateApiClient.mockReturnValue({
      get: vi
        .fn()
        .mockRejectedValueOnce(new Error("session failed"))
        .mockResolvedValueOnce({
          organizations: [],
        }),
    });

    const { getWorkspaceShellData } = await import("./server");

    await expect(getWorkspaceShellData("ws-1")).rejects.toBe(redirectError);
    expect(mockRedirect).toHaveBeenCalledWith("/auth/login");
  });

  it("returns workspace shell data when auth fetches succeed", async () => {
    mockCreateApiClient.mockReturnValue({
      get: vi
        .fn()
        .mockResolvedValueOnce({
          workspace_memberships: [{ workspace_id: "ws-1", role: "workspace_admin" }],
          organization_memberships: [{ organization_id: "org-1", role: "org_admin" }],
        })
        .mockResolvedValueOnce({
          organizations: [
            {
              id: "org-1",
              name: "AgentClash",
              slug: "agentclash",
              role: "org_admin",
              workspaces: [{ id: "ws-1", name: "Main", slug: "main", role: "workspace_admin" }],
            },
          ],
        }),
    });

    const { getWorkspaceShellData } = await import("./server");

    await expect(getWorkspaceShellData("ws-1")).resolves.toMatchObject({
      hasMembership: true,
      hasOrgAccess: true,
      orgName: "AgentClash",
      orgSlug: "agentclash",
    });
  });
});
