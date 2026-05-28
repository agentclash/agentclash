"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ArrowRight, Terminal } from "lucide-react";
import { getTryCliApiBase } from "@/lib/try-cli/config";
import type { DemoMeta } from "@/lib/try-cli/types";

export function TryCliLandingClient() {
  const [demos, setDemos] = useState<DemoMeta[]>([]);
  const apiBase = getTryCliApiBase();

  useEffect(() => {
    fetch(`${apiBase}/demos`)
      .then((r) => r.json())
      .then(setDemos)
      .catch(() => setDemos([]));
  }, [apiBase]);

  return (
    <div className="mx-auto max-w-4xl px-6 py-16">
      <div className="inline-flex items-center gap-2 rounded-full border border-border px-3 py-1 text-xs text-muted-foreground">
        <Terminal className="size-3.5" />
        AgentClash primitive · E2B sandboxes
      </div>
      <h1 className="mt-6 text-3xl font-semibold tracking-tight md:text-4xl">
        Try any CLI in your browser before you install it
      </h1>
      <p className="mt-4 max-w-2xl text-muted-foreground">
        Interactive README demos for developer tools. One badge, disposable Linux terminal,
        pre-installed CLIs — powered by AgentClash and E2B.
      </p>
      <pre className="mt-6 overflow-x-auto rounded-lg border border-border bg-muted/30 p-4 text-sm">
        npx @agentclash/try-cli init && npx @agentclash/try-cli publish
      </pre>

      <section className="mt-12">
        <h2 className="text-sm font-medium uppercase tracking-wide text-muted-foreground">
          Live demos
        </h2>
        <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {demos.map((d) => (
            <Link
              key={d.slug}
              href={`/try/${d.slug}`}
              className="group rounded-lg border border-border p-4 transition-colors hover:border-foreground/30 hover:bg-muted/20"
            >
              <span className="font-medium">{d.name}</span>
              <span className="mt-1 flex items-center gap-1 text-xs text-muted-foreground group-hover:text-foreground">
                Open terminal <ArrowRight className="size-3" />
              </span>
            </Link>
          ))}
        </div>
      </section>

      <section className="mt-12 rounded-lg border border-border p-6">
        <h2 className="font-medium">For maintainers</h2>
        <ol className="mt-3 list-decimal space-y-2 pl-5 text-sm text-muted-foreground">
          <li>Add <code className="text-foreground">.trycli.yml</code> to your repo</li>
          <li>Run <code className="text-foreground">npx @agentclash/try-cli publish</code></li>
          <li>Users click your badge → real terminal on AgentClash</li>
        </ol>
        <Link href="/docs/concepts/try-cli" className="mt-4 inline-block text-sm underline">
          Read the docs →
        </Link>
      </section>
    </div>
  );
}
