"use server";

import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import {
  buildDeviceReturnTo,
  normalizeDeviceUserCode,
} from "@/lib/auth/return-to";

function buildDeviceStatusPath(
  userCode: string,
  updates: Record<string, string>,
): string {
  const url = new URL(buildDeviceReturnTo(userCode), "http://agentclash.local");
  for (const [key, value] of Object.entries(updates)) {
    url.searchParams.set(key, value);
  }
  return `${url.pathname}${url.search}`;
}

function mapApproveError(error: unknown): string {
  if (error instanceof ApiError) {
    const message = error.message.toLowerCase();
    if (message.includes("expired")) {
      return "expired_code";
    }
    if (message.includes("required")) {
      return "missing_code";
    }
    if (message.includes("not found")) {
      return "invalid_code";
    }
    if (error.status === 401) {
      return "auth_required";
    }
  }

  return "request_failed";
}

export async function approveDeviceCodeAction(formData: FormData) {
  const userCode = normalizeDeviceUserCode(formData.get("user_code"));
  if (!userCode) {
    redirect(buildDeviceStatusPath("", { error: "missing_code" }));
  }

  const { user, accessToken } = await withAuth();
  if (!user || !accessToken) {
    const returnTo = buildDeviceReturnTo(userCode);
    redirect(`/auth/login?returnTo=${encodeURIComponent(returnTo)}`);
  }

  const api = createApiClient(accessToken);
  try {
    await api.post<{ ok: boolean }>("/v1/cli-auth/device/approve", {
      user_code: userCode,
    });
  } catch (error) {
    redirect(buildDeviceStatusPath(userCode, { error: mapApproveError(error) }));
  }

  redirect(buildDeviceStatusPath(userCode, { status: "approved" }));
}
