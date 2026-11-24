import { renderToReadableStream } from "react-dom/server";
import { expect, it, vi } from "vitest";
import { createApiClient } from "@/lib/api/client";
import GitHubSetupPage from "./page";

vi.mock("@workos-inc/authkit-nextjs", () => ({
  withAuth: async () => ({ user: { id: "user-1" }, accessToken: "token" }),
}));

vi.mock("@/lib/api/client", () => ({
  createApiClient: vi.fn(),
}));

it("links a workspace admin to CI setup when GitHub returns no state", async () => {
  const get = vi.fn().mockResolvedValue({
    user_id: "user-1",
    organizations: [
      {
        id: "org-1",
        role: "org_member",
        workspaces: [{ id: "ws-1", name: "Support", role: "workspace_admin" }],
      },
    ],
  });
  vi.mocked(createApiClient).mockReturnValue({ get } as ReturnType<typeof createApiClient>);

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
});
