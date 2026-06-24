import { describe, expect, it } from "vitest";
import { assertAgent, Contains, evaluate } from "../src/index.js";

describe("evals spike", () => {
  it("passes contains metric", async () => {
    const report = await evaluate("hello world", [new Contains("world")]);
    expect(report.exit_code).toBe(0);
  });

  it("assertAgent throws on failure", async () => {
    await expect(assertAgent("hello", [new Contains("world")])).rejects.toThrow();
  });
});
