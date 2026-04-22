"use client";

import { useCallback, useSyncExternalStore } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

export type ArenaMode = "dev" | "race";

const STORAGE_KEY = "agentclash:arena-mode";
const STORAGE_EVENT = "agentclash:arena-mode:changed";

function isArenaMode(v: unknown): v is ArenaMode {
  return v === "dev" || v === "race";
}

function readStoredMode(): ArenaMode | null {
  if (typeof window === "undefined") return null;
  try {
    const v = window.localStorage.getItem(STORAGE_KEY);
    return isArenaMode(v) ? v : null;
  } catch {
    return null;
  }
}

function subscribeStorage(cb: () => void): () => void {
  if (typeof window === "undefined") return () => {};
  // Cross-tab updates (real storage event) AND same-tab (our custom event).
  window.addEventListener("storage", cb);
  window.addEventListener(STORAGE_EVENT, cb);
  return () => {
    window.removeEventListener("storage", cb);
    window.removeEventListener(STORAGE_EVENT, cb);
  };
}

/**
 * Resolves arena mode with URL > localStorage > "dev" precedence.
 *
 * URL param (?mode=race) is the shareable source of truth; localStorage
 * carries the user's preference across sessions. Setting the mode updates
 * both — and dispatches a same-tab event so peer consumers of this hook
 * re-render.
 */
export function useArenaMode(): [ArenaMode, (next: ArenaMode) => void] {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();

  const storedMode = useSyncExternalStore(
    subscribeStorage,
    readStoredMode,
    () => null,
  );

  const urlMode = searchParams.get("mode");
  const mode: ArenaMode = isArenaMode(urlMode)
    ? urlMode
    : (storedMode ?? "dev");

  const setMode = useCallback(
    (next: ArenaMode) => {
      try {
        window.localStorage.setItem(STORAGE_KEY, next);
        window.dispatchEvent(new Event(STORAGE_EVENT));
      } catch {
        // localStorage unavailable — URL is still the source of truth.
      }
      const params = new URLSearchParams(searchParams.toString());
      if (next === "race") params.set("mode", "race");
      else params.delete("mode");
      const query = params.toString();
      router.replace(query ? `${pathname}?${query}` : pathname, {
        scroll: false,
      });
    },
    [pathname, router, searchParams],
  );

  return [mode, setMode];
}
