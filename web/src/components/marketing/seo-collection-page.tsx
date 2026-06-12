import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { ClashMark } from "@/components/marketing/clash-mark";
import { JsonLd, breadcrumbSchema } from "@/components/marketing/json-ld";
import type { SeoPageConfig } from "@/lib/seo-pages/types";

type SecondarySection = {
  title: string;
  intro?: string;
  pages: SeoPageConfig[];
};

type SeoCollectionPageProps = {
  path: string;
  title: string;
  description: string;
  eyebrow: string;
  h1: string;
  intro: string;
  pages: SeoPageConfig[];
  secondarySections?: SecondarySection[];
  schemaId: string;
};

export function createSeoCollectionMetadata({
  path,
  title,
  description,
}: Pick<SeoCollectionPageProps, "path" | "title" | "description">): Metadata {
  return {
    title,
    description,
    alternates: {
      canonical: path,
    },
    openGraph: {
      title,
      description,
      url: path,
      type: "website",
      locale: "en_US",
      siteName: "AgentClash",
      images: [
        {
          url: "/og-image.png",
          width: 1200,
          height: 630,
          alt: `${title} social preview.`,
        },
      ],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
      images: [
        {
          url: "/twitter-image.png",
          alt: `${title} social preview.`,
        },
      ],
    },
  };
}

export function SeoCollectionPage({
  path,
  title,
  description,
  eyebrow,
  h1,
  intro,
  pages,
  secondarySections = [],
  schemaId,
}: SeoCollectionPageProps) {
  return (
    <>
      <JsonLd
        id={schemaId}
        data={[
          breadcrumbSchema([
            { name: "Home", url: "/" },
            { name: eyebrow, url: path },
          ]),
        ]}
      />
      <header className="border-b border-white/[0.06] px-5 py-5 sm:px-12 sm:py-6">
        <div className="mx-auto flex max-w-[1440px] items-center justify-between gap-4">
          <Link
            href="/"
            className="inline-flex items-center gap-2.5 text-white/90"
          >
            <ClashMark className="size-6" />
            <span className="font-[family-name:var(--font-display)] text-xl tracking-normal">
              AgentClash
            </span>
          </Link>
          <Link
            href="/auth/login"
            className="inline-flex items-center gap-1.5 rounded-md bg-white px-3 py-1.5 text-xs font-medium text-[#060606] transition-colors hover:bg-white/90"
          >
            Start
            <ArrowRight className="size-3.5" />
          </Link>
        </div>
      </header>
      <main className="min-h-screen bg-[#060606] px-6 py-20 text-white sm:px-12 sm:py-28">
        <div className="mx-auto max-w-[960px]">
          <nav className="flex items-center gap-2 text-xs text-white/35">
            <Link href="/" className="transition-colors hover:text-white/70">
              Home
            </Link>
            <span>/</span>
            <span>{eyebrow}</span>
          </nav>
          <p className="mt-10 font-[family-name:var(--font-mono)] text-2xs uppercase tracking-normal text-cyan-200/70">
            {eyebrow}
          </p>
          <h1 className="mt-5 font-[family-name:var(--font-display)] text-5xl font-normal leading-none tracking-normal text-white sm:text-6xl">
            {h1}
          </h1>
          <p className="mt-8 max-w-[58ch] text-base leading-8 text-white/62 sm:text-lg">
            {intro}
          </p>
          <p className="sr-only">{description}</p>
          <div className="mt-12 grid gap-3">
            {pages.map((page) => (
              <Link
                key={page.path}
                href={page.path}
                className="group rounded-md border border-white/[0.08] bg-white/[0.03] p-5 transition-colors hover:border-white/20 hover:bg-white/[0.05]"
              >
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <h2 className="text-base font-semibold tracking-normal text-white">
                      {page.sitemapTitle}
                    </h2>
                    <p className="mt-2 text-sm leading-6 text-white/50">
                      {page.sitemapDescription}
                    </p>
                  </div>
                  <ArrowRight className="size-4 shrink-0 text-white/35 transition-colors group-hover:text-white/75" />
                </div>
              </Link>
            ))}
          </div>
          {secondarySections.map((section) => (
            <section key={section.title} className="mt-16">
              <h2 className="text-xl font-semibold tracking-tight text-white">
                {section.title}
              </h2>
              {section.intro ? (
                <p className="mt-3 max-w-[58ch] text-sm leading-7 text-white/55">
                  {section.intro}
                </p>
              ) : null}
              <div className="mt-6 grid gap-3">
                {section.pages.map((page) => (
                  <Link
                    key={page.path}
                    href={page.path}
                    className="group rounded-md border border-white/[0.08] bg-white/[0.03] p-5 transition-colors hover:border-white/20 hover:bg-white/[0.05]"
                  >
                    <div className="flex items-center justify-between gap-4">
                      <div>
                        <h3 className="text-base font-semibold tracking-normal text-white">
                          {page.sitemapTitle}
                        </h3>
                        <p className="mt-2 text-sm leading-6 text-white/50">
                          {page.sitemapDescription}
                        </p>
                      </div>
                      <ArrowRight className="size-4 shrink-0 text-white/35 transition-colors group-hover:text-white/75" />
                    </div>
                  </Link>
                ))}
              </div>
            </section>
          ))}
        </div>
      </main>
    </>
  );
}
