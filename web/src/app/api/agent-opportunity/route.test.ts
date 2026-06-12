import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const report = {
  analyzedUrl: "http://93.184.216.34/",
  companyName: "Example",
  generatedAt: "2026-06-12T00:00:00.000Z",
  agentFitScore: 68,
  fitLevel: "moderate",
  shouldBuildAgent: "narrow_pilot",
  honestVerdict: "A narrow support workflow is worth testing first.",
  summary: "The site suggests repeatable customer education work.",
  useCases: [
    {
      title: "Support triage",
      workflow: "Classify inbound requests and draft replies.",
      fit: "high",
      estimatedMonthlyHoursSaved: "15-35",
      estimatedMonthlySavingsUsd: "$1,000-$3,500",
      complexity: "medium",
      why: "The homepage describes support-heavy customer workflows.",
      firstEvalTasks: ["Pricing question", "Refund escalation"],
    },
  ],
  risks: [
    {
      risk: "Wrong policy answer",
      severity: "high",
      mitigation: "Evaluate policy edge cases before deployment.",
    },
    {
      risk: "Low-confidence routing",
      severity: "medium",
      mitigation: "Escalate uncertain turns to a human.",
    },
  ],
  evaluationPack: {
    name: "Support triage starter pack",
    recommendedCases: 25,
    adversarialCases: 8,
    successCriteria: ["No policy hallucinations", "90% correct routing"],
  },
  nextSteps: ["Collect real tickets", "Run a same-task AgentClash race"],
  evidenceLimitations: ["Only public homepage content was available."],
};

describe("POST /api/agent-opportunity", () => {
  const originalOpenAIKey = process.env.OPENAI_API_KEY;
  const fetchMock = vi.fn();

  beforeEach(() => {
    vi.resetModules();
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    if (originalOpenAIKey === undefined) delete process.env.OPENAI_API_KEY;
    else process.env.OPENAI_API_KEY = originalOpenAIKey;
    vi.unstubAllGlobals();
  });

  it("returns 400 for invalid URLs before fetching", async () => {
    process.env.OPENAI_API_KEY = "sk-test";
    const { POST } = await import("./route");

    const response = await POST(
      new Request("https://agentclash.dev/api/agent-opportunity", {
        method: "POST",
        body: JSON.stringify({ url: "file:///etc/passwd" }),
      }),
    );

    expect(response.status).toBe(400);
    expect(fetchMock).not.toHaveBeenCalled();
    await expect(response.json()).resolves.toMatchObject({
      ok: false,
      code: "invalid_url",
    });
  });

  it("returns 503 when OpenAI is not configured", async () => {
    delete process.env.OPENAI_API_KEY;
    const { POST } = await import("./route");

    const response = await POST(
      new Request("https://agentclash.dev/api/agent-opportunity", {
        method: "POST",
        body: JSON.stringify({ url: "http://93.184.216.34" }),
      }),
    );

    expect(response.status).toBe(503);
    expect(fetchMock).not.toHaveBeenCalled();
    await expect(response.json()).resolves.toMatchObject({
      ok: false,
      code: "openai_not_configured",
    });
  });

  it("returns a complete report with mocked page and OpenAI fetches", async () => {
    process.env.OPENAI_API_KEY = "sk-test";
    fetchMock
      .mockResolvedValueOnce(
        new Response(
          "<html><head><title>Example</title></head><body>Support automation for customer teams.</body></html>",
          { headers: { "content-type": "text/html" } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ output_text: JSON.stringify(report) }), {
          headers: { "content-type": "application/json" },
        }),
      );
    const { POST } = await import("./route");

    const response = await POST(
      new Request("https://agentclash.dev/api/agent-opportunity", {
        method: "POST",
        body: JSON.stringify({
          url: "http://93.184.216.34",
          companySize: "11-50",
          currentPain: "support",
        }),
      }),
    );

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    const openAIRequest = fetchMock.mock.calls[1];
    expect(openAIRequest[0]).toBe("https://api.openai.com/v1/responses");
    expect(openAIRequest[1].headers.authorization).toBe("Bearer sk-test");
    await expect(response.json()).resolves.toMatchObject({
      ok: true,
      report: {
        companyName: "Example",
        shouldBuildAgent: "narrow_pilot",
      },
    });
  });
});
