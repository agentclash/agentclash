import fs from "fs";
import path from "path";
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
  challengePack: string;
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

function parseResults(value: unknown): BenchmarkResult[] {
  if (!Array.isArray(value)) return [];
  const results: BenchmarkResult[] = [];
  for (const [index, raw] of value.entries()) {
    if (typeof raw !== "object" || raw === null) continue;
    const entry = raw as Record<string, unknown>;
    const model = requiredText(entry.model);
    if (!model) continue;
    const rankValue = scoreOrNull(entry.rank);
    results.push({
      model,
      provider: optionalText(entry.provider),
      rank: rankValue === null ? index + 1 : Math.round(rankValue),
      winner: entry.winner === true,
      composite: scoreOrNull(entry.composite),
      correctness: scoreOrNull(entry.correctness),
      reliability: scoreOrNull(entry.reliability),
      latency: scoreOrNull(entry.latency),
      cost: scoreOrNull(entry.cost),
      costPerCorrectUsd: scoreOrNull(entry.costPerCorrectUsd),
    });
  }
  results.sort((a, b) => a.rank - b.rank);
  return results;
}

export function parseBenchmarkReport(
  slug: string,
  raw: string,
): BenchmarkReportWithContent | null {
  try {
    const { data, content } = matter(raw);
    const title = requiredText(data.title);
    const date = requiredText(data.date);
    const description = requiredText(data.description);
    const author = requiredText(data.author);
    const featuredModel = requiredText(data.featuredModel);
    const verdict = requiredText(data.verdict);
    const results = parseResults(data.results);

    if (
      !title ||
      !date ||
      !description ||
      !author ||
      !featuredModel ||
      !verdict ||
      results.length === 0
    ) {
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
      challengePack: optionalText(data.challengePack),
      sample: data.sample === true,
      runShareUrl: optionalText(data.runShareUrl),
      tasks: parseTasks(data.tasks),
      results,
      content,
    };
  } catch {
    return null;
  }
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

export function getAllReports(): BenchmarkReport[] {
  if (!fs.existsSync(CONTENT_DIR)) return [];
  const files = fs.readdirSync(CONTENT_DIR).filter((f) => f.endsWith(".mdx"));
  const reports: BenchmarkReport[] = [];

  for (const filename of files) {
    const report = readReportBySlug(filename.replace(/\.mdx$/, ""));
    if (!report) continue;
    reports.push(stripContent(report));
  }

  return reports.sort((a, b) => (a.date > b.date ? -1 : 1));
}

export function getReportBySlug(slug: string): BenchmarkReportWithContent | null {
  return readReportBySlug(slug);
}

export function getAllSlugs(): string[] {
  return getAllReports().map((report) => report.slug);
}
