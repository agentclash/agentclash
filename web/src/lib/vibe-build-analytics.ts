import { WEB_EVENTS, type WebEventPayloads } from "./analytics/events";
import { captureWebEvent, getPostHogSessionID } from "./analytics/posthog-client";

export type BuildEvent = Extract<(typeof WEB_EVENTS)[keyof typeof WEB_EVENTS], `web.vibe.build.${string}`>;
type BuildFields<E extends BuildEvent> = Omit<WebEventPayloads[E], "schema_version" | "door" | "viewport">;

const seen = new Set<string>();

export function captureBuildEvent<E extends BuildEvent>(event: E, fields: BuildFields<E>, once?: string) {
  if (typeof window === "undefined") return;
  if (once) {
    const identity = `${getPostHogSessionID() || "this-tab"}:${event}:${once}`;
    if (seen.has(identity)) return;
    try {
      const key = `vibe-build-event:${identity}`;
      if (window.sessionStorage.getItem(key)) return;
      window.sessionStorage.setItem(key, "1");
    } catch { /* Analytics storage must never block the product. */ }
    seen.add(identity);
  }
  captureWebEvent(event, {
    schema_version: 1,
    door: "build",
    viewport: window.innerWidth <= 600 ? "mobile" : window.innerWidth <= 900 ? "tablet" : "desktop",
    ...fields,
  } as WebEventPayloads[E]);
}
