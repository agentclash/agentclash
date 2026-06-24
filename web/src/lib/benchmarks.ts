import fs from "fs";
import path from "path";
import { cache } from "react";
import matter from "gray-matter";

const CONTENT_DIR = path.join(process.cwd(), "content", "benchmarks");

// A single task in the head-to-head race (e.g. "Fix the auth bug").
export type BenchmarkTask = {
  id: string;
  name: string;
  summary: string;
};

// One model's row on the scoreboard. Scores are 0–1 floats, matching the
// run-ranking document the backend emits (see backend/internal/api/run_ranking.go).
export type BenchmarkResult = {
  model: string;
  provider: string;
  rank: number;
  winner: boolean;
  composite: number | null;
  correctness: number | null;
  reliability: number | null;
  latency: number | null;
  cost: number | null;
  costPerCorrectUsd: number | null;
};

export type BenchmarkReport = {
  slug: string;
  title: string;
  date: string;
  description: string;
  author: string;
  featuredModel: string;
  verdict: string;
  evalPack: string;
  // True when the scoreboard holds representative/illustrative data rather than
  // numbers from a real race. Drives the disclaimer banner on the report page.
  sample: boolean;
  runShareUrl: string;
  tasks: BenchmarkTask[];
  results: BenchmarkResult[];
};

export type BenchmarkReportWithContent = BenchmarkReport & {
  content: string;
};

