import { handleAuth } from "@workos-inc/authkit-nextjs";
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";

const AUTH_CALLBACK_ERROR = {
  error: {
    message: "Something went wrong",
    description:
      "Couldn't sign in. If you are not sure what happened, please contact your organization admin.",
  },
} as const;

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
    return NextResponse.json(AUTH_CALLBACK_ERROR, { status: 500 });
  },
});
