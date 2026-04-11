import { NextRequest, NextResponse } from "next/server";
import { deleteSessionCookie } from "@/lib/auth/session";

/**
 * GET /auth/sign-out
 *
 * Destroys the session cookie and redirects to the landing page.
 */
export async function GET(request: NextRequest) {
  const response = NextResponse.redirect(new URL("/", request.url));
  deleteSessionCookie(response.cookies);
  return response;
}
