import Link from "next/link";
import type { Metadata } from "next";
import { MarketingShell } from "@/components/marketing/marketing-shell";
import { markdownAlternate } from "@/lib/seo";

type Page = {
  path: string;
  title: string;
  description: string;
  sections: readonly {
    heading: string;
    body: string;
    links?: readonly { label: string; href: string }[];
  }[];
};

export function factualMetadata(page: Page): Metadata {
  return {
    title: page.title,
    description: page.description,
    alternates: { canonical: page.path, types: markdownAlternate(page.path) },
  };
}

export function FactualPage({ page }: { page: Page }) {
  return (
    <MarketingShell>
      <main className="mx-auto max-w-3xl px-6 py-20 text-white">
        <h1 className="font-sans text-4xl font-semibold tracking-tight">
          {page.title.split(" | ")[0]}
        </h1>
        <p className="mt-5 leading-7 text-white/65">{page.description}</p>
        {page.sections.map((section) => (
          <section key={section.heading} className="mt-12">
            <h2 className="text-2xl font-semibold tracking-tight">{section.heading}</h2>
            <p className="mt-4 leading-7 text-white/65">{section.body}</p>
            {section.links && (
              <ul className="mt-4 space-y-2">
                {section.links.map((link) => (
                  <li key={link.href}>
                    <Link className="text-cyan-200 underline" href={link.href}>{link.label}</Link>
                  </li>
                ))}
              </ul>
            )}
          </section>
        ))}
      </main>
    </MarketingShell>
  );
}
