"use server";

import { getSignInUrl } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";

export async function signInAction() {
  const url = await getSignInUrl({ returnTo: "/dashboard" });
  redirect(url);
}
