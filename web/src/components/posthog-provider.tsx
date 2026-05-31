"use client";

/**
 * Mounts posthog-js on the client and captures App Router pageviews. Wraps
 * children unchanged when NEXT_PUBLIC_POSTHOG_KEY is unset — local dev, CI,
 * and self-hosted Next.js builds incur zero overhead.
 *
 * Identify happens in IdentifyOnSession below: any authenticated layout
 * should mount that component once.
 */

import { Suspense, useEffect, type ReactNode } from "react";
import { usePathname, useSearchParams } from "next/navigation";
import {
  capturePageView,
  initPostHog,
  isPostHogReady,
} from "@/lib/analytics/posthog-client";

const POSTHOG_KEY = process.env.NEXT_PUBLIC_POSTHOG_KEY ?? "";
// Default to the first-party reverse proxy (see next.config.ts rewrites).
const POSTHOG_HOST = process.env.NEXT_PUBLIC_POSTHOG_HOST ?? "/ingest";

export function PostHogProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    if (!POSTHOG_KEY) return;
    initPostHog({ apiKey: POSTHOG_KEY, apiHost: POSTHOG_HOST });
  }, []);

  return (
    <>
      <Suspense fallback={null}>
        <PostHogPageView />
      </Suspense>
      {children}
    </>
  );
}

function PostHogPageView() {
  const pathname = usePathname();
  const searchParams = useSearchParams();

  useEffect(() => {
    if (!isPostHogReady()) return;
    if (!pathname) return;
    let url = window.origin + pathname;
    const qs = searchParams?.toString();
    if (qs) url = `${url}?${qs}`;
    capturePageView(url);
  }, [pathname, searchParams]);

  return null;
}
