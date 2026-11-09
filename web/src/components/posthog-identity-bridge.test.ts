import React, { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const posthog = vi.hoisted(() => ({
  init: vi.fn(),
  capture: vi.fn(),
  identify: vi.fn(),
  reset: vi.fn(),
  get_session_id: vi.fn(() => "session-identity"),
}));
vi.mock("posthog-js", () => ({ default: posthog }));

const bridgeMocks = vi.hoisted(() => ({
  user: null as { id: string } | null,
  getAccessToken: vi.fn(async () => "access-token"),
  getSession: vi.fn(),
}));

vi.mock("@workos-inc/authkit-nextjs/components", () => ({
  useAuth: vi.fn(() => ({ user: bridgeMocks.user, loading: false })),
  useAccessToken: vi.fn(() => ({
    getAccessToken: bridgeMocks.getAccessToken,
  })),
}));
vi.mock("@/lib/api/client", () => ({
  createApiClient: vi.fn(() => ({ get: bridgeMocks.getSession })),
}));

import {
  identifyPostHogSession,
  PostHogIdentityBridge,
} from "./posthog-identity-bridge";
import {
  initPostHog,
  resetPostHog,
  resetPostHogModuleForTests,
} from "@/lib/analytics/posthog-client";

import { recordFirstTouch } from "@/lib/analytics/attribution";

let root: Root | null = null;
let container: HTMLDivElement | null = null;

describe("identified PostHog sessions", () => {
  beforeEach(() => {
    resetPostHogModuleForTests();
    vi.clearAllMocks();
    window.localStorage.clear();
    window.sessionStorage.clear();
    document.cookie = "ac_auth_completed=; Max-Age=0; Path=/";
    bridgeMocks.user = null;
    bridgeMocks.getAccessToken.mockResolvedValue("access-token");
    initPostHog({ apiKey: "phc_test", apiHost: "/ingest" });
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root?.unmount());
    container?.remove();
    root = null;
    container = null;
  });

  it("merges anonymously collected activity and deduplicates callback/session events", () => {
    document.cookie = "ac_auth_completed=1; Path=/; SameSite=Lax";
    const session = {
      user_id: "internal-user-uuid",
      organization_memberships: [
        { organization_id: "org-1", role: "org_admin" },
      ],
      workspace_memberships: [
        { workspace_id: "workspace-1", role: "workspace_admin" },
      ],
    };

    identifyPostHogSession(session);
    identifyPostHogSession(session);

    expect(posthog.identify).toHaveBeenCalledWith(
      "internal-user-uuid",
      { org_ids: ["org-1"], workspace_ids: ["workspace-1"] },
      {},
    );
    expect(
      posthog.capture.mock.calls.filter(([event]) => event === "web.auth.completed"),
    ).toHaveLength(1);
    expect(
      posthog.capture.mock.calls.filter(
        ([event]) => event === "web.app.session_started",
      ),
    ).toHaveLength(1);
  });

  it.each(["claude", "codex"])("retains tagged %s acquisition on internal identity across auth", (host) => {
    window.history.replaceState({}, "", `/tryouts?utm_source=${host}`);
    recordFirstTouch();
    window.history.replaceState({}, "", "/onboarding?utm_source=other");
    recordFirstTouch();
    document.cookie = "ac_auth_completed=1; Path=/; SameSite=Lax";
    const userID = host === "claude" ? "11111111-1111-4111-8111-111111111111" : "22222222-2222-4222-8222-222222222222";
    identifyPostHogSession({ user_id: userID, organization_memberships: [], workspace_memberships: [] });
    expect(posthog.identify).toHaveBeenCalledWith(userID, {org_ids: [], workspace_ids: []}, expect.objectContaining({acquisition_channel: host, acquisition_entry_path: "/tryouts"}));
    expect(posthog.capture.mock.calls.map(([name]) => name)).not.toContain("product.account.signup_completed");
  });

  it("does not restore the old identity when logout races a session fetch", async () => {
    bridgeMocks.user = { id: "workos-user" };
    let resolveSession: ((session: unknown) => void) | undefined;
    bridgeMocks.getSession.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveSession = resolve;
        }),
    );

    await act(async () => {
      root?.render(React.createElement(PostHogIdentityBridge));
      await Promise.resolve();
    });
    expect(bridgeMocks.getSession).toHaveBeenCalledTimes(1);

    act(() => resetPostHog());
    await act(async () => {
      resolveSession?.({
        user_id: "internal-user-uuid",
        organization_memberships: [],
        workspace_memberships: [],
      });
      await Promise.resolve();
    });

    expect(posthog.reset).toHaveBeenCalledWith(true);
    expect(posthog.identify).not.toHaveBeenCalled();
  });
});
