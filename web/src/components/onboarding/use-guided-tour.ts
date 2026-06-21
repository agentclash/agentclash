"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";
import { driver, type Config, type DriveStep, type Driver } from "driver.js";
import "driver.js/dist/driver.css";

// Page-agnostic guided-tour hook built on driver.js. Each tour is keyed by a
// stable `tourId`; the "already seen" flag persists in localStorage (mirroring
// the dismissal pattern in activation-banner.tsx) so a tour auto-runs only once
// but can be re-launched from a button at any time. Reusable across features —
// it knows nothing about tools or the canvas.

const seenKey = (tourId: string) => `agentclash:tour-seen:${tourId}`;

function readSeen(tourId: string): boolean {
  try {
    return window.localStorage.getItem(seenKey(tourId)) === "1";
  } catch {
    return false; // private mode / storage disabled
  }
}

function markSeen(tourId: string): void {
  try {
    window.localStorage.setItem(seenKey(tourId), "1");
  } catch {
    // ignore
  }
}

export interface GuidedTourOptions {
  /** Auto-run the tour the first time this user sees it (persisted per tourId). */
  autoStartOnce?: boolean;
  /** Gate auto-start until anchor elements exist (e.g. async data has loaded). */
  ready?: boolean;
  /** Extra driver.js config overrides. */
  config?: Partial<Config>;
}

export interface GuidedTour {
  /** Start (or restart) the tour now. Safe to call repeatedly. */
  start: () => void;
  /** Whether this tour has been shown before. */
  hasSeen: () => boolean;
  /** True while the tour overlay is on screen — gate Esc/shortcuts on this. */
  isActive: () => boolean;
}

export function useGuidedTour(
  tourId: string,
  steps: DriveStep[],
  { autoStartOnce = false, ready = true, config }: GuidedTourOptions = {},
): GuidedTour {
  const driverRef = useRef<Driver | null>(null);
  // Hold the latest steps/config in refs so `start` stays referentially stable
  // even when a caller passes inline arrays/objects. An unstable `start` would
  // otherwise re-run the auto-start effect on every render.
  const stepsRef = useRef(steps);
  const configRef = useRef(config);
  useEffect(() => {
    stepsRef.current = steps;
    configRef.current = config;
  });

  const start = useCallback(() => {
    if (typeof window === "undefined") return;
    const userConfig = configRef.current;
    // Recreate the driver each launch so it always reflects the latest steps,
    // and so re-running after a previous tour starts cleanly from step one.
    driverRef.current?.destroy();
    const d = driver({
      showProgress: true,
      allowClose: true,
      overlayColor: "rgba(2, 6, 23, 0.72)",
      stagePadding: 6,
      stageRadius: 10,
      popoverClass: "agentclash-tour",
      nextBtnText: "Next",
      prevBtnText: "Back",
      doneBtnText: "Got it",
      ...userConfig,
      steps: stepsRef.current,
      // Mark "seen" only once the tour actually ends (completed or dismissed) —
      // not the instant it launches. A tour that never renders (e.g. a missing
      // anchor) then won't suppress the next visit's auto-start.
      onDestroyed: (element, step, opts) => {
        markSeen(tourId);
        userConfig?.onDestroyed?.(element, step, opts);
      },
    });
    driverRef.current = d;
    d.drive();
  }, [tourId]);

  // Auto-start once, after anchors are ready.
  useEffect(() => {
    if (!autoStartOnce || !ready) return;
    if (typeof window === "undefined" || readSeen(tourId)) return;
    // Defer a frame so anchor elements are mounted and measurable.
    const t = window.setTimeout(start, 400);
    return () => window.clearTimeout(t);
  }, [autoStartOnce, ready, tourId, start]);

  // Tear down the driver instance (and any open overlay) on unmount.
  useEffect(() => {
    return () => {
      driverRef.current?.destroy();
      driverRef.current = null;
    };
  }, []);

  const hasSeen = useCallback(
    () => (typeof window === "undefined" ? false : readSeen(tourId)),
    [tourId],
  );
  const isActive = useCallback(() => driverRef.current?.isActive() ?? false, []);

  // Stable object so consumers can safely use it in effect dependency arrays.
  return useMemo(
    () => ({ start, hasSeen, isActive }),
    [start, hasSeen, isActive],
  );
}
