import { NextRequest, NextResponse } from "next/server";
import { getIronSession } from "iron-session";
import { getSessionOptions } from "@/lib/auth/session";
import type { SessionData } from "@/lib/auth/session";

/**
 * GET /auth/sign-out
 *
 * Destroys the session cookie and redirects to the landing page.
 * Uses getIronSession with response.cookies (same reason as callback).
 */
export async function GET(request: NextRequest) {
  const response = NextResponse.redirect(new URL("/", request.url));
  const session = await getIronSession<{ data?: SessionData }>(
    response.cookies,
    getSessionOptions(),
  );
  session.destroy();
  return response;
}
