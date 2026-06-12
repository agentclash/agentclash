import { describe, expect, it, vi } from "vitest";
import {
  AgentOpportunityError,
  extractPageSnapshot,
  fetchCompanyResearch,
  fetchCompanySnapshot,
  isPrivateIPAddress,
  normalizePublicUrl,
  parseAgentOpportunityReport,
} from "./agent-opportunity";

const resolvePublic = async () => ["93.184.216.34"];
const resolvePrivate = async () => ["127.0.0.1"];

describe("normalizePublicUrl", () => {
  it("accepts normal public https URLs and strips credentials/hash", async () => {
    await expect(
      normalizePublicUrl("https://user:pass@example.com/path#frag", resolvePublic),
    ).resolves.toBe("https://example.com/path");
  });

  it("blocks localhost and private DNS results", async () => {
    await expect(
      normalizePublicUrl("http://localhost:3000", resolvePublic),
    ).rejects.toMatchObject({ code: "blocked_url" });
    await expect(
      normalizePublicUrl("https://internal.example.com", resolvePrivate),
    ).rejects.toMatchObject({ code: "blocked_url" });
  });

  it("rejects non-http URLs", async () => {
    await expect(
      normalizePublicUrl("file:///etc/passwd", resolvePublic),
    ).rejects.toMatchObject({ code: "invalid_url" });
  });
});

describe("isPrivateIPAddress", () => {
  it("detects common private and loopback ranges", () => {
    expect(isPrivateIPAddress("127.0.0.1")).toBe(true);
    expect(isPrivateIPAddress("10.0.0.5")).toBe(true);
    expect(isPrivateIPAddress("172.20.1.1")).toBe(true);
    expect(isPrivateIPAddress("192.168.1.1")).toBe(true);
    expect(isPrivateIPAddress("::1")).toBe(true);
    expect(isPrivateIPAddress("8.8.8.8")).toBe(false);
  });
});

describe("extractPageSnapshot", () => {
  it("removes scripts and styles while preserving useful page signals", () => {
    const snapshot = extractPageSnapshot(
      "https://example.com",
      `<!doctype html>
        <html>
          <head>
            <title>Example Co</title>
            <meta name="description" content="AI support for commerce teams">
            <style>.hidden { color: red; }</style>
          </head>
          <body>
            <h1>Support automation</h1>
            <script>window.secret = "nope"</script>
            <p>Resolve customer questions faster &amp; route edge cases.</p>
            <p>Safety &lt; speed &#8217; support &#x2019; routing &apos;done&apos;.</p>
          </body>
        </html>`,
    );

    expect(snapshot.title).toBe("Example Co");
    expect(snapshot.description).toBe("AI support for commerce teams");
    expect(snapshot.text).toContain("Support automation");
    expect(snapshot.text).toContain("faster & route");
    expect(snapshot.text).toContain(
      `Safety < speed ${String.fromCodePoint(8217)} support ${String.fromCodePoint(
        8217,
      )} routing 'done'.`,
    );
    expect(snapshot.text).not.toContain("window.secret");
    expect(snapshot.text).not.toContain("hidden");
  });
});

describe("fetchCompanySnapshot", () => {
  it("rejects pages with oversized content-length before reading the body", async () => {
    const response = new Response("tiny", {
      headers: {
        "content-type": "text/html",
        "content-length": String(1024 * 1024),
      },
    });
    const fetchImpl = async () => response;

    await expect(
      fetchCompanySnapshot("https://example.com", fetchImpl as typeof fetch),
    ).rejects.toMatchObject({ code: "fetch_failed" });
  });
});

describe("fetchCompanyResearch", () => {
  it("returns the primary snapshot and ignores failed supplementary pages", async () => {
    const fetchImpl = vi.fn(async (url: string) => {
      if (url === "https://example.com/") {
        return new Response(
          "<html><head><title>Example</title></head><body>Support automation.</body></html>",
          { headers: { "content-type": "text/html" } },
        );
      }
      throw new Error("missing page");
    });

    const research = await fetchCompanyResearch(
      "https://example.com/",
      fetchImpl as typeof fetch,
    );

    expect(research.primary.title).toBe("Example");
    expect(research.supplementary).toHaveLength(0);
  });

  it("still fetches other supplementary pages when the primary URL matches one path", async () => {
    const fetchImpl = vi.fn(async (url: string) => {
      if (
        url === "https://example.com/about" ||
        url === "https://example.com/pricing"
      ) {
        return new Response(
          `<html><head><title>${url}</title></head><body>More context.</body></html>`,
          { headers: { "content-type": "text/html" } },
        );
      }
      throw new Error(`missing page: ${url}`);
    });

    const research = await fetchCompanyResearch(
      "https://example.com/about",
      fetchImpl as typeof fetch,
    );

    expect(research.primary.url).toBe("https://example.com/about");
    expect(research.supplementary).toHaveLength(1);
    expect(research.supplementary[0]?.url).toBe("https://example.com/pricing");
  });
});

describe("parseAgentOpportunityReport", () => {
  const validReport = {
    analyzedUrl: "https://example.com/",
    companyName: "Example",
    generatedAt: "2026-06-12T00:00:00.000Z",
    agentFitScore: 72,
    fitLevel: "moderate",
    shouldBuildAgent: "narrow_pilot",
    honestVerdict: "A narrow support triage pilot is worth testing.",
    summary: "The site shows repeatable support and onboarding workflows.",
    useCases: [
      {
        title: "Support triage",
        workflow: "Classify inbound questions and draft replies.",
        fit: "high",
        estimatedMonthlyHoursSaved: "20-40",
        estimatedMonthlySavingsUsd: "$1,500-$4,000",
        complexity: "medium",
        why: "The site has docs and clear support categories.",
        firstEvalTasks: ["Refund request", "Pricing question"],
      },
    ],
    risks: [
      {
        risk: "Incorrect customer advice",
        severity: "medium",
        mitigation: "Route low-confidence answers to humans.",
      },
      {
        risk: "Unclear escalation boundaries",
        severity: "medium",
        mitigation: "Evaluate edge cases before launch.",
      },
    ],
    evaluationPack: {
      name: "Support triage pilot",
      recommendedCases: 25,
      adversarialCases: 8,
      successCriteria: ["90% correct routing", "No policy hallucinations"],
    },
    nextSteps: ["Collect real tickets", "Run an AgentClash race"],
    evidenceLimitations: ["Only public homepage content was analyzed."],
  };

  it("returns valid structured reports", () => {
    expect(parseAgentOpportunityReport(validReport).companyName).toBe("Example");
  });

  it("rejects malformed report payloads", () => {
    expect(() =>
      parseAgentOpportunityReport({ ...validReport, agentFitScore: 140 }),
    ).toThrow(AgentOpportunityError);
  });
});
