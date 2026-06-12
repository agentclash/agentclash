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

export type CompanyResearchBundle = {
  primary: PageSnapshot;
  supplementary: PageSnapshot[];
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
const MAX_HTML_BYTES = 256 * 1024;
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
    .replace(/&apos;/gi, "'")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/&#(\d+);/g, (_match, code: string) => {
      const parsed = Number(code);
      return Number.isSafeInteger(parsed) && parsed >= 0 && parsed <= 0x10ffff
        ? String.fromCodePoint(parsed)
        : " ";
    })
    .replace(/&#x([0-9a-f]+);/gi, (_match, hex: string) => {
      const parsed = Number.parseInt(hex, 16);
      return Number.isSafeInteger(parsed) && parsed >= 0 && parsed <= 0x10ffff
        ? String.fromCodePoint(parsed)
        : " ";
    })
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

async function readLimitedResponseText(response: Response): Promise<string> {
  const contentLength = response.headers.get("content-length");
  if (contentLength) {
    const parsedLength = Number(contentLength);
    if (Number.isFinite(parsedLength) && parsedLength > MAX_HTML_BYTES) {
      throw new AgentOpportunityError(
        "fetch_failed",
        "That page is too large to analyze from this public endpoint.",
        502,
      );
    }
  }

  if (!response.body) {
    return (await response.text()).slice(0, MAX_HTML_BYTES);
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  const chunks: string[] = [];
  let bytesRead = 0;

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    bytesRead += value.byteLength;
    if (bytesRead > MAX_HTML_BYTES) {
      await reader.cancel().catch(() => undefined);
      throw new AgentOpportunityError(
        "fetch_failed",
        "That page is too large to analyze from this public endpoint.",
        502,
      );
    }
    chunks.push(decoder.decode(value, { stream: true }));
  }

  chunks.push(decoder.decode());
  return chunks.join("");
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

    return extractPageSnapshot(currentUrl, await readLimitedResponseText(response));
  }

  throw new AgentOpportunityError(
    "fetch_failed",
    "The company site redirected too many times.",
    502,
  );
}

const SUPPLEMENTARY_PATHS = ["/about", "/pricing", "/docs", "/product", "/solutions"];

export async function fetchCompanyResearch(
  normalizedUrl: string,
  fetchImpl: typeof fetch = fetch,
): Promise<CompanyResearchBundle> {
  const primary = await fetchCompanySnapshot(normalizedUrl, fetchImpl);
  const origin = new URL(normalizedUrl).origin;
  const primaryPath = new URL(normalizedUrl).pathname.replace(/\/$/, "") || "/";
  const supplementary: PageSnapshot[] = [];

  for (const path of SUPPLEMENTARY_PATHS) {
    if (path === primaryPath || supplementary.length >= 2) break;
    try {
      const candidate = await normalizePublicUrl(`${origin}${path}`);
      supplementary.push(await fetchCompanySnapshot(candidate, fetchImpl));
    } catch {
      // Optional enrichment only.
    }
  }

  return { primary, supplementary };
}

const AGENT_OPPORTUNITY_SYSTEM_PROMPT = `You are a senior AI agent automation analyst working for AgentClash.

Your job is to decide, with evidence, whether a company should build an AI agent now, later, or not at all. You must be skeptical and conservative. Most companies do not need a customer-facing agent yet.

Research protocol (follow in order):
1. Use web search to learn what the company sells, who they serve, hiring signals, support/docs posture, and any public automation or AI claims. Prefer primary sources: the company site, docs, careers pages, and reputable news.
2. Cross-check the provided page snapshots. Call out mismatches between search results and fetched pages.
3. Identify repeatable workflows with clear inputs, outputs, and success criteria. Ignore vague "AI everywhere" ideas.
4. Estimate ROI as ranges, never point estimates. Tie hours saved to a plausible team size and ticket/doc volume.
5. List concrete failure modes (policy errors, PII leakage, wrong escalations, tool misuse) and how AgentClash should test them before launch.
6. Recommend a narrow AgentClash eval pack: realistic cases, adversarial cases, and measurable success criteria tied to the best workflow candidate.

Verdict rules:
- not_yet: no repeatable workflow, weak public evidence, or risk outweighs upside.
- eval_first: plausible workflow but evidence is thin or risks are high; test before building.
- narrow_pilot: one workflow is credible with bounded scope and clear human escalation.
- strong_fit: multiple high-fit workflows, strong public evidence, and risks are manageable with eval gates.

Output discipline:
- Do not flatter the company or assume they need an agent.
- Cite research gaps in evidenceLimitations when search or snapshots are incomplete.
- Return JSON matching the schema exactly.`;

