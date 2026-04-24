import type { ReactNode } from "react";
import Link from "next/link";
import { ChevronRight } from "lucide-react";

export type Breadcrumb = { label: string; href?: string };

type Props = {
  eyebrow: string;
  title: ReactNode;
  subtitle: ReactNode;
  breadcrumbs?: Breadcrumb[];
  cta?: ReactNode;
  aside?: ReactNode;
};

export function PageHeader({
  eyebrow,
  title,
  subtitle,
  breadcrumbs,
  cta,
  aside,
}: Props) {
  return (
    <section className="px-8 sm:px-12 pt-28 pb-20 sm:pt-40 sm:pb-28 border-b border-white/[0.06]">
      <div className="mx-auto max-w-[1440px]">
        {breadcrumbs && breadcrumbs.length > 0 ? (
          <nav
            aria-label="Breadcrumb"
            className="mb-10 flex flex-wrap items-center gap-1.5 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.18em] text-white/35"
          >
            {breadcrumbs.map((crumb, i) => (
              <span key={`${crumb.label}-${i}`} className="flex items-center gap-1.5">
                {crumb.href ? (
                  <Link
                    href={crumb.href}
                    className="hover:text-white/70 transition-colors"
                  >
                    {crumb.label}
                  </Link>
                ) : (
                  <span className="text-white/55">{crumb.label}</span>
                )}
                {i < breadcrumbs.length - 1 ? (
                  <ChevronRight className="size-3 text-white/25" />
                ) : null}
              </span>
            ))}
          </nav>
        ) : null}

        <div className="grid gap-16 md:grid-cols-[1.5fr_1fr] md:gap-20 items-start">
          <div>
            <p className="mb-6 inline-flex items-center gap-2 text-[11px] font-[family-name:var(--font-mono)] uppercase tracking-[0.22em] text-white/45">
              <span className="inline-block size-1 rounded-full bg-white/60" />
              {eyebrow}
            </p>
            <h1 className="font-[family-name:var(--font-display)] font-normal tracking-[-0.04em] leading-[0.98] text-[clamp(2.75rem,6vw,5.75rem)] max-w-[18ch]">
              {title}
            </h1>
            <div className="mt-10 max-w-[56ch] text-lg sm:text-xl leading-[1.55] text-white/55">
              {subtitle}
            </div>
            {cta ? <div className="mt-10">{cta}</div> : null}
          </div>

          {aside ? (
            <div className="flex items-start justify-center">{aside}</div>
          ) : null}
        </div>
      </div>
    </section>
  );
}
