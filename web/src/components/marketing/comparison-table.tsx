import { ComparisonMark, type MarkKind } from "./comparison-mark";

export type ComparisonColumn = {
  name: string;
  tag: string;
  highlight?: boolean;
};

export type ComparisonRow = {
  label: string;
  sub: string;
  cells: MarkKind[];
};

type Props = {
  columns: ComparisonColumn[];
  rows: ComparisonRow[];
};

export function ComparisonTable({ columns, rows }: Props) {
  const gridTemplate = `grid-cols-[1.7fr_repeat(${columns.length},minmax(0,1fr))]`;

  return (
    <div>
      {/* Mobile: stacked per-capability cards */}
      <div className="md:hidden space-y-10">
        {rows.map((row) => (
          <div
            key={`m-${row.label}`}
            className="border-b border-white/[0.08] pb-8 last:border-b-0"
          >
            <p className="text-[16px] font-medium text-white/90 leading-[1.35]">
              {row.label}
            </p>
            <p className="mt-2 text-[13px] leading-[1.55] text-white/40">
              {row.sub}
            </p>
            <dl className="mt-5 grid grid-cols-1 gap-0">
              {columns.map((col, j) => (
                <div
                  key={col.name}
                  className={`flex items-center justify-between gap-4 border-b border-white/[0.05] py-2.5 last:border-b-0 ${
                    col.highlight ? "bg-white/[0.025] -mx-3 px-3 rounded" : ""
                  }`}
                >
                  <div className="flex flex-col">
                    <dt
                      className={`text-[13px] ${
                        col.highlight
                          ? "text-white/95 font-medium"
                          : "text-white/60"
                      }`}
                    >
                      {col.name}
                    </dt>
                    <span
                      className={`text-[9px] font-[family-name:var(--font-mono)] uppercase tracking-[0.2em] ${
                        col.highlight ? "text-white/45" : "text-white/25"
                      }`}
                    >
                      {col.tag}
                    </span>
                  </div>
                  <dd>
                    <ComparisonMark
                      kind={row.cells[j]}
                      highlight={col.highlight}
                    />
                  </dd>
                </div>
              ))}
            </dl>
          </div>
        ))}
      </div>

      {/* Desktop: full matrix */}
      <div className="hidden md:block -mx-8 sm:mx-0 overflow-x-auto">
        <div className="min-w-[1040px] px-8 sm:px-0">
          <div className={`grid ${gridTemplate} border-b border-white/[0.12]`}>
            <div className="pb-5 pr-4">
              <p className="text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.18em] text-white/35">
                Capability
              </p>
            </div>
            {columns.map((col) => (
              <div
                key={col.name}
                className="flex flex-col items-center justify-end gap-1 pb-5 px-2"
              >
                <span
                  className={`text-center leading-tight ${
                    col.highlight
                      ? "text-[13px] font-[family-name:var(--font-display)] tracking-[-0.01em] text-white/95"
                      : "text-[12px] font-[family-name:var(--font-mono)] uppercase tracking-[0.16em] text-white/45"
                  }`}
                >
                  {col.name}
                </span>
                <span
                  className={`text-[9px] font-[family-name:var(--font-mono)] uppercase tracking-[0.2em] ${
                    col.highlight ? "text-white/30" : "text-white/25"
                  }`}
                >
                  {col.tag}
                </span>
              </div>
            ))}
          </div>

          {rows.map((row) => (
            <div
              key={row.label}
              className={`grid ${gridTemplate} border-b border-white/[0.05] last:border-b-0`}
            >
              <div className="py-7 pr-6">
                <p className="text-[15px] text-white/85">{row.label}</p>
                <p className="mt-1.5 text-[12px] leading-[1.5] text-white/40">
                  {row.sub}
                </p>
              </div>
              {row.cells.map((mark, j) => (
                <div
                  key={j}
                  className={`flex items-center justify-center py-7 px-2 ${
                    columns[j]?.highlight ? "bg-white/[0.025]" : ""
                  }`}
                >
                  <ComparisonMark
                    kind={mark}
                    highlight={columns[j]?.highlight ?? false}
                  />
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>

      <p className="mt-10 text-[12px] font-[family-name:var(--font-mono)] text-white/35">
        <span className="text-white/60">●</span>&nbsp;&nbsp;supported
        &nbsp;·&nbsp; <span className="text-white/45">◐</span>
        &nbsp;&nbsp;partial &nbsp;·&nbsp;
        <span className="text-white/30">—</span>&nbsp;&nbsp;not a core capability
      </p>
    </div>
  );
}