const OPENAI_POLL_INTERVAL_MS = 2500;
const OPENAI_POLL_TIMEOUT_MS = 90_000;

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

type OpenAIResponsesPayload = {
  id?: string;
  status?: string;
  output_text?: string;
  output?: unknown;
  error?: { message?: string };
};

async function pollOpenAIResponse(
  responseId: string,
  apiKey: string,
  fetchImpl: typeof fetch,
): Promise<OpenAIResponsesPayload> {
  const deadline = Date.now() + OPENAI_POLL_TIMEOUT_MS;

  while (Date.now() < deadline) {
    await sleep(OPENAI_POLL_INTERVAL_MS);
    const response = await fetchImpl(
      `https://api.openai.com/v1/responses/${responseId}`,
      {
        headers: { authorization: `Bearer ${apiKey}` },
      },
    );
    if (!response.ok) {
      throw new AgentOpportunityError(
        "openai_failed",
        "OpenAI could not finish the report.",
        502,
      );
    }

    const payload = (await response.json()) as OpenAIResponsesPayload;
    const status = payload.status ?? "completed";
    if (status === "completed") return payload;
    if (status === "failed" || status === "cancelled" || status === "incomplete") {
      const message =
        payload.error?.message?.trim() ||
        `OpenAI returned status ${status}.`;
      throw new AgentOpportunityError("openai_failed", message, 502);
    }
  }

  throw new AgentOpportunityError(
    "openai_failed",
    "OpenAI took too long to generate the report.",
    504,
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
  research,
  apiKey = process.env.OPENAI_API_KEY,
  model = process.env.AGENT_OPPORTUNITY_OPENAI_MODEL || "gpt-5.4-mini",
  fetchImpl = fetch,
  enableWebSearch = process.env.AGENT_OPPORTUNITY_WEB_SEARCH !== "false",
}: {
  input: AgentOpportunityInput;
  research: CompanyResearchBundle;
  apiKey?: string;
  model?: string;
  fetchImpl?: typeof fetch;
  enableWebSearch?: boolean;
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
      tools: enableWebSearch ? [{ type: "web_search_preview" }] : undefined,
      input: [
        {
          role: "developer",
          content: AGENT_OPPORTUNITY_SYSTEM_PROMPT,
        },
        {
          role: "user",
          content: JSON.stringify({
            submitted: input,
            research: {
              primaryPage: research.primary,
              supplementaryPages: research.supplementary,
            },
            analysisTargets: [
              `Company URL: ${input.url}`,
              input.companySize
                ? `Declared company size: ${input.companySize}`
                : null,
              input.currentPain
                ? `Declared main pain: ${input.currentPain}`
                : null,
              input.monthlySupportVolume
                ? `Declared support volume: ${input.monthlySupportVolume}`
                : null,
            ].filter(Boolean),
            outputRequirements: [
              "Use web search plus the page snapshots as evidence.",
              "Prefer one strong workflow over a laundry list of agents.",
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

  let payload = (await response.json().catch(() => null)) as OpenAIResponsesPayload | null;
  if (
    payload?.id &&
    payload.status &&
    !["completed", "failed", "cancelled", "incomplete"].includes(payload.status)
  ) {
    payload = await pollOpenAIResponse(payload.id, apiKey, fetchImpl);
  }

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
