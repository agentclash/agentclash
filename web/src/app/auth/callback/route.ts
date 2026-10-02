import { callbackIntentCookie, readCallbackIntent } from "@/lib/auth/callback-intent";
import { handleAuth } from "@workos-inc/authkit-nextjs";
import { cookies } from "next/headers";
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { RETURNING_COOKIE } from "@/lib/auth/returning";
import { AUTH_COMPLETED_COOKIE } from "@/lib/analytics/auth-marker";

function logAuthCallbackError(error: unknown, request: NextRequest) {
  console.error("[auth/callback]", {
    message: error instanceof Error ? error.message : String(error),
    path: request.nextUrl.pathname,
    hasCode: request.nextUrl.searchParams.has("code"),
    hasState: request.nextUrl.searchParams.has("state"),
    hasPkceCookie: request.cookies.getAll().some(cookie => cookie.name === "wos-auth-verifier" || cookie.name.startsWith("wos-auth-verifier-")),
  });
}

const configuredCallback = process.env.NEXT_PUBLIC_WORKOS_REDIRECT_URI;
// Next's development request URL may normalize 127.0.0.1 to localhost. Use the
// configured, trusted callback origin so the same browser cookies survive.
const callbackOrigin = configuredCallback ? new URL(configuredCallback).origin : undefined;

const authHandler = handleAuth({
  baseURL: callbackOrigin,
  returnPathname: "/dashboard",
  onSuccess: async () => {
    // Mark this browser as a returning visitor so logged-out marketing surfaces
    // can offer "Sign in" instead of "Sign up". Non-sensitive hint, not auth.
    const cookieStore = await cookies();
    cookieStore.set(RETURNING_COOKIE, "1", {
      maxAge: 60 * 60 * 24 * 365,
      path: "/",
      sameSite: "lax",
      secure: (process.env.NEXT_PUBLIC_WORKOS_REDIRECT_URI ?? "").startsWith(
        "https://",
      ),
      domain: process.env.WORKOS_COOKIE_DOMAIN || undefined,
      httpOnly: true,
    });
    // A short-lived, non-sensitive marker lets the identified client emit one
    // callback completion. It is consumed and removed by the identity bridge.
    cookieStore.set(AUTH_COMPLETED_COOKIE, "1", {
      maxAge: 10 * 60,
      path: "/",
      sameSite: "lax",
      secure: (process.env.NEXT_PUBLIC_WORKOS_REDIRECT_URI ?? "").startsWith(
        "https://",
      ),
      httpOnly: false,
    });
  },
  onError: ({ error, request }) => {
    logAuthCallbackError(error, request);

    const loginUrl = new URL("/auth/login", callbackOrigin || request.url);
    loginUrl.searchParams.set("error", "callback_failed");
    const state = request.nextUrl.searchParams.get("state") || "";
    const trustedOrigin = !!callbackOrigin && (request.nextUrl.origin === callbackOrigin || request.headers.get("host") === new URL(callbackOrigin).host);
    const intent = trustedOrigin ? readCallbackIntent(state, request.cookies.get(callbackIntentCookie(state))?.value) : null;
    if (intent) { loginUrl.searchParams.set("returnTo", intent.destination); loginUrl.searchParams.set("mode", intent.mode); }
    return NextResponse.redirect(loginUrl);
  },
});

export async function GET(request: NextRequest) {
  const response = await authHandler(request);
  const state = request.nextUrl.searchParams.get("state");
  if (state && state.length <= 8192) {
    const result = new NextResponse(response.body, response);
    result.cookies.set(callbackIntentCookie(state), "", { path: "/auth", maxAge: 0, httpOnly: true, secure: process.env.NODE_ENV === "production", sameSite: "lax" });
    return result;
  }
  return response;
}