function requiredText(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function optionalText(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

// Coerce a frontmatter value into a 0–1 score, or null when absent/invalid.
function scoreOrNull(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return null;
}

function parseTasks(value: unknown): BenchmarkTask[] {
  if (!Array.isArray(value)) return [];
  const tasks: BenchmarkTask[] = [];
  for (const raw of value) {
    if (typeof raw !== "object" || raw === null) continue;
    const entry = raw as Record<string, unknown>;
    const id = requiredText(entry.id);
    const name = requiredText(entry.name);
    const summary = requiredText(entry.summary);
    if (!id || !name) continue;
    tasks.push({ id, name, summary });
  }
  return tasks;
}

// Coerce + range-check a 0–1 score field. A value outside [0,1] is almost always
// an authoring mistake (e.g. `91` meant `0.91`), so warn and treat it as missing
// rather than let formatScore render an absurd "9100". Does NOT apply to
// costPerCorrectUsd (an absolute dollar amount) or rank.
function score01(
  value: unknown,
  field: string,
  model: string,
  slug: string,
): number | null {
  const parsed = scoreOrNull(value);
  if (parsed === null) return null;
  if (parsed < 0 || parsed > 1) {
    console.warn(
      `[benchmarks] ${slug}: ${model} ${field}=${parsed} is outside the 0–1 range; treating as missing.`,
    );
    return null;
  }
  return parsed;
}

function parseResults(value: unknown, slug: string): BenchmarkResult[] {
  if (!Array.isArray(value)) return [];
  const rows: Array<
    BenchmarkResult & { explicitRank: number | null; order: number }
  > = [];
  for (const [index, raw] of value.entries()) {
    if (typeof raw !== "object" || raw === null) continue;
    const entry = raw as Record<string, unknown>;
    const model = requiredText(entry.model);
    if (!model) continue;
    const explicitRank = scoreOrNull(entry.rank);
    rows.push({
      model,
      provider: optionalText(entry.provider),
      rank: 0,
      winner: entry.winner === true,
      composite: score01(entry.composite, "composite", model, slug),
      correctness: score01(entry.correctness, "correctness", model, slug),
      reliability: score01(entry.reliability, "reliability", model, slug),
      latency: score01(entry.latency, "latency", model, slug),
      cost: score01(entry.cost, "cost", model, slug),
      costPerCorrectUsd: scoreOrNull(entry.costPerCorrectUsd),
      explicitRank: explicitRank === null ? null : Math.round(explicitRank),
      order: index,
    });
  }
  // Order by explicit rank first (unranked rows last), then composite desc, then
  // input order — then assign unique sequential 1..N display ranks. Deriving the
  // "#" from final position (instead of the raw per-row rank) means partial or
  // duplicate `rank:` values can never collide in the scoreboard.
  rows.sort((a, b) => {
    const ar = a.explicitRank ?? Number.POSITIVE_INFINITY;
    const br = b.explicitRank ?? Number.POSITIVE_INFINITY;
    if (ar !== br) return ar - br;
    const ac = a.composite ?? Number.NEGATIVE_INFINITY;
    const bc = b.composite ?? Number.NEGATIVE_INFINITY;
    if (ac !== bc) return bc - ac;
    return a.order - b.order;
  });
  return rows.map((row, index) => ({
    model: row.model,
    provider: row.provider,
    rank: index + 1,
    winner: row.winner,
    composite: row.composite,
    correctness: row.correctness,
    reliability: row.reliability,
    latency: row.latency,
    cost: row.cost,
    costPerCorrectUsd: row.costPerCorrectUsd,
  }));
}

export function parseBenchmarkReport(
  slug: string,
  raw: string,
): BenchmarkReportWithContent | null {
  let parsed: ReturnType<typeof matter>;
  try {
    parsed = matter(raw);
  } catch (error) {
    console.warn(
      `[benchmarks] ${slug}.mdx: could not parse frontmatter — skipping.`,
      error,
    );
    return null;
  }

  const { data, content } = parsed;
  const title = requiredText(data.title);
  const date = requiredText(data.date);
  const description = requiredText(data.description);
  const author = requiredText(data.author);
  const featuredModel = requiredText(data.featuredModel);
  const verdict = requiredText(data.verdict);
  const results = parseResults(data.results, slug);

  // Surface authoring mistakes loudly instead of silently dropping the report
  // (a silent drop makes it vanish from the page, sitemap, RSS, and static
  // params on a green build). A misspelled/omitted required field — or a date
  // that does not parse (which would otherwise crash the sitemap) — leaves the
  // report unpublished, so name exactly what is wrong.
  const problems: string[] = [];
  if (!title) problems.push("title");
  if (!description) problems.push("description");
  if (!author) problems.push("author");
  if (!featuredModel) problems.push("featuredModel");
  if (!verdict) problems.push("verdict");
  if (results.length === 0) problems.push("results");
  if (!date) {
    problems.push("date");
  } else if (Number.isNaN(new Date(date).getTime())) {
    problems.push(`date (unparseable: "${date}")`);
  }

  if (problems.length > 0) {
    console.warn(
      `[benchmarks] ${slug}.mdx: skipped — missing or invalid field(s): ${problems.join(", ")}.`,
    );
    return null;
  }

  return {
    slug,
    title,
    date,
    description,
    author,
    featuredModel,
    verdict,
    evalPack: optionalText(data.evalPack),
    sample: data.sample === true,
    runShareUrl: optionalText(data.runShareUrl),
    tasks: parseTasks(data.tasks),
    results,
    content,
  };
}

function readReportBySlug(slug: string): BenchmarkReportWithContent | null {
  const filePath = path.join(CONTENT_DIR, `${slug}.mdx`);

  if (!fs.existsSync(filePath)) return null;

  try {
    return parseBenchmarkReport(slug, fs.readFileSync(filePath, "utf-8"));
  } catch {
    return null;
  }
}

function stripContent(report: BenchmarkReportWithContent): BenchmarkReport {
  const { content: _content, ...meta } = report;
  void _content;
  return meta;
}

// `cache()` dedupes the filesystem scan within a single render/request: a
// MarketingShell page reaches this through the header, the footer, and its own
// metadata + body, so without it one request would scan content/benchmarks
// several times over.
export const getAllReports = cache((): BenchmarkReport[] => {
  if (!fs.existsSync(CONTENT_DIR)) return [];
  const files = fs.readdirSync(CONTENT_DIR).filter((f) => f.endsWith(".mdx"));
  const reports: BenchmarkReport[] = [];

  for (const filename of files) {
    const report = readReportBySlug(filename.replace(/\.mdx$/, ""));
    if (!report) continue;
    reports.push(stripContent(report));
  }

  return reports.sort((a, b) => (a.date > b.date ? -1 : 1));
});

export function getReportBySlug(slug: string): BenchmarkReportWithContent | null {
  return readReportBySlug(slug);
}

export function getAllSlugs(): string[] {
  return getAllReports().map((report) => report.slug);
}

// True once at least one real (non-`sample`) report is published. Single source
// of truth for whether the Benchmarks section is "live": it gates public listing
// (page, sitemap, nav links) and indexability. While only illustrative `sample`
// reports exist, the section stays in a noindexed "coming soon" state and flips
// on automatically the moment a measured benchmark ships.
export function hasPublishedBenchmarks(): boolean {
  return getAllReports().some((report) => !report.sample);
}
