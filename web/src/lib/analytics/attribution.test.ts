import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  attributionSetOnce,
  attributionStorageKeyForTests,
  recordFirstTouch,
  rememberAcquisitionCTA,
} from "./attribution";

describe("first-touch attribution", () => {
  beforeEach(() => {
    window.localStorage.clear();
    window.history.replaceState(
      {},
      "",
      "/pricing?utm_source=google&utm_campaign=launch&utm_content=private%40example.com&email=private%40example.com",
    );
    Object.defineProperty(document, "referrer", {
      configurable: true,
      value: "https://search.example/private?q=secret",
    });
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-08-20T00:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("persists first touch and the latest acquisition CTA without PII", () => {
    recordFirstTouch();
    rememberAcquisitionCTA("pricing.closing.start_free");
    window.history.replaceState({}, "", "/enterprise?utm_source=other");
    recordFirstTouch();

    expect(attributionSetOnce()).toEqual({
      acquisition_entry_path: "/pricing",
      acquisition_channel: "unknown",
      acquisition_referrer_hostname: "search.example",
      acquisition_utm_source: "google",
      acquisition_utm_campaign: "launch",
      acquisition_cta_id: "pricing.closing.start_free",
    });
    expect(window.localStorage.getItem(attributionStorageKeyForTests)).not.toContain(
      "private@example.com",
    );
  });

  it("expires first touch and CTA after 30 days", () => {
    recordFirstTouch();
    rememberAcquisitionCTA("pricing.closing.start_free");
    vi.advanceTimersByTime(31 * 24 * 60 * 60 * 1_000);

    expect(attributionSetOnce()).toEqual({});
  });
});

it.each(["claude", "codex"])("retains original %s channel across auth and later visits", (host) => {
  window.localStorage.clear();
  window.history.replaceState({}, "", `/tryouts?utm_source=${host}`);
  recordFirstTouch();
  window.history.replaceState({}, "", "/auth/callback?utm_source=other");
  recordFirstTouch();
  expect(attributionSetOnce().acquisition_channel).toBe(host);
});

it("sanitizes tampered storage before identifying a person", () => {
  window.localStorage.setItem(attributionStorageKeyForTests, JSON.stringify({
    first_touch: { entry_path: "/auth/callback?token=private", expires_at: Date.now() + 10000,
      channel: "claude", referrer_hostname: "https://secret.invalid/path?token=secret",
      utm: { utm_source: "codex", utm_campaign: "private@example.com", secret_token: "private" } },
    acquisition_cta: { cta_id: { secret: "private" }, expires_at: Date.now() + 10000 },
  }));
  expect(attributionSetOnce()).toEqual({ acquisition_entry_path: "/auth/callback", acquisition_channel: "codex", acquisition_utm_source: "codex" });
});

it("does not turn an original unknown touch into a later tagged acquisition", () => {
  window.localStorage.clear();
  window.history.replaceState({}, "", "/pricing");
  recordFirstTouch();
  window.history.replaceState({}, "", "/onboarding?utm_source=codex");
  recordFirstTouch();
  expect(attributionSetOnce().acquisition_channel).toBe("unknown");
});

it("drops unsafe or unbounded persisted path values", () => {
  for (const entry_path of ["/users/private@example.com", "/users/private%40example.com", "/" + "x".repeat(600)]) {
    window.localStorage.setItem(attributionStorageKeyForTests, JSON.stringify({ first_touch: { entry_path, expires_at: Date.now() + 10000 } }));
    expect(attributionSetOnce().acquisition_entry_path).toBe("/");
  }
});
