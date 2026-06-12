"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";
import { Quote, Check, X, Code2 } from "lucide-react";
import { scoreColor } from "@/lib/scores";
import type { JudgeCall } from "./utils";

/**
 * A structured, *visual* rendering of a single LLM-judge sample.
 *
 * The v1 scorecard rendered `response_text` inside a `<pre>` block — a raw JSON
 * dump that forced the user to read the model's verdict through a code lens.
 * This card parses the response (defensively — judges misbehave) and shows
 * each known field as the right visual primitive:
 *
 *   verdict  → pass/fail pill
 *   score    → coloured mono figure
 *   confidence → small chip
 *   rationale-family fields → quoted prose block (readable, serif quotation mark)
 *   evidence-family fields  → bulleted list
 *   feedback-family fields  → per-bucket sections (strengths / weaknesses / ...)
 *   anything else → compact key/value chips
 *   parse failure → graceful pre-wrapped fallback, labelled as raw
 *
 * Users can still see the raw JSON via a small toggle — best of both worlds.
 */

interface Parsed {
  verdict?: { label: string; kind: "pass" | "fail" };
  score?: number;
  confidence?: string;
  ranking?: string[];
  rationale?: string;
  evidence?: string[];
  feedback: { bucket: string; items: string[] }[];
  other: { key: string; value: string }[];
  unparseable: boolean;
}

const RATIONALE_KEYS = new Set([
  "rationale",
  "reason",
  "reasoning",
  "justification",
  "explanation",
  "analysis",
  "notes",
  "thinking",
  "thought",
  "commentary",
]);

const EVIDENCE_KEYS = new Set([
  "evidence",
  "citations",
  "examples",
  "references",
  "quotes",
  "sources",
  "supporting_quotes",
]);

const FEEDBACK_KEYS = new Set([
  "strengths",
  "weaknesses",
  "concerns",
  "improvements",
  "suggestions",
  "issues",
  "risks",
  "observations",
]);

function sanitizeResponseText(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed.startsWith("```")) {
    const stripped = trimmed
      .replace(/^```(?:json|JSON)?/, "")
      .replace(/```$/, "")
      .trim();
    return stripped;
  }
  return trimmed;
}

function extractJsonSlice(raw: string): string | null {
  const start = raw.indexOf("{");
  const end = raw.lastIndexOf("}");
  if (start >= 0 && end > start) return raw.slice(start, end + 1);
  return null;
}

function parseResponse(raw: string): Parsed {
  const base: Parsed = { feedback: [], other: [], unparseable: false };
  if (!raw) return { ...base, unparseable: true };

  const sanitized = sanitizeResponseText(raw);
  let obj: Record<string, unknown> | null = null;
  try {
    obj = JSON.parse(sanitized);
  } catch {
    const slice = extractJsonSlice(sanitized);
    if (slice) {
      try {
        obj = JSON.parse(slice);
      } catch {
        // fall through
      }
    }
  }
  if (!obj || typeof obj !== "object" || Array.isArray(obj)) {
    return { ...base, unparseable: true };
  }

  const out: Parsed = { feedback: [], other: [], unparseable: false };

  for (const [rawKey, rawValue] of Object.entries(obj)) {
    const key = rawKey.toLowerCase();

    if (key === "pass" && typeof rawValue === "boolean") {
      out.verdict = { label: rawValue ? "Pass" : "Fail", kind: rawValue ? "pass" : "fail" };
      continue;
    }
    if (key === "verdict" && typeof rawValue === "string") {
      const v = rawValue.toLowerCase();
      if (["pass", "yes", "true", "correct"].includes(v)) {
        out.verdict = { label: "Pass", kind: "pass" };
      } else if (["fail", "no", "false", "incorrect"].includes(v)) {
        out.verdict = { label: "Fail", kind: "fail" };
      } else {
        out.other.push({ key: rawKey, value: rawValue });
      }
      continue;
    }
    if (key === "score" && typeof rawValue === "number") {
      out.score = rawValue;
      continue;
    }
    if (key === "confidence" && typeof rawValue === "string") {
      out.confidence = rawValue;
      continue;
    }
    if (
      (key === "ranking" || key === "ranked_ids") &&
      Array.isArray(rawValue) &&
      rawValue.every((v) => typeof v === "string")
    ) {
      out.ranking = rawValue as string[];
      continue;
    }

    if (RATIONALE_KEYS.has(key)) {
      const text = stringifyValue(rawValue);
      if (text) {
        // If multiple rationale-ish fields show up, concat them; but keep them
        // separated by a blank line so readers still perceive the grouping.
        out.rationale = out.rationale ? `${out.rationale}\n\n${text}` : text;
      }
      continue;
    }

    if (EVIDENCE_KEYS.has(key)) {
      const list = toStringList(rawValue);
      if (list.length > 0) {
        out.evidence = (out.evidence ?? []).concat(list);
      } else {
        const text = stringifyValue(rawValue);
        if (text) out.other.push({ key: rawKey, value: text });
      }
      continue;
    }

    if (FEEDBACK_KEYS.has(key)) {
      const list = toStringList(rawValue);
      if (list.length > 0) {
        out.feedback.push({ bucket: rawKey, items: list });
      } else {
        const text = stringifyValue(rawValue);
        if (text) out.other.push({ key: rawKey, value: text });
      }
      continue;
    }

    // Anything left goes into "other" as a compact chip. Arrays/objects get
    // JSON-stringified but kept short.
    const text = stringifyValue(rawValue);
    if (text) out.other.push({ key: rawKey, value: text });
  }

  return out;
}

