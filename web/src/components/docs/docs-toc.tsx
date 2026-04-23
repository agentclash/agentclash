import type { DocHeading } from "@/lib/docs";

export function DocsToc({ headings }: { headings: DocHeading[] }) {
  if (headings.length === 0) return null;

  return (
    <aside className="hidden xl:block">
      <div className="sticky top-24 pt-8">
        <p className="mb-4 text-xs font-semibold text-zinc-100">
          On this page
        </p>
        <div className="space-y-3 border-l border-zinc-800/60 pl-4">
          {headings.map((heading) => (
            <a
              key={heading.id}
              href={`#${heading.id}`}
              className={`block text-sm text-zinc-400 transition-colors hover:text-zinc-100 ${
                heading.level === 3 ? "pl-3" : ""
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
