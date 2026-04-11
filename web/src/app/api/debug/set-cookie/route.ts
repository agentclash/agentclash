import { NextResponse } from "next/server";
import { sealSessionToResponse } from "@/lib/auth/session";

/**
 * DEBUG ONLY — tests cookie setting on a redirect response.
 * Visit /api/debug/set-cookie then check if the cookie was set.
 */
export async function GET() {
  const response = NextResponse.redirect(new URL("/api/debug/read-cookie", "http://localhost:3000"));
  await sealSessionToResponse(response, {
    mode: "dev",
    userId: "test-123",
    email: "debug@test.com",
    displayName: "Debug User",
    orgMemberships: "",
    workspaceMemberships: "",
  });

  // Log what headers are being sent
  const setCookie = response.headers.get("set-cookie");
  console.log("[DEBUG] Set-Cookie header present:", !!setCookie);
  console.log("[DEBUG] Set-Cookie length:", setCookie?.length);
  console.log("[DEBUG] Set-Cookie first 200 chars:", setCookie?.substring(0, 200));

  return response;
}
