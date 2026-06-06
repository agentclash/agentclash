import { handleAuth } from "@workos-inc/authkit-nextjs";
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";

function logAuthCallbackError(error: unknown, request: NextRequest) {
  console.error("[auth/callback]", {
    message: error instanceof Error ? error.message : String(error),
    path: request.nextUrl.pathname,
    hasCode: request.nextUrl.searchParams.has("code"),
    hasState: request.nextUrl.searchParams.has("state"),
    hasPkceCookie: request.cookies.has("wos-auth-verifier"),
  });
}

export const GET = handleAuth({
  returnPathname: "/dashboard",
  onError: ({ error, request }) => {
    logAuthCallbackError(error, request);

    const loginUrl = new URL("/auth/login", request.url);
    loginUrl.searchParams.set("error", "callback_failed");
    return NextResponse.redirect(loginUrl);
  },
});
