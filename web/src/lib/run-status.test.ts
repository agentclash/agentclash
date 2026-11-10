import { describe, expect, it } from "vitest";

import {
  isAgentLive,
  isAgentTerminal,
  isRunLive,
  isRunTerminal,
} from "./run-status";

describe("isRunLive", () => {
  it("is live while the run is active", () => {
    expect(isRunLive("running", [])).toBe(true);
    expect(isRunLive("scoring", ["completed"])).toBe(true);
  });

  it("is live while a non-terminal run still has active agents", () => {
    expect(isRunLive("draft", ["queued"])).toBe(true);
  });

  it("is not live for a terminal run, even with agents left executing", () => {
    expect(isRunLive("cancelled", ["executing", "queued"])).toBe(false);
    expect(isRunLive("failed", ["evaluating"])).toBe(false);
    expect(isRunLive("completed", ["ready"])).toBe(false);
  });

  it("is not live for a draft run with no active agents", () => {
    expect(isRunLive("draft", [])).toBe(false);
  });
});

describe("isAgentLive", () => {
  it("follows the agent status while the run is not terminal", () => {
    expect(isAgentLive("executing", "running")).toBe(true);
    expect(isAgentLive("completed", "running")).toBe(false);
  });

  it("is never live when the run is terminal", () => {
    expect(isAgentLive("executing", "cancelled")).toBe(false);
    expect(isAgentLive("queued", "failed")).toBe(false);
  });
});

describe("terminal helpers", () => {
  it("knows the terminal run statuses", () => {
    expect(isRunTerminal("completed")).toBe(true);
    expect(isRunTerminal("failed")).toBe(true);
    expect(isRunTerminal("cancelled")).toBe(true);
    expect(isRunTerminal("scoring")).toBe(false);
    expect(isRunTerminal("draft")).toBe(false);
  });

  it("knows the terminal agent statuses", () => {
    expect(isAgentTerminal("completed")).toBe(true);
    expect(isAgentTerminal("failed")).toBe(true);
    expect(isAgentTerminal("evaluating")).toBe(false);
  });
});
