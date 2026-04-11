import { getIronSession, sealData, unsealData, type SessionOptions } from "iron-session";
import { cookies } from "next/headers";
import type { ResponseCookies } from "next/dist/compiled/@edge-runtime/cookies";
import { getSessionSecret } from "./config";

/**
 * Session payload stored in the encrypted cookie.
 * Discriminated union on `mode` so TypeScript can narrow the type.
 */
export type SessionData =
  | WorkOSSessionData
  | DevSessionData;

export interface WorkOSSessionData {
  mode: "workos";
  accessToken: string;
  refreshToken: string;
  expiresAt: number; // Unix timestamp (seconds)
}

export interface DevSessionData {
  mode: "dev";
  userId: string;
  email: string;
  displayName: string;
  orgMemberships: string;       // "uuid:role,uuid:role"
  workspaceMemberships: string; // "uuid:role,uuid:role"
}

const COOKIE_NAME = "agentclash_session";
const SESSION_TTL = 60 * 60 * 8; // 8 hours

export function getSessionOptions(): SessionOptions {
  return {
    password: getSessionSecret(),
    cookieName: COOKIE_NAME,
    ttl: SESSION_TTL,
    cookieOptions: {
      secure: process.env.NODE_ENV === "production",
      httpOnly: true,
      sameSite: "lax" as const,
      path: "/",
    },
  };
}

/**
 * Read the current session from the request cookies.
 * Returns the session data, or null if no session exists.
 */
export async function getSession(): Promise<SessionData | null> {
  const cookieStore = await cookies();
  const session = await getIronSession<{ data?: SessionData }>(
    cookieStore,
    getSessionOptions(),
  );
  return session.data ?? null;
}

/**
 * Create a dev session from the login form.
 * Works in Server Actions where `cookies()` has write access.
 */
export async function createDevSession(input: {
  userId: string;
  email: string;
  displayName: string;
  orgMemberships: string;
  workspaceMemberships: string;
}): Promise<void> {
  const cookieStore = await cookies();
  const session = await getIronSession<{ data?: SessionData }>(
    cookieStore,
    getSessionOptions(),
  );
  session.data = {
    mode: "dev",
    ...input,
  };
  await session.save();
}

/**
 * Create a WorkOS session via the cookies() API.
 * Works in Server Components and Server Actions where cookies() has
 * write access. For Route Handlers, use setSessionCookie() instead.
 */
export async function createWorkOSSession(
  accessToken: string,
  refreshToken: string,
  expiresInSeconds: number,
): Promise<void> {
  const cookieStore = await cookies();
  const session = await getIronSession<{ data?: SessionData }>(
    cookieStore,
    getSessionOptions(),
  );
  session.data = {
    mode: "workos",
    accessToken,
    refreshToken,
    expiresAt: Math.floor(Date.now() / 1000) + expiresInSeconds,
  };
  await session.save();
}

/**
 * Seal session data into an encrypted string and set it as a cookie
 * on a NextResponse. Use this in Route Handlers where `cookies()`
 * from next/headers does not have write access (e.g. redirect responses).
 */
export async function setSessionCookie(
  responseCookies: ResponseCookies,
  data: SessionData,
): Promise<void> {
  const password = getSessionSecret();
  const sealed = await sealData({ data }, { password, ttl: SESSION_TTL });
  responseCookies.set(COOKIE_NAME, sealed, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: SESSION_TTL,
  });
}

/**
 * Delete the session cookie on a NextResponse.
 * Use this in Route Handlers (e.g. sign-out).
 */
export function deleteSessionCookie(
  responseCookies: ResponseCookies,
): void {
  responseCookies.delete(COOKIE_NAME);
}

/**
 * Destroy the current session via the cookies() API.
 * Works in Server Actions and server component contexts.
 */
export async function destroySession(): Promise<void> {
  const cookieStore = await cookies();
  const session = await getIronSession<{ data?: SessionData }>(
    cookieStore,
    getSessionOptions(),
  );
  session.destroy();
}

/**
 * Unseal a raw cookie value back to session data.
 * Used when reading session outside of iron-session's getIronSession
 * (e.g. after setting it via sealData in the same request cycle).
 */
export async function unsealSessionData(sealed: string): Promise<SessionData | null> {
  try {
    const password = getSessionSecret();
    const result = await unsealData<{ data?: SessionData }>(sealed, { password });
    return result.data ?? null;
  } catch {
    return null;
  }
}

/**
 * Name of the session cookie. Used by proxy for existence checks
 * without decrypting (proxy runs on Edge where iron-session
 * decryption may not be available).
 */
export const SESSION_COOKIE_NAME = COOKIE_NAME;
