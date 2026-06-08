import { cookies } from "next/headers";

/**
 * First-party hint cookie marking a browser that has completed at least one
 * successful login. It is NOT auth — WorkOS still fully enforces sessions — it
 * only lets logged-out marketing surfaces choose between a "Sign in" affordance
 * (returning visitor) and a "Sign up" affordance (likely new visitor).
 *
 * Set on the auth callback's onSuccess (see app/auth/callback/route.ts).
 */
export const RETURNING_COOKIE = "ac_returning";

/** Read the returning-visitor hint. Server-only (reads the cookie store). */
export async function isReturningVisitor(): Promise<boolean> {
  return (await cookies()).get(RETURNING_COOKIE)?.value === "1";
}
