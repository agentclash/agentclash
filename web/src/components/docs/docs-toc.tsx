import type { DocHeading } from "@/lib/docs";

export function DocsToc({ headings }: { headings: DocHeading[] }) {
  if (headings.length === 0) return null;

  return (
    <aside className="hidden xl:block">
      <div className="sticky top-8 rounded-[28px] border border-white/[0.08] bg-white/[0.03] p-5">
        <p className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.18em] text-white/30">
          On This Page
        </p>
        <div className="mt-4 space-y-2">
          {headings.map((heading) => (
            <a
              key={heading.id}
              href={`#${heading.id}`}
              className={`block text-sm leading-6 text-white/52 transition-colors hover:text-white ${
                heading.level === 3 ? "pl-4" : ""
              }`}
            >
              {heading.text}
            </a>
          ))}
        </div>
      </div>
    </aside>
  );
}
