import { lookup } from "node:dns/promises";
import net from "node:net";

export type AgentOpportunityUseCase = {
  title: string;
  workflow: string;
  fit: "low" | "medium" | "high";
  estimatedMonthlyHoursSaved: string;
  estimatedMonthlySavingsUsd: string;
  complexity: "low" | "medium" | "high";
  why: string;
  firstEvalTasks: string[];
};

export type AgentOpportunityRisk = {
  risk: string;
  severity: "low" | "medium" | "high";
  mitigation: string;
};

export type AgentOpportunityReport = {
  analyzedUrl: string;
  companyName: string;
  generatedAt: string;
  agentFitScore: number;
  fitLevel: "low" | "moderate" | "high";
  shouldBuildAgent: "not_yet" | "narrow_pilot" | "strong_fit" | "eval_first";
  honestVerdict: string;
  summary: string;
  useCases: AgentOpportunityUseCase[];
  risks: AgentOpportunityRisk[];
  evaluationPack: {
    name: string;
    recommendedCases: number;
    adversarialCases: number;
    successCriteria: string[];
  };
  nextSteps: string[];
  evidenceLimitations: string[];
};

export type PageSnapshot = {
  url: string;
  title: string;
  description: string;
  text: string;
};

export type AgentOpportunityInput = {
  url: string;
  companySize?: string;
  currentPain?: string;
  monthlySupportVolume?: string;
};

export class AgentOpportunityError extends Error {
  constructor(
    public code:
      | "invalid_url"
      | "blocked_url"
      | "fetch_failed"
      | "openai_not_configured"
      | "openai_failed"
      | "invalid_model_response",
    message: string,
    public status: number,
  ) {
    super(message);
    this.name = "AgentOpportunityError";
  }
}

const MAX_URL_LENGTH = 2048;
const MAX_TEXT_CHARS = 12000;
const USER_AGENT =
  "AgentClash-Agent-Opportunity-Report/1.0 (+https://agentclash.dev)";

export function isPrivateIPAddress(address: string): boolean {
  const mappedV4 = address.match(/^::ffff:(\d+\.\d+\.\d+\.\d+)$/i);
  if (mappedV4) return isPrivateIPAddress(mappedV4[1]);

  const ipVersion = net.isIP(address);
  if (ipVersion === 4) {
    const octets = address.split(".").map(Number);
    const [a, b] = octets;
    return (
      a === 0 ||
      a === 10 ||
      a === 127 ||
      (a === 100 && b >= 64 && b <= 127) ||
      (a === 169 && b === 254) ||
      (a === 172 && b >= 16 && b <= 31) ||
      (a === 192 && b === 168) ||
      (a === 192 && b === 0) ||
      (a === 198 && (b === 18 || b === 19)) ||
      (a === 198 && b === 51 && octets[2] === 100) ||
      (a === 203 && b === 0 && octets[2] === 113) ||
      a >= 224
    );
  }

  if (ipVersion === 6) {
    const normalized = address.toLowerCase();
    return (
      normalized === "::" ||
      normalized === "::1" ||
      normalized.startsWith("fc") ||
      normalized.startsWith("fd") ||
      normalized.startsWith("fe8") ||
      normalized.startsWith("fe9") ||
      normalized.startsWith("fea") ||
      normalized.startsWith("feb")
    );
  }

  return true;
}

function hasBlockedHostname(hostname: string): boolean {
  const value = hostname.toLowerCase().replace(/\.$/, "");
  return (
    value === "localhost" ||
    value.endsWith(".localhost") ||
    value.endsWith(".local") ||
    value.endsWith(".internal") ||
    value.endsWith(".test") ||
    value.endsWith(".invalid")
  );
}

