import { NextRequest, type NextFetchEvent } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const authkitMock = vi.hoisted(() =>
  vi.fn(async () => {
    const { NextResponse } = await import("next/server");
    return NextResponse.next();
  }),
);

vi.mock("@workos-inc/authkit-nextjs", () => ({
  authkitMiddleware: () => authkitMock,
}));

import middleware from "./middleware";

const event = {} as NextFetchEvent;

function request(path: string) {
  return new NextRequest(`https://www.agentclash.dev${path}`);
}

describe("middleware AuthKit coverage", () => {
  afterEach(() => vi.unstubAllEnvs());

  it.each(["GET", "HEAD"])("keeps private Vibe %s on the HTML AuthKit route", async method => {
    vi.stubEnv("MARKDOWN_NEGOTIATION_ENABLED", "true");
    const response = await middleware(new NextRequest("https://www.agentclash.dev/vibe-evals?session=private", { method, headers: { accept: "text/markdown" } }), event);
    expect(authkitMock).toHaveBeenCalledOnce();
    expect(response.headers.get("x-middleware-rewrite")).toBeNull();
    expect(response.headers.get("link")).toBeNull();
  });

  beforeEach(() => {
    authkitMock.mockClear();
  });

  it.each(["/", "/docs", "/blog", "/compare", "/publications", "/pricing"])(
    "runs AuthKit on public HTML %s",
    async (path) => {
      await middleware(request(path), event);
      expect(authkitMock).toHaveBeenCalledOnce();
    },
  );

  it("does not run AuthKit on machine markdown", async () => {
    await middleware(request("/md/pricing"), event);
    expect(authkitMock).not.toHaveBeenCalled();
  });
});
