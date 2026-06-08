"use server";

import { getSignInUrl, getSignUpUrl } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { sanitizeReturnTo } from "@/lib/auth/return-to";

function resolveReturnTo(formData: FormData): string {
  const raw = formData.get("returnTo");
  return typeof raw === "string" ? sanitizeReturnTo(raw) : "/dashboard";
}

export async function signInAction(formData: FormData) {
  const url = await getSignInUrl({ returnTo: resolveReturnTo(formData) });
  redirect(url);
}

export async function signUpAction(formData: FormData) {
  const url = await getSignUpUrl({ returnTo: resolveReturnTo(formData) });
  redirect(url);
}