export async function normalizePublicUrl(
  input: string,
  resolveHostname: (hostname: string) => Promise<string[]> = async (hostname) => {
    const records = await lookup(hostname, { all: true, verbatim: true });
    return records.map((record) => record.address);
  },
): Promise<string> {
  const trimmed = input.trim();
  if (!trimmed || trimmed.length > MAX_URL_LENGTH) {
    throw new AgentOpportunityError(
      "invalid_url",
      "Enter a valid company URL.",
      400,
    );
  }

  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    throw new AgentOpportunityError(
      "invalid_url",
      "Enter a full URL, including https://.",
      400,
    );
  }

  if (parsed.protocol !== "https:" && parsed.protocol !== "http:") {
    throw new AgentOpportunityError(
      "invalid_url",
      "Only http and https URLs can be analyzed.",
      400,
    );
  }
  parsed.username = "";
  parsed.password = "";
  parsed.hash = "";

  if (hasBlockedHostname(parsed.hostname)) {
    throw new AgentOpportunityError(
      "blocked_url",
      "That hostname cannot be analyzed from this public endpoint.",
      400,
    );
  }

  if (net.isIP(parsed.hostname)) {
    if (isPrivateIPAddress(parsed.hostname)) {
      throw new AgentOpportunityError(
        "blocked_url",
        "Private network URLs cannot be analyzed.",
        400,
      );
    }
    return parsed.toString();
  }

  let addresses: string[];
  try {
    addresses = await resolveHostname(parsed.hostname);
  } catch {
    throw new AgentOpportunityError(
      "invalid_url",
      "We could not resolve that company URL.",
      400,
    );
  }

  if (addresses.length === 0 || addresses.some(isPrivateIPAddress)) {
    throw new AgentOpportunityError(
      "blocked_url",
      "That URL resolves to a private or restricted network address.",
      400,
    );
  }

  return parsed.toString();
}

