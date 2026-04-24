import type { ReactNode } from "react";

type Props = {
  eyebrow?: string;
  title: ReactNode;
  body: ReactNode;
  aside?: ReactNode;
  reverse?: boolean;
  id?: string;
};

export function SplitSection({
  eyebrow,
  title,
  body,
  aside,
  reverse = false,
  id,
}: Props) {
  return (
    <section
      id={id}
      className="border-t border-white/[0.06] px-8 sm:px-12 py-28 sm:py-40"
    >
      <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-2 md:gap-20 items-center">
        <div className={reverse ? "md:order-2" : ""}>
          {eyebrow ? (
            <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
              <span className="inline-block size-1 rounded-full bg-white/60" />
              {eyebrow}
            </p>
          ) : null}
          <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2rem,4.5vw,3.75rem)] max-w-[22ch]">
            {title}
          </h2>
          <div className="mt-8 max-w-[52ch] text-[15px] sm:text-lg leading-[1.65] text-white/55">
            {body}
          </div>
        </div>
        <div className={reverse ? "md:order-1" : ""}>{aside}</div>
      </div>
    </section>
  );
}
