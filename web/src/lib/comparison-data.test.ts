import { describe, expect, it } from "vitest";
import {
  AGENTCLASH_COLUMN_INDEX,
  COMPARISON_COLUMNS,
  COMPARISON_ROWS,
  COMPETITORS,
  competitorFaq,
  competitorRows,
  competitorSlug,
  getCompetitorBySlug,
} from "./comparison-data";

describe("comparison data", () => {
  it("puts AgentClash first and highlighted", () => {
    expect(AGENTCLASH_COLUMN_INDEX).toBe(0);
    expect(COMPARISON_COLUMNS[0]).toMatchObject({
      name: "AgentClash",
      highlight: true,
    });
  });

  it("keeps every row's cell count aligned with the columns", () => {
    for (const row of COMPARISON_ROWS) {
      expect(row.cells).toHaveLength(COMPARISON_COLUMNS.length);
    }
  });

  it("derives one competitor per non-AgentClash column", () => {
    expect(COMPETITORS).toHaveLength(COMPARISON_COLUMNS.length - 1);
    for (const competitor of COMPETITORS) {
      expect(COMPARISON_COLUMNS[competitor.columnIndex].highlight).toBeFalsy();
      expect(competitor.whereItFits.length).toBeGreaterThan(0);
    }
  });

  it("builds vs-style slugs and resolves them back", () => {
    expect(competitorSlug("LangSmith")).toBe("agentclash-vs-langsmith");
    expect(competitorSlug("Arize Phoenix")).toBe("agentclash-vs-arize-phoenix");
    expect(competitorSlug("OpenAI Evals")).toBe("agentclash-vs-openai-evals");

    const langsmith = getCompetitorBySlug("agentclash-vs-langsmith");
    expect(langsmith?.name).toBe("LangSmith");
    expect(getCompetitorBySlug("does-not-exist")).toBeUndefined();
  });

  it("pairs AgentClash and competitor verdicts per row", () => {
    const langsmith = getCompetitorBySlug("agentclash-vs-langsmith");
    expect(langsmith).toBeDefined();
    const rows = competitorRows(langsmith!);

    expect(rows).toHaveLength(COMPARISON_ROWS.length);
    expect(rows[0]).toMatchObject({
      label: "Multi-turn agent loops",
      agentclash: "yes",
    });
    expect(rows.every((row) => ["yes", "partial", "no"].includes(row.competitor))).toBe(
      true,
    );
  });

  it("generates three answer-shaped FAQ entries per competitor", () => {
    const langsmith = getCompetitorBySlug("agentclash-vs-langsmith")!;
    const faq = competitorFaq(langsmith);

    expect(faq).toHaveLength(3);
    expect(faq[0].question).toContain("LangSmith");
    for (const entry of faq) {
      expect(entry.answer.length).toBeGreaterThan(0);
    }
  });
});