export function extractPageSnapshot(url: string, html: string): PageSnapshot {
  const title =
    html.match(/<title[^>]*>([\s\S]*?)<\/title>/i)?.[1]?.trim() ?? "";
  const description =
    html.match(
      /<meta[^>]+name=["']description["'][^>]+content=["']([^"']+)["'][^>]*>/i,
    )?.[1] ??
    html.match(
      /<meta[^>]+content=["']([^"']+)["'][^>]+name=["']description["'][^>]*>/i,
    )?.[1] ??
    "";
  const text = html
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<noscript[\s\S]*?<\/noscript>/gi, " ")
    .replace(/<!--[\s\S]*?-->/g, " ")
    .replace(/<[^>]+>/g, " ")
    .replace(/&nbsp;/gi, " ")
    .replace(/&amp;/gi, "&")
    .replace(/&quot;/gi, '"')
    .replace(/&#39;/gi, "'")
    .replace(/\s+/g, " ")
    .trim()
    .slice(0, MAX_TEXT_CHARS);

  return {
    url,
    title: title.replace(/\s+/g, " ").slice(0, 180),
    description: description.replace(/\s+/g, " ").slice(0, 300),
    text,
  };
}

export async function fetchCompanySnapshot(
  normalizedUrl: string,
  fetchImpl: typeof fetch = fetch,
): Promise<PageSnapshot> {
  let currentUrl = normalizedUrl;

  for (let redirectCount = 0; redirectCount < 3; redirectCount += 1) {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10000);
    let response: Response;
    try {
      response = await fetchImpl(currentUrl, {
        headers: { "user-agent": USER_AGENT, accept: "text/html" },
        redirect: "manual",
        signal: controller.signal,
      });
    } catch {
      throw new AgentOpportunityError(
        "fetch_failed",
        "We could not fetch that company site.",
        502,
      );
    } finally {
      clearTimeout(timeout);
    }

    if (
      response.status >= 300 &&
      response.status < 400 &&
      response.headers.get("location")
    ) {
      const redirected = new URL(response.headers.get("location")!, currentUrl);
      currentUrl = await normalizePublicUrl(redirected.toString());
      continue;
    }

    if (!response.ok) {
      throw new AgentOpportunityError(
        "fetch_failed",
        `The company site returned HTTP ${response.status}.`,
        502,
      );
    }

    const contentType = response.headers.get("content-type") ?? "";
    if (!contentType.includes("text/html")) {
      throw new AgentOpportunityError(
        "fetch_failed",
        "That URL did not return an HTML page we can analyze.",
        502,
      );
    }

    return extractPageSnapshot(currentUrl, await response.text());
  }

  throw new AgentOpportunityError(
    "fetch_failed",
    "The company site redirected too many times.",
    502,
  );
}

function assertString(value: unknown): value is string {
  return typeof value === "string" && value.trim().length > 0;
}

function assertStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every(assertString);
}

function isUseCase(value: unknown): value is AgentOpportunityUseCase {
  if (!value || typeof value !== "object") return false;
  const item = value as Partial<AgentOpportunityUseCase>;
  return (
    assertString(item.title) &&
    assertString(item.workflow) &&
    ["low", "medium", "high"].includes(item.fit ?? "") &&
    assertString(item.estimatedMonthlyHoursSaved) &&
    assertString(item.estimatedMonthlySavingsUsd) &&
    ["low", "medium", "high"].includes(item.complexity ?? "") &&
    assertString(item.why) &&
    assertStringArray(item.firstEvalTasks)
  );
}

function isRisk(value: unknown): value is AgentOpportunityRisk {
  if (!value || typeof value !== "object") return false;
  const item = value as Partial<AgentOpportunityRisk>;
  return (
    assertString(item.risk) &&
    ["low", "medium", "high"].includes(item.severity ?? "") &&
    assertString(item.mitigation)
  );
}

export function parseAgentOpportunityReport(
  value: unknown,
): AgentOpportunityReport {
  if (!value || typeof value !== "object") {
    throw new AgentOpportunityError(
      "invalid_model_response",
      "The model returned an invalid report.",
      502,
    );
  }

  const report = value as Partial<AgentOpportunityReport>;
  const fitLevels = ["low", "moderate", "high"];
  const verdicts = ["not_yet", "narrow_pilot", "strong_fit", "eval_first"];

  if (
    !assertString(report.analyzedUrl) ||
    !assertString(report.companyName) ||
    !assertString(report.generatedAt) ||
    typeof report.agentFitScore !== "number" ||
    report.agentFitScore < 0 ||
    report.agentFitScore > 100 ||
    !fitLevels.includes(report.fitLevel ?? "") ||
    !verdicts.includes(report.shouldBuildAgent ?? "") ||
    !assertString(report.honestVerdict) ||
    !assertString(report.summary) ||
    !Array.isArray(report.useCases) ||
    !report.useCases.every(isUseCase) ||
    !Array.isArray(report.risks) ||
    !report.risks.every(isRisk) ||
    !report.evaluationPack ||
    typeof report.evaluationPack !== "object" ||
    !assertString(report.evaluationPack.name) ||
    typeof report.evaluationPack.recommendedCases !== "number" ||
    typeof report.evaluationPack.adversarialCases !== "number" ||
    !assertStringArray(report.evaluationPack.successCriteria) ||
    !assertStringArray(report.nextSteps) ||
    !assertStringArray(report.evidenceLimitations)
  ) {
    throw new AgentOpportunityError(
      "invalid_model_response",
      "The model returned an incomplete report.",
      502,
    );
  }

  return report as AgentOpportunityReport;
}

const REPORT_SCHEMA = {
  name: "agent_opportunity_report",
  strict: true,
  schema: {
    type: "object",
    additionalProperties: false,
    required: [
      "analyzedUrl",
      "companyName",
      "generatedAt",
      "agentFitScore",
      "fitLevel",
      "shouldBuildAgent",
      "honestVerdict",
      "summary",
      "useCases",
      "risks",
      "evaluationPack",
      "nextSteps",
      "evidenceLimitations",
    ],
    properties: {
      analyzedUrl: { type: "string" },
      companyName: { type: "string" },
      generatedAt: { type: "string" },
      agentFitScore: { type: "number", minimum: 0, maximum: 100 },
      fitLevel: { type: "string", enum: ["low", "moderate", "high"] },
      shouldBuildAgent: {
        type: "string",
        enum: ["not_yet", "narrow_pilot", "strong_fit", "eval_first"],
      },
      honestVerdict: { type: "string" },
      summary: { type: "string" },
      useCases: {
        type: "array",
        minItems: 1,
        maxItems: 4,
        items: {
          type: "object",
          additionalProperties: false,
          required: [
            "title",
            "workflow",
            "fit",
            "estimatedMonthlyHoursSaved",
            "estimatedMonthlySavingsUsd",
            "complexity",
            "why",
            "firstEvalTasks",
          ],
          properties: {
            title: { type: "string" },
            workflow: { type: "string" },
            fit: { type: "string", enum: ["low", "medium", "high"] },
            estimatedMonthlyHoursSaved: { type: "string" },
            estimatedMonthlySavingsUsd: { type: "string" },
            complexity: { type: "string", enum: ["low", "medium", "high"] },
            why: { type: "string" },
            firstEvalTasks: {
              type: "array",
              minItems: 2,
              maxItems: 5,
              items: { type: "string" },
            },
          },
        },
      },
      risks: {
        type: "array",
        minItems: 2,
        maxItems: 5,
        items: {
          type: "object",
          additionalProperties: false,
          required: ["risk", "severity", "mitigation"],
          properties: {
            risk: { type: "string" },
            severity: { type: "string", enum: ["low", "medium", "high"] },
            mitigation: { type: "string" },
          },
        },
      },
      evaluationPack: {
        type: "object",
        additionalProperties: false,
        required: [
          "name",
          "recommendedCases",
          "adversarialCases",
          "successCriteria",
        ],
        properties: {
          name: { type: "string" },
          recommendedCases: { type: "number" },
          adversarialCases: { type: "number" },
          successCriteria: {
            type: "array",
            minItems: 2,
            maxItems: 5,
            items: { type: "string" },
          },
        },
      },
      nextSteps: {
        type: "array",
        minItems: 2,
        maxItems: 5,
        items: { type: "string" },
      },
      evidenceLimitations: {
        type: "array",
        minItems: 1,
        maxItems: 4,
        items: { type: "string" },
      },
    },
  },
} as const;

function extractOpenAIText(payload: unknown): string {
  if (!payload || typeof payload !== "object") return "";
  const root = payload as { output_text?: unknown; output?: unknown };
  if (typeof root.output_text === "string") return root.output_text;

  if (Array.isArray(root.output)) {
    for (const item of root.output) {
      if (!item || typeof item !== "object") continue;
      const content = (item as { content?: unknown }).content;
      if (!Array.isArray(content)) continue;
      for (const part of content) {
        if (
          part &&
          typeof part === "object" &&
          typeof (part as { text?: unknown }).text === "string"
        ) {
          return (part as { text: string }).text;
        }
      }
    }
  }

  return "";
}

export async function analyzeAgentOpportunity({
  input,
  snapshot,
  apiKey = process.env.OPENAI_API_KEY,
  model = process.env.AGENT_OPPORTUNITY_OPENAI_MODEL || "gpt-5.4-mini",
  fetchImpl = fetch,
}: {
  input: AgentOpportunityInput;
  snapshot: PageSnapshot;
  apiKey?: string;
  model?: string;
  fetchImpl?: typeof fetch;
}): Promise<AgentOpportunityReport> {
  if (!apiKey) {
    throw new AgentOpportunityError(
      "openai_not_configured",
      "OpenAI is not configured for this endpoint.",
      503,
    );
  }

  const response = await fetchImpl("https://api.openai.com/v1/responses", {
    method: "POST",
    headers: {
      authorization: `Bearer ${apiKey}`,
      "content-type": "application/json",
    },
    body: JSON.stringify({
      model,
      input: [
        {
          role: "system",
          content:
            "You are an honest AI agent automation analyst for AgentClash. Decide whether a company should build an AI agent, where it would help, where it would fail, and how AgentClash should evaluate it before deployment. Do not overstate savings. If the company does not clearly need an agent, say so.",
        },
        {
          role: "user",
          content: JSON.stringify({
            submitted: input,
            page: snapshot,
            instructions: [
              "Base your analysis only on the public page snapshot and user-provided context.",
              "Use conservative savings ranges. Avoid fake precision.",
              "Tie recommended evaluation cases to real workflows visible from the site.",
              "Return JSON matching the schema exactly.",
            ],
          }),
        },
      ],
      text: {
        format: {
          type: "json_schema",
          ...REPORT_SCHEMA,
        },
      },
    }),
  });

  if (!response.ok) {
    throw new AgentOpportunityError(
      "openai_failed",
      "OpenAI could not generate the report.",
      502,
    );
  }

  const payload = await response.json().catch(() => null);
  const text = extractOpenAIText(payload);
  if (!text) {
    throw new AgentOpportunityError(
      "invalid_model_response",
      "OpenAI returned an empty report.",
      502,
    );
  }

  try {
    return parseAgentOpportunityReport(JSON.parse(text));
  } catch (error) {
    if (error instanceof AgentOpportunityError) throw error;
    throw new AgentOpportunityError(
      "invalid_model_response",
      "OpenAI returned malformed JSON.",
      502,
    );
  }
}
