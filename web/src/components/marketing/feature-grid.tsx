import type { ReactNode } from "react";

export type FeatureItem = {
  label: string;
  title: string;
  body: string;
  glyph?: ReactNode;
};

type Props = {
  features: FeatureItem[];
  columns?: 2 | 3 | 4;
};

const gridCols: Record<2 | 3 | 4, string> = {
  2: "sm:grid-cols-2",
  3: "sm:grid-cols-2 lg:grid-cols-3",
  4: "sm:grid-cols-2 lg:grid-cols-4",
};

export function FeatureGrid({ features, columns = 3 }: Props) {
  return (
    <ul
      className={`grid grid-cols-1 ${gridCols[columns]} gap-px border-y border-white/[0.06] bg-white/[0.06]`}
    >
      {features.map((feature) => (
        <li
          key={feature.label}
          className="group relative flex flex-col bg-[#060606] px-8 py-12 transition-colors hover:bg-white/[0.015]"
        >
          {feature.glyph ? (
            <div className="inline-flex size-12 items-center justify-center rounded-full border border-white/[0.12] bg-white/[0.02] transition-colors group-hover:border-white/25">
              {feature.glyph}
            </div>
          ) : (
            <div className="inline-flex size-12 items-center justify-center rounded-full border border-white/[0.12] bg-white/[0.02]">
              <span className="block size-1.5 rounded-full bg-white/70" />
            </div>
          )}

          <p className="mt-8 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.2em] text-white/40">
            {feature.label}
          </p>

          <h3 className="mt-3 font-[family-name:var(--font-display)] text-2xl leading-[1.15] tracking-[-0.02em] text-white/95">
            {feature.title}
          </h3>

          <p className="mt-4 text-[14px] leading-[1.65] text-white/55">
            {feature.body}
          </p>
        </li>
      ))}
    </ul>
  );
}
