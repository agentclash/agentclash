"use client";

/**
 * Calls posthog.identify(userId, traits) once after an authenticated layout
 * mounts, and fires web.auth.login.success exactly once per fresh tab
 * session. Intended to be rendered inside AuthenticatedAppProviders.
 *
 * The login-success guard uses sessionStorage so reloading the page in the
 * same tab does not re-fire the event, but opening a new tab (a fresh login
 * journey) does — that matches the funnel intent.
 */

import { useEffect } from "react";
import { captureWebEvent, identifyUser } from "@/lib/analytics/posthog-client";
import { WEB_EVENTS } from "@/lib/analytics/events";
import type { SessionResponse } from "@/lib/api/types";

const LOGIN_FIRED_PREFIX = "posthog:login-fired:";

export function PostHogIdentify({ session }: { session: SessionResponse }) {
  useEffect(() => {
    if (!session?.user_id) return;
    identifyUser(session.user_id, {
      email: session.email,
      display_name: session.display_name,
      org_ids: session.organization_memberships.map((m) => m.organization_id),
      workspace_ids: session.workspace_memberships.map((m) => m.workspace_id),
    });
    if (typeof window === "undefined") return;
    const flag = `${LOGIN_FIRED_PREFIX}${session.user_id}`;
    if (window.sessionStorage.getItem(flag)) return;
    captureWebEvent(WEB_EVENTS.AUTH_LOGIN_SUCCESS, { user_id: session.user_id });
    window.sessionStorage.setItem(flag, "1");
  }, [session]);
  return null;
}
