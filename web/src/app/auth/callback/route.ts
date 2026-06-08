import { handleAuth } from "@workos-inc/authkit-nextjs";
import { cookies } from "next/headers";
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { RETURNING_COOKIE } from "@/lib/auth/returning";

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
  onSuccess: async () => {
    // Mark this browser as a returning visitor so logged-out marketing surfaces
    // can offer "Sign in" instead of "Sign up". Non-sensitive hint, not auth.
    (await cookies()).set(RETURNING_COOKIE, "1", {
      maxAge: 60 * 60 * 24 * 365,
      path: "/",
      sameSite: "lax",
      secure: (process.env.NEXT_PUBLIC_WORKOS_REDIRECT_URI ?? "").startsWith(
        "https://",
      ),
      domain: process.env.WORKOS_COOKIE_DOMAIN || undefined,
      httpOnly: true,
    });
  },
  onError: ({ error, request }) => {
    logAuthCallbackError(error, request);

    const loginUrl = new URL("/auth/login", request.url);
    loginUrl.searchParams.set("error", "callback_failed");
    return NextResponse.redirect(loginUrl);
  },
});
