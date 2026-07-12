import type { BenchmarkResult } from "@/lib/benchmarks";
import { cn } from "@/lib/utils";

type Props = {
  results: BenchmarkResult[];
  featuredModel?: string;
};

// 0–1 score → "NN" on a 0–100 scale, or an em dash when missing.
function formatScore(value: number | null): string {
  if (value === null) return "—";
  return Math.round(value * 100).toString();
}

function formatCost(value: number | null): string {
  if (value === null) return "—";
  // Cost-per-correct is an absolute dollar amount that can fall below a cent as
  // models get cheaper; toFixed(2) alone would flatten e.g. 0.0042 to "$0.00".
  if (value > 0 && value < 0.01) return `$${value.toPrecision(2)}`;
  return `$${value.toFixed(2)}`;
}

// Restrict columns to numeric (number | null) fields. This excludes string/
// boolean keys like `model`/`provider`/`winner`, so adding a non-numeric column
// is a compile error instead of a silent NaN at render time.
type NumericResultKey = {
  [K in keyof BenchmarkResult]: BenchmarkResult[K] extends number | null
    ? K
    : never;
}[keyof BenchmarkResult];

type Column = {
  key: NumericResultKey;
  label: string;
  format: (value: number | null) => string;
};

const COLUMNS: Column[] = [
  { key: "composite", label: "Composite", format: formatScore },
  { key: "correctness", label: "Correctness", format: formatScore },
  { key: "reliability", label: "Reliability", format: formatScore },
  { key: "latency", label: "Latency", format: formatScore },
  { key: "cost", label: "Cost", format: formatScore },
  { key: "costPerCorrectUsd", label: "$/correct", format: formatCost },
];

export function BenchmarkScoreboard({ results, featuredModel }: Props) {
  if (results.length === 0) return null;

  // A report may omit a metric for every row (e.g. latency was not measured, or
  // the report deliberately does not assert dollar prices). Rendering those
  // columns anyway produces a wall of em dashes, so drop any column whose values
  // are null across the board and let the table narrow to what was measured.
  const visibleColumns = COLUMNS.filter((col) =>
    results.some((result) => result[col.key] !== null),
  );

  // Keep the horizontal-scroll floor proportional to the visible columns so a
  // narrowed table is not forced to scroll on small screens for absent metrics.
  const minWidthClass =
    visibleColumns.length > 4 ? "min-w-[680px]" : "min-w-[520px]";

  return (
    <div className="w-full overflow-x-auto rounded-xl border border-white/[0.08] bg-white/[0.02]">
      <table
        className={cn("w-full border-collapse text-left", minWidthClass)}
      >
        <thead>
          <tr className="border-b border-white/[0.08]">
            <th className="px-4 py-3 font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.14em] text-white/35">
              #
            </th>
            <th className="px-4 py-3 font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.14em] text-white/35">
              Model
            </th>
            {visibleColumns.map((col) => (
              <th
                key={col.key}
                className="px-4 py-3 text-right font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.14em] text-white/35"
              >
                {col.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {results.map((result) => {
            const isFeatured =
              !!featuredModel && result.model === featuredModel;
            return (
              <tr
                key={`${result.rank}-${result.model}`}
                className={cn(
                  "border-b border-white/[0.05] last:border-b-0 transition-colors",
                  result.winner ? "bg-white/[0.04]" : "hover:bg-white/[0.02]",
                )}
              >
                <td className="px-4 py-3 font-[family-name:var(--font-mono)] text-sm text-white/45">
                  {result.rank}
                </td>
                <td className="px-4 py-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <span
                      className={cn(
                        "text-sm font-medium",
                        isFeatured ? "text-white" : "text-white/85",
                      )}
                    >
                      {result.model}
                    </span>
                    {result.winner && (
                      <span className="rounded-full border border-emerald-400/30 bg-emerald-400/10 px-2 py-0.5 font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.12em] text-emerald-300">
                        Winner
                      </span>
                    )}
                  </div>
                  {result.provider && (
                    <span className="mt-0.5 block text-2xs text-white/35">
                      {result.provider}
                    </span>
                  )}
                </td>
                {visibleColumns.map((col) => (
                  <td
                    key={col.key}
                    className="px-4 py-3 text-right font-[family-name:var(--font-mono)] text-sm tabular-nums text-white/70"
                  >
                    {col.format(result[col.key])}
                  </td>
                ))}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
