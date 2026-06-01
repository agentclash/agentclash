import { describe, expect, it } from "vitest";
import { HOME_FAQ } from "./home-faq";

describe("HOME_FAQ", () => {
  it("provides several non-empty, answer-shaped Q&A entries", () => {
    expect(HOME_FAQ.length).toBeGreaterThanOrEqual(5);
    for (const entry of HOME_FAQ) {
      expect(entry.question.trim().length).toBeGreaterThan(0);
      expect(entry.question.trim().endsWith("?")).toBe(true);
      expect(entry.answer.trim().length).toBeGreaterThan(40);
    }
  });

  it("covers the agent-eval-vs-prompt-eval positioning", () => {
    const joined = HOME_FAQ.map((entry) => entry.question).join(" ");
    expect(joined).toContain("What is AgentClash?");
    expect(joined.toLowerCase()).toContain("prompt-evaluation");
  });
});
