"use server";

import { getSignInUrl } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { sanitizeReturnTo } from "@/lib/auth/return-to";

export async function signInAction(formData: FormData) {
  const rawReturnTo = formData.get("returnTo");
  const returnTo =
    typeof rawReturnTo === "string"
      ? sanitizeReturnTo(rawReturnTo)
      : "/dashboard";

  const url = await getSignInUrl({ returnTo });
  redirect(url);
}
