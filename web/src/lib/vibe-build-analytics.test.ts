import { beforeEach, expect, it, vi } from "vitest";
import { WEB_EVENTS } from "./analytics/events";

const capture = vi.hoisted(() => vi.fn());
vi.mock("./analytics/posthog-client", () => ({ captureWebEvent: capture, getPostHogSessionID: () => "analytics-session" }));

import { captureBuildEvent } from "./vibe-build-analytics";

beforeEach(() => { capture.mockClear(); window.sessionStorage.clear(); });

it("captures only typed, low-detail Build milestones and deduplicates viewed runs", () => {
  const fields = { session_id: "session-1", operation_id: "run-1", outcome: "failure" as const, passed: 2, failed: 1, unknown: 0 };
  captureBuildEvent(WEB_EVENTS.VIBE_BUILD_RESULT_VIEWED, fields, "session-1:run-1");
  captureBuildEvent(WEB_EVENTS.VIBE_BUILD_RESULT_VIEWED, fields, "session-1:run-1");
  expect(capture).toHaveBeenCalledTimes(1);
  expect(capture).toHaveBeenCalledWith(WEB_EVENTS.VIBE_BUILD_RESULT_VIEWED, expect.objectContaining({ schema_version: 1, door: "build", ...fields }));
  expect(JSON.stringify(capture.mock.calls)).not.toMatch(/prompt|reply|email|title|rule_text/);
});
