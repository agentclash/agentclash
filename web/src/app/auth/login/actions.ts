"use server";

import { getSignInUrl, getSignUpUrl } from "@workos-inc/authkit-nextjs";
import { cookies } from "next/headers";
import { callbackIntentCookie, callbackIntentTTL, signCallbackIntent } from "@/lib/auth/callback-intent";
import { redirect } from "next/navigation";
import { sanitizeReturnTo } from "@/lib/auth/return-to";

function resolveReturnTo(formData: FormData): string {
  const raw = formData.get("returnTo");
  return typeof raw === "string" ? sanitizeReturnTo(raw) : "/dashboard";
}

async function rememberIntent(url: string, destination: string, mode: "signin" | "signup") {
  const state = new URL(url).searchParams.get("state");
  if (!state || state.length > 8192) throw new Error("Invalid authorization state");
  const store = await cookies();
  // Bound concurrent attempts, retaining the newest independent flows.
  const previous = store.getAll().filter(cookie => cookie.name.startsWith("ac-auth-intent-"));
  for (const cookie of previous.slice(0, Math.max(0, previous.length - 3))) store.set(cookie.name, "", { path: "/auth", maxAge: 0, httpOnly: true, sameSite: "lax", secure: process.env.NODE_ENV === "production" });
  store.set(callbackIntentCookie(state), signCallbackIntent(state, destination, mode), {
    httpOnly: true, secure: process.env.NODE_ENV === "production", sameSite: "lax", path: "/auth", maxAge: callbackIntentTTL,
  });
}

export async function signInAction(formData: FormData) {
  const url = await getSignInUrl({ returnTo: resolveReturnTo(formData) });
  await rememberIntent(url, resolveReturnTo(formData), "signin");
  redirect(url);
}

export async function signUpAction(formData: FormData) {
  const url = await getSignUpUrl({ returnTo: resolveReturnTo(formData) });
  await rememberIntent(url, resolveReturnTo(formData), "signup");
  redirect(url);
}