function toStringList(v: unknown): string[] {
  if (Array.isArray(v)) {
    return v
      .map((item) => stringifyValue(item).trim())
      .filter((s) => s.length > 0);
  }
  return [];
}

function stringifyValue(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  try {
    return JSON.stringify(v);
  } catch {
    return "";
  }
}

/* --------------------------------------------------------------- Component */

export function JudgeSampleCard({ call }: { call: JudgeCall }) {
  const [showRaw, setShowRaw] = useState(false);
  const hasError = !!call.error;
  const hasResponse = !!call.responseText;

  const parsed = hasResponse ? parseResponse(call.responseText!) : null;

  // Tint the card by call score — green for high, red for low.
  const accent =
    call.score == null
      ? "rgba(255,255,255,0.18)"
      : call.score >= 0.8
        ? "rgb(52, 211, 153)"
        : call.score >= 0.5
          ? "rgb(251, 191, 36)"
          : "rgb(248, 113, 113)";

  return (
    <div
      className={cn(
        "relative border border-white/[0.06] rounded-md overflow-hidden",
        "bg-[linear-gradient(180deg,rgba(255,255,255,0.015),rgba(255,255,255,0.005))]",
      )}
    >
      {/* Left accent stripe — colour hints at score without shouting */}
      <span
        aria-hidden
        className="absolute left-0 top-0 bottom-0 w-[2px] opacity-60"
        style={{ background: accent }}
      />

      {/* Header — sample #, model, score, confidence, raw toggle */}
      <div className="flex items-center gap-3 pl-4 pr-3 h-10 border-b border-white/[0.05]">
        <span className="font-[family-name:var(--font-mono)] text-2xs text-white/35 tabular-nums w-6">
          #{call.sampleIndex ?? "?"}
        </span>
        <span
          className="font-[family-name:var(--font-mono)] text-2xs text-white/65 truncate flex-1"
          title={call.model}
        >
          {call.model}
        </span>
        {parsed?.confidence && <ConfidenceChip value={parsed.confidence} />}
        {parsed?.verdict && <VerdictChip verdict={parsed.verdict} />}
        {call.score != null && (
          <span
            className={cn(
              "font-[family-name:var(--font-mono)] text-xs tabular-nums",
              scoreColor(call.score),
            )}
          >
            {(call.score * 100).toFixed(1)}
          </span>
        )}
        {hasResponse && (
          <button
            type="button"
            onClick={() => setShowRaw(!showRaw)}
            className={cn(
              "ml-1 inline-flex items-center gap-1 text-2xs uppercase tracking-[0.14em] rounded px-1.5 h-6 border transition-colors",
              showRaw
                ? "border-white/25 bg-white/[0.05] text-white/75"
                : "border-white/10 text-white/40 hover:text-white/70 hover:border-white/20",
            )}
            title="Toggle raw response"
          >
            <Code2 className="size-2.5" />
            raw
          </button>
        )}
      </div>

      {/* Body */}
      <div className="px-4 py-3.5 space-y-3.5">
        {hasError && (
          <div className="text-xs text-red-300/85 leading-snug whitespace-pre-wrap">
            {call.error}
          </div>
        )}

        {!hasError && parsed && !parsed.unparseable && (
          <>
            {parsed.ranking && parsed.ranking.length > 0 && (
              <RankingBlock ranking={parsed.ranking} />
            )}

            {parsed.rationale && <RationaleBlock text={parsed.rationale} />}

            {parsed.evidence && parsed.evidence.length > 0 && (
              <BulletBlock title="Evidence" items={parsed.evidence} />
            )}

            {parsed.feedback.map((group) => (
              <BulletBlock
                key={group.bucket}
                title={group.bucket.replace(/_/g, " ")}
                items={group.items}
                tone={
                  /weak|concern|issue|risk/.test(group.bucket.toLowerCase())
                    ? "warn"
                    : /strength|improv|suggest|observ/.test(
                          group.bucket.toLowerCase(),
                        )
                      ? "good"
                      : "neutral"
                }
              />
            ))}

            {parsed.other.length > 0 && <OtherFields fields={parsed.other} />}

            {!parsed.rationale &&
              !parsed.evidence &&
              parsed.feedback.length === 0 &&
              parsed.other.length === 0 &&
              !parsed.ranking && (
                <p className="text-xs text-white/40 italic">
                  No rationale was provided by the judge.
                </p>
              )}
          </>
        )}

        {!hasError && parsed?.unparseable && hasResponse && (
          <FallbackText text={call.responseText!} />
        )}

        {/* Raw toggle — always available when response exists */}
        {hasResponse && showRaw && (
          <details open className="text-2xs">
            <summary className="cursor-pointer text-white/40 uppercase tracking-[0.16em] mb-1.5">
              Raw response
            </summary>
            <pre className="mt-1 font-[family-name:var(--font-mono)] whitespace-pre-wrap text-white/55 bg-black/40 border border-white/[0.05] rounded px-2.5 py-2 max-h-60 overflow-y-auto leading-snug">
              {call.responseText}
            </pre>
          </details>
        )}
      </div>
    </div>
  );
}

