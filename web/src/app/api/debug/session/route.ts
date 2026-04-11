import { NextResponse } from "next/server";
import { getSession } from "@/lib/auth/session";
import { cookies } from "next/headers";
import { SESSION_COOKIE_NAME } from "@/lib/auth/session";

/**
 * DEBUG: shows raw session state visible to server-side code.
 */
export async function GET() {
  const cookieStore = await cookies();
  const rawCookie = cookieStore.get(SESSION_COOKIE_NAME);

  const session = await getSession();

  return NextResponse.json({
    hasCookie: !!rawCookie,
    cookieValueLength: rawCookie?.value?.length ?? 0,
    cookieValuePrefix: rawCookie?.value?.substring(0, 30) ?? null,
    session,
  });
}
