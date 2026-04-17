"use server";

import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import {
  buildDeviceReturnTo,
  normalizeDeviceUserCode,
} from "@/lib/auth/return-to";
import { buildDeviceStatusPath, mapDeviceActionError } from "./status";

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
    redirect(buildDeviceStatusPath(userCode, { error: mapDeviceActionError(error) }));
  }

  redirect(buildDeviceStatusPath(userCode, { status: "approved" }));
}

export async function denyDeviceCodeAction(formData: FormData) {
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
    await api.post<{ ok: boolean }>("/v1/cli-auth/device/deny", {
      user_code: userCode,
    });
  } catch (error) {
    redirect(buildDeviceStatusPath(userCode, { error: mapDeviceActionError(error) }));
  }

  redirect(buildDeviceStatusPath(userCode, { status: "denied" }));
}
