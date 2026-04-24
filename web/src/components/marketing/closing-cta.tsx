import type { ReactNode } from "react";

type Props = {
  title: ReactNode;
  body?: ReactNode;
  children: ReactNode;
};

export function ClosingCTA({ title, body, children }: Props) {
  return (
    <section className="border-t border-white/[0.06] px-8 sm:px-12 py-32 sm:py-48">
      <div className="mx-auto max-w-[1440px] grid gap-16 md:grid-cols-[1.3fr_1fr] md:gap-20 items-center">
        <div>
          <h2 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.04em] leading-[0.98] text-[clamp(2.5rem,5.5vw,5rem)] max-w-[16ch]">
            {title}
          </h2>
          {body ? (
            <div className="mt-8 max-w-[52ch] text-[15px] sm:text-lg leading-[1.6] text-white/55">
              {body}
            </div>
          ) : null}
          <div className="mt-10">{children}</div>
        </div>
      </div>
    </section>
  );
}
