import { renderToReadableStream } from "react-dom/server";
import { beforeEach, expect, it, vi } from "vitest";
import GitHubSetupPage from "./page";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  withAuth: vi.fn(),
}));
vi.mock("@workos-inc/authkit-nextjs", () => ({ withAuth: mocks.withAuth }));
vi.mock("@/lib/api/client", () => ({
  createApiClient: () => ({ get: mocks.get, post: mocks.post }),
}));
vi.mock("next/navigation", () => ({
  redirect: (url: string) => { throw new Error(url); },
}));

const workspaceID = "018f9f2e-65cb-7d0b-9c98-0df2ce4076d8";
const state = `${Buffer.from(JSON.stringify({ workspace_id: workspaceID })).toString("base64url")}.sig`;
const callback = { installation_id: "42", state, setup_action: "install", code: "oauth_Code-123" };

beforeEach(() => vi.clearAllMocks());

it("links a workspace admin to CI setup when GitHub returns no state", async () => {
  mocks.withAuth.mockResolvedValue({ user: { id: "user-1" }, accessToken: "token" });
  mocks.get.mockResolvedValue({
    user_id: "user-1",
    organizations: [
      {
        id: "org-1",
        role: "org_member",
        workspaces: [{ id: "ws-1", name: "Support", role: "workspace_admin" }],
      },
    ],
  });

  const page = await GitHubSetupPage({
    searchParams: Promise.resolve({ installation_id: "123", setup_action: "install" }),
  });
  const stream = await renderToReadableStream(page);
  const html = await new Response(stream).text();
  const document = new DOMParser().parseFromString(html, "text/html");
  const recoveryLink = Array.from(document.querySelectorAll("a")).find(
    (link) => link.textContent === "Support",
  );

  expect(recoveryLink?.getAttribute("href")).toBe("/workspaces/ws-1/ci-setup");
  expect(mocks.post).not.toHaveBeenCalled();
});

it("submits the GitHub authorization code when completing the installation", async () => {
  mocks.withAuth.mockResolvedValue({ user: { id: "user" }, accessToken: "session-token" });

  await expect(GitHubSetupPage({ searchParams: Promise.resolve(callback) }))
    .rejects.toThrow(`/workspaces/${workspaceID}`);
  expect(mocks.post).toHaveBeenCalledWith(
    `/v1/workspaces/${workspaceID}/github/installations/complete`,
    { installation_id: 42, state, code: callback.code },
  );
});

it("keeps the GitHub authorization code through the login redirect", async () => {
  mocks.withAuth.mockResolvedValue({ user: null, accessToken: null });
  const returnTo = `/github/setup?${new URLSearchParams(callback)}`;

  await expect(GitHubSetupPage({ searchParams: Promise.resolve(callback) }))
    .rejects.toThrow(`/auth/login?returnTo=${encodeURIComponent(returnTo)}`);
  expect(mocks.post).not.toHaveBeenCalled();
});
