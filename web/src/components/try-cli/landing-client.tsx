"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight, ArrowUpRight, Terminal } from "lucide-react";
import { getTryCliApiBase } from "@/lib/try-cli/config";
import type { DemoMeta } from "@/lib/try-cli/types";

const CATEGORY_ORDER = ["AI coding agents", "Developer tools"];

function categoryRank(category: string): number {
  const i = CATEGORY_ORDER.indexOf(category);
  return i === -1 ? CATEGORY_ORDER.length : i;
}

export function TryCliLandingClient() {
  const [demos, setDemos] = useState<DemoMeta[]>([]);
  const [loaded, setLoaded] = useState(false);
  const apiBase = getTryCliApiBase();

  useEffect(() => {
    fetch(`${apiBase}/demos`)
      .then((r) => r.json())
      .then((d) => setDemos(Array.isArray(d) ? d : []))
      .catch(() => setDemos([]))
      .finally(() => setLoaded(true));
  }, [apiBase]);

  const groups = useMemo(() => {
    const byCat = new Map<string, DemoMeta[]>();
    for (const d of demos) {
      const cat = d.category ?? "Developer tools";
      byCat.set(cat, [...(byCat.get(cat) ?? []), d]);
    }
    return [...byCat.entries()].sort((a, b) => categoryRank(a[0]) - categoryRank(b[0]));
  }, [demos]);

  return (
    <main className="mx-auto max-w-[1400px] px-5 pb-28 sm:px-8">
      {/* Hero */}
      <section className="relative pt-20 sm:pt-28">
        <div
          aria-hidden
          className="pointer-events-none absolute -top-10 left-0 -z-10 h-[340px] w-[680px] max-w-full rounded-full bg-[radial-gradient(ellipse_at_left,rgba(255,255,255,0.08),transparent_70%)] blur-2xl"
        />
        <div className="inline-flex items-center gap-2 rounded-full border border-white/[0.08] bg-white/[0.03] px-3 py-1 text-xs text-white/55">
          <Terminal className="size-3.5" />
          AgentClash primitive · live E2B sandboxes
        </div>
        <h1 className="mt-8 max-w-[18ch] font-[family-name:var(--font-display)] text-[clamp(2.75rem,6.5vw,6rem)] font-normal leading-[0.95] tracking-[-0.04em]">
          Run any CLI.
          <br />
          <span className="text-white/45">In your browser.</span>
        </h1>
        <p className="mt-8 max-w-[52ch] text-lg leading-[1.55] text-white/55">
          Spin up a disposable cloud terminal and try AI coding agents and developer
          tools before you install a thing. No setup, no signup — just type.
        </p>
        <div className="mt-10 flex flex-wrap items-center gap-3">
          <a
            href="#demos"
            className="inline-flex items-center gap-2 rounded-md bg-white px-6 py-3 text-sm font-medium text-[#060606] transition-colors hover:bg-white/90"
          >
            Browse demos
            <ArrowRight className="size-4" />
          </a>
          <Link
            href="/docs/concepts/try-cli"
            className="inline-flex items-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-6 py-3 text-sm text-white/80 transition-colors hover:border-white/25 hover:text-white"
          >
            How it works
          </Link>
        </div>
      </section>

      {/* Demos */}
      <section id="demos" className="mt-24 scroll-mt-24">
        {!loaded && (
          <div className="grid gap-px overflow-hidden rounded-xl border border-white/[0.08] bg-white/[0.06] sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="h-[132px] animate-pulse bg-[#0a0a0a]" />
            ))}
          </div>
        )}

        {loaded && demos.length === 0 && (
          <p className="text-sm text-white/40">No demos available right now.</p>
        )}

        <div className="space-y-16">
          {groups.map(([category, items]) => (
            <div key={category}>
              <div className="mb-5 flex items-baseline justify-between">
                <h2 className="font-[family-name:var(--font-display)] text-2xl tracking-[-0.02em] text-white/90">
                  {category}
                </h2>
                <span className="text-xs text-white/35">{items.length} demos</span>
              </div>
              <div className="grid gap-px overflow-hidden rounded-xl border border-white/[0.08] bg-white/[0.06] sm:grid-cols-2 lg:grid-cols-3">
                {items.map((d) => (
                  <Link
                    key={d.slug}
                    href={`/try/${d.slug}`}
                    className="group relative flex min-h-[132px] flex-col justify-between bg-[#0a0a0a] p-5 transition-all duration-200 hover:bg-white/[0.03] hover:shadow-[inset_0_0_0_1px_rgba(255,255,255,0.12)]"
                  >
                    <div>
                      <div className="flex items-center justify-between">
                        <span className="font-[family-name:var(--font-display)] text-xl tracking-[-0.01em] text-white">
                          {d.name}
                        </span>
                        <ArrowUpRight className="size-4 text-white/25 transition-all group-hover:translate-x-0.5 group-hover:-translate-y-0.5 group-hover:text-white/70" />
                      </div>
                      {d.tagline && (
                        <p className="mt-2 text-sm leading-snug text-white/45">{d.tagline}</p>
                      )}
                    </div>
                    <span className="mt-4 inline-flex items-center gap-1.5 text-xs text-white/35 transition-colors group-hover:text-white/70">
                      Open terminal
                    </span>
                  </Link>
                ))}
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* Maintainers */}
      <section className="mt-28 grid gap-12 border-t border-white/[0.06] pt-16 md:grid-cols-[1fr_1.1fr] md:gap-20">
        <div>
          <h2 className="font-[family-name:var(--font-display)] text-[clamp(1.75rem,3vw,2.75rem)] font-normal leading-[1.05] tracking-[-0.03em]">
            Ship a live demo of <span className="text-white/45">your</span> CLI.
          </h2>
          <p className="mt-6 max-w-[44ch] text-white/55">
            Drop a <code className="font-[family-name:var(--font-mono)] text-white/80">.trycli.yml</code>{" "}
            in your repo, publish a badge, and let anyone try your tool in one click —
            running on AgentClash sandboxes.
          </p>
          <Link
            href="/docs/concepts/try-cli"
            className="mt-8 inline-flex items-center gap-1.5 text-sm text-white/80 underline-offset-4 transition-colors hover:text-white hover:underline"
          >
            Read the docs <ArrowRight className="size-3.5" />
          </Link>
        </div>
        <div className="rounded-xl border border-white/[0.08] bg-white/[0.02] p-6">
          <ol className="space-y-5">
            {[
              { k: "01", t: "Add a config", d: "npx @agentclash/try-cli init" },
              { k: "02", t: "Publish a badge", d: "npx @agentclash/try-cli publish" },
              { k: "03", t: "Users click → real terminal", d: "No install. Just your tool, live." },
            ].map((s) => (
              <li key={s.k} className="flex gap-4">
                <span className="font-[family-name:var(--font-mono)] text-sm text-white/30">{s.k}</span>
                <div>
                  <div className="text-sm font-medium text-white/90">{s.t}</div>
                  <div className="mt-1 font-[family-name:var(--font-mono)] text-xs text-white/45">{s.d}</div>
                </div>
              </li>
            ))}
          </ol>
        </div>
      </section>
    </main>
  );
}