/* ----------------------------------------------------------------- Blocks */

function RationaleBlock({ text }: { text: string }) {
  return (
    <div className="relative pl-6">
      <Quote
        aria-hidden
        className="absolute left-0 top-0 size-4 text-white/20"
      />
      <p className="text-sm text-white/80 leading-relaxed whitespace-pre-wrap font-[family-name:var(--font-body)]">
        {text}
      </p>
    </div>
  );
}

function BulletBlock({
  title,
  items,
  tone = "neutral",
}: {
  title: string;
  items: string[];
  tone?: "good" | "warn" | "neutral";
}) {
  const marker = {
    good: "bg-emerald-400/70",
    warn: "bg-amber-400/70",
    neutral: "bg-white/30",
  }[tone];
  return (
    <div>
      <h4 className="text-2xs uppercase tracking-[0.2em] text-white/45 mb-1.5 font-medium">
        {title}
      </h4>
      <ul className="space-y-1">
        {items.map((item, i) => (
          <li
            key={i}
            className="flex gap-2 text-xs text-white/75 leading-relaxed"
          >
            <span
              className={cn("mt-[7px] size-1 rounded-full shrink-0", marker)}
              aria-hidden
            />
            <span className="flex-1">{item}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function RankingBlock({ ranking }: { ranking: string[] }) {
  return (
    <div>
      <h4 className="text-2xs uppercase tracking-[0.2em] text-white/45 mb-1.5 font-medium">
        Ranking
      </h4>
      <ol className="space-y-1">
        {ranking.map((id, i) => (
          <li
            key={id}
            className="flex gap-3 text-xs text-white/70 font-[family-name:var(--font-mono)] leading-tight"
          >
            <span className="text-white/35 tabular-nums w-4">{i + 1}.</span>
            <span className="truncate">{id}</span>
          </li>
        ))}
      </ol>
    </div>
  );
}

function OtherFields({
  fields,
}: {
  fields: { key: string; value: string }[];
}) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {fields.map((f) => (
        <div
          key={f.key}
          className="inline-flex items-baseline gap-1.5 px-2 py-1 rounded border border-white/[0.08] bg-white/[0.02]"
        >
          <span className="text-2xs uppercase tracking-[0.14em] text-white/40">
            {f.key.replace(/_/g, " ")}
          </span>
          <span
            className="text-2xs text-white/75 font-[family-name:var(--font-mono)] max-w-[240px] truncate"
            title={f.value}
          >
            {f.value}
          </span>
        </div>
      ))}
    </div>
  );
}

function FallbackText({ text }: { text: string }) {
  return (
    <div>
      <div className="flex items-center gap-2 mb-1.5">
        <span className="text-2xs uppercase tracking-[0.18em] text-white/40">
          Raw response
        </span>
        <span className="text-2xs text-white/30">
          · couldn&apos;t parse as structured JSON
        </span>
      </div>
      <pre className="font-[family-name:var(--font-mono)] text-2xs text-white/70 leading-relaxed whitespace-pre-wrap bg-black/30 border border-white/[0.05] rounded px-3 py-2 max-h-60 overflow-y-auto">
        {text}
      </pre>
    </div>
  );
}

function VerdictChip({
  verdict,
}: {
  verdict: { label: string; kind: "pass" | "fail" };
}) {
  const pass = verdict.kind === "pass";
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 text-2xs uppercase tracking-[0.14em] px-1.5 h-5 rounded border",
        pass
          ? "border-emerald-500/30 bg-emerald-500/[0.08] text-emerald-300"
          : "border-red-500/30 bg-red-500/[0.08] text-red-300",
      )}
    >
      {pass ? <Check className="size-2.5" /> : <X className="size-2.5" />}
      {verdict.label}
    </span>
  );
}

function ConfidenceChip({ value }: { value: string }) {
  const v = value.toLowerCase();
  const tone = v.includes("high")
    ? "text-emerald-300/85 border-emerald-500/20"
    : v.includes("low")
      ? "text-red-300/85 border-red-500/20"
      : "text-amber-300/85 border-amber-500/20";
  return (
    <span
      className={cn(
        "inline-flex items-center text-2xs uppercase tracking-[0.14em] px-1.5 h-5 rounded border bg-white/[0.015]",
        tone,
      )}
    >
      {value}
    </span>
  );
}

