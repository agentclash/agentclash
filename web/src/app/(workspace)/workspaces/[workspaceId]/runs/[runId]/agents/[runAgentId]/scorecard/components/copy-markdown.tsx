"use client";

import { useState } from "react";
import type {
  Run,
  RunAgent,
  ScorecardResponse,
} from "@/lib/api/types";
import { Clipboard, Check } from "lucide-react";
import { parseJudgePayload, sortDimensionKeys } from "./utils";
import { cn } from "@/lib/utils";

/**
 * "Copy as markdown" is a dev-tool affordance: the most common thing a user
 * does with a scorecard in practice is paste it into Slack or a PR description.
 * Building the markdown client-side keeps it one button, no round-trip.
 */
export function CopyMarkdownButton({
  run,
  agent,
  scorecard,
}: {
  run: Run;
  agent: RunAgent;
  scorecard: ScorecardResponse;
}) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    const md = buildScorecardMarkdown(run, agent, scorecard);
    try {
      await navigator.clipboard.writeText(md);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Best-effort — silently fail on clipboard errors.
    }
  }

  return (
    <button
      type="button"
      onClick={handleCopy}
      className={cn(
        "inline-flex items-center gap-1.5 px-2.5 h-7 rounded-md border transition-colors",
        "text-2xs uppercase tracking-[0.12em]",
        copied
          ? "border-emerald-500/30 bg-emerald-500/[0.06] text-emerald-300"
          : "border-white/10 bg-white/[0.02] text-white/55 hover:text-white/85 hover:border-white/20",
      )}
    >
      {copied ? (
        <Check className="size-3" />
      ) : (
        <Clipboard className="size-3" />
      )}
      {copied ? "Copied" : "Copy MD"}
    </button>
  );
}

function buildScorecardMarkdown(
  run: Run,
  agent: RunAgent,
  scorecard: ScorecardResponse,
): string {
  const doc = scorecard.scorecard;
  const pct = (s?: number) =>
    s == null ? "—" : `${(s * 100).toFixed(1)}%`;

  const lines: string[] = [];
  lines.push(`### ${agent.label} — ${run.name}`);
  if (scorecard.overall_score != null) {
    lines.push(
      `**Overall:** ${pct(scorecard.overall_score)}${
        doc?.passed != null ? (doc.passed ? " ✓ passed" : " ✗ failed") : ""
      }${doc?.strategy ? ` · ${doc.strategy}` : ""}`,
    );
  }
  if (doc?.overall_reason) lines.push(`> ${doc.overall_reason}`);

  // Dimensions
  const dims = doc?.dimensions ?? {};
  const dimKeys = sortDimensionKeys(Object.keys(dims));
  if (dimKeys.length > 0) {
    lines.push("", "**Dimensions**");
    lines.push("| Dimension | Score | State |");
    lines.push("|---|---|---|");
    for (const k of dimKeys) {
      const d = dims[k];
      lines.push(`| ${k} | ${pct(d.score)} | ${d.state} |`);
    }
  }

  // Validators
  const vs = doc?.validator_details ?? [];
  if (vs.length > 0) {
    lines.push("", "**Validators**");
    lines.push("| Key | Type | Verdict | Score |");
    lines.push("|---|---|---|---|");
    for (const v of vs) {
      lines.push(
        `| ${v.key} | ${v.type} | ${v.verdict || v.state} | ${pct(v.normalized_score)} |`,
      );
    }
  }

  // Judges
  const js = scorecard.llm_judge_results ?? [];
  if (js.length > 0) {
    lines.push("", "**LLM Judges**");
    lines.push("| Judge | Mode | Score | σ² | Confidence |");
    lines.push("|---|---|---|---|---|");
    for (const j of js) {
      const parsed = parseJudgePayload(j);
      lines.push(
        `| ${j.judge_key} | ${j.mode} | ${parsed.available ? pct(j.normalized_score) : "—"} | ${
          j.variance != null ? j.variance.toFixed(4) : "—"
        } | ${j.confidence ?? "—"} |`,
      );
    }
  }

  // Warnings
  const warnings = doc?.warnings ?? [];
  if (warnings.length > 0) {
    lines.push("", "**Warnings**");
    for (const w of warnings) lines.push(`- ${w}`);
  }

  return lines.join("\n");
}
