"use client";

import { useSyncExternalStore } from "react";
import Link from "next/link";
import { ArrowRight, X } from "lucide-react";
import { captureWebEvent } from "@/lib/analytics/posthog-client";
import { WEB_EVENTS } from "@/lib/analytics/events";

const DISMISS_KEY = "ac-agent-promo-dismissed";

const listeners = new Set<() => void>();

function subscribe(cb: () => void) {
  listeners.add(cb);
  window.addEventListener("storage", cb);
  return () => {
    listeners.delete(cb);
    window.removeEventListener("storage", cb);
  };
}

function emit() {
  for (const cb of listeners) cb();
}

function getSnapshot() {
  return localStorage.getItem(DISMISS_KEY) === "1";
}

// Hidden during SSR; revealed on the client unless previously dismissed. Using
// useSyncExternalStore keeps server/client renders consistent (no mismatch).
function getServerSnapshot() {
  return true;
}

type Offer = {
  offer: string;
  href: string;
  label: string;
  cta: string;
};

const OFFERS: Offer[] = [
  {
    offer: "agent_opportunity",
    href: "/agent-opportunity",
    label: "Should you build an AI agent?",
    cta: "Free ROI report",
  },
  {
    offer: "tryout",
    href: "/tryouts",
    label: "Try an agent on your own workflow",
    cta: "Start free",
  },
];

type Props = {
  /** Page surface this banner renders on, used for analytics. */
  page: string;
};

export function AgentPromoBanner({ page }: Props) {
  const dismissed = useSyncExternalStore(
    subscribe,
    getSnapshot,
    getServerSnapshot,
  );

  if (dismissed) return null;

  const dismiss = () => {
    localStorage.setItem(DISMISS_KEY, "1");
    emit();
  };

  return (
    <div className="relative border-b border-white/[0.06] bg-white/[0.02]">
      <div className="mx-auto flex max-w-[1440px] flex-wrap items-center justify-center gap-x-6 gap-y-1.5 px-12 sm:px-14 py-2.5 text-xs sm:text-sm">
        {OFFERS.map((offer, index) => (
          <div key={offer.offer} className="flex items-center gap-x-6">
            {index > 0 && (
              <span
                aria-hidden
                className="hidden h-3.5 w-px bg-white/10 sm:inline-block"
              />
            )}
            <Link
              href={offer.href}
              onClick={() =>
                captureWebEvent(WEB_EVENTS.PROMO_BANNER_CLICKED, {
                  offer: offer.offer,
                  destination: offer.href,
                  page,
                })
              }
              className="group inline-flex items-center gap-2"
            >
              <span className="text-white/65 transition-colors group-hover:text-white/85">
                {offer.label}
              </span>
              <span className="inline-flex items-center gap-1 font-medium text-white/85 transition-colors group-hover:text-white">
                {offer.cta}
                <ArrowRight className="size-3.5 transition-transform group-hover:translate-x-0.5" />
              </span>
            </Link>
          </div>
        ))}
      </div>
      <button
        type="button"
        onClick={dismiss}
        aria-label="Dismiss"
        className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded p-1 text-white/30 transition-colors hover:text-white/70"
      >
        <X className="size-3.5" />
      </button>
    </div>
  );
}
