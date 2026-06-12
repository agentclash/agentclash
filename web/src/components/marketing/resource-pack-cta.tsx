import Link from "next/link";
import { PRIMARY_RESOURCE, RESOURCE_LIBRARY } from "@/lib/resource-library";

type Props = {
  className?: string;
  compact?: boolean;
};

export function ResourcePackCTA({ className = "", compact = false }: Props) {
  return (
    <aside
      className={`rounded-lg border border-white/[0.1] bg-white/[0.035] p-5 sm:p-6 ${className}`}
    >
      <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.14em] text-white/40">
        Free resource pack
      </p>
      <h2 className="mt-3 text-lg font-sans font-semibold tracking-tight text-white">
        {PRIMARY_RESOURCE.title}
      </h2>
      <p className="mt-2 text-sm leading-6 text-white/50">
        Download the flagship checklist plus {RESOURCE_LIBRARY.length - 1}{" "}
        companion worksheets for release gates, pilots, procurement, and your
        first 30 days of agent evals.
      </p>
      {!compact ? (
        <ul className="mt-4 grid gap-2 text-sm text-white/55 sm:grid-cols-2">
          {RESOURCE_LIBRARY.slice(0, 4).map((resource) => (
            <li key={resource.slug}>{resource.kicker}</li>
          ))}
        </ul>
      ) : null}
      <Link
        href="/resources/eval-checklist"
        className="mt-5 inline-flex min-h-11 items-center justify-center rounded-lg bg-white px-5 text-sm font-semibold text-[#060606] transition-colors hover:bg-white/90"
      >
        Download the PDFs
      </Link>
    </aside>
  );
}
