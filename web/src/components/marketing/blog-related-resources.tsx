import Link from "next/link";
import type { BlogRelatedResourceLink } from "@/lib/blog-related-resources";

type Props = {
  links: BlogRelatedResourceLink[];
  className?: string;
};

export function BlogRelatedResources({ links, className = "mt-12" }: Props) {
  if (links.length === 0) return null;

  return (
    <section className={className} aria-labelledby="blog-related-resources">
      <p className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.14em] text-white/40">
        Explore
      </p>
      <h2
        id="blog-related-resources"
        className="mt-3 text-xl font-sans font-semibold tracking-[-0.02em] text-white"
      >
        Related resources
      </h2>
      <ul className="mt-6 grid gap-px overflow-hidden rounded-xl border border-white/[0.08] bg-white/[0.08]">
        {links.map((link) => (
          <li key={link.href}>
            <Link
              href={link.href}
              className="flex flex-col bg-[#060606] px-5 py-4 transition-colors hover:bg-white/[0.025] sm:px-6 sm:py-5"
            >
              <span className="text-base font-sans font-semibold text-white">
                {link.label}
              </span>
              <span className="mt-1.5 text-sm leading-6 text-white/45">
                {link.description}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
