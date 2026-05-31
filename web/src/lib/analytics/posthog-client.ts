/**
 * Browser PostHog wrapper. Initialised once by PostHogProvider; every other
 * surface calls `captureWebEvent` / `identifyUser` / `resetPostHog` from this
 * module so the rest of the app never imports posthog-js directly.
 *
 * When NEXT_PUBLIC_POSTHOG_KEY is unset (local dev, CI, self-hosted builds)
 * the helpers are no-ops — matching the optional-service posture taken by the
 * backend.
 */

import posthog from "posthog-js";
import type { WebEventName, WebEventPayloads } from "./events";

let initialized = false;

export interface InitPostHogOptions {
  apiKey: string;
  apiHost: string;
}

export function initPostHog({ apiKey, apiHost }: InitPostHogOptions): boolean {
  if (initialized) return true;
  if (!apiKey) return false;
  if (typeof window === "undefined") return false;
  posthog.init(apiKey, {
    // api_host points at our first-party reverse proxy ("/ingest" by default,
    // see next.config.ts) so ad-blockers don't drop events. ui_host is the
    // real PostHog app so in-app links (e.g. toolbar) resolve correctly.
    api_host: apiHost || "/ingest",
    ui_host: "https://us.posthog.com",
    // Pageviews are captured manually by <PostHogPageView /> so the Next.js
    // App Router's client-side transitions are tracked correctly.
    capture_pageview: false,
    capture_pageleave: true,
    // Only create person profiles for identified users to keep MAU under the
    // free-tier ceiling. Anonymous events still land.
    person_profiles: "identified_only",
    autocapture: false,
    disable_session_recording: true,
  });
  initialized = true;
  return true;
}

export function isPostHogReady(): boolean {
  return initialized;
}

export function captureWebEvent<E extends WebEventName>(
  event: E,
  properties: WebEventPayloads[E],
): void {
  if (!initialized) return;
  posthog.capture(event, properties as Record<string, unknown>);
}

export function capturePageView(url: string): void {
  if (!initialized) return;
  posthog.capture("$pageview", { $current_url: url });
}

export interface IdentifyTraits {
  email?: string;
  display_name?: string;
  org_ids?: string[];
  workspace_ids?: string[];
}

export function identifyUser(userId: string, traits: IdentifyTraits): void {
  if (!initialized) return;
  if (!userId) return;
  posthog.identify(userId, traits as Record<string, unknown>);
}

export function resetPostHog(): void {
  if (!initialized) return;
  posthog.reset();
}
