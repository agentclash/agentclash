"use client";

import Link from "next/link";
import { MessageSquare, BookOpen, Map, Github } from "lucide-react";
import type { DocNavSection } from "@/lib/docs";

export function DocsSidebar({
  sections,
  currentHref,
}: {
  sections: DocNavSection[];
  currentHref: string;
}) {
  return (
    <div className="flex flex-col gap-8 pb-10">
      <div className="space-y-2 px-1">
        <div className="flex cursor-not-allowed items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium text-zinc-600">
          <MessageSquare className="size-4" />
          Community
        </div>
        <Link href="/blog" className="flex items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium text-zinc-400 transition-colors hover:bg-zinc-800/50 hover:text-zinc-100">
          <BookOpen className="size-4" />
          Blog
        </Link>
        <div className="flex cursor-not-allowed items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium text-zinc-600">
          <Map className="size-4" />
          Roadmap
        </div>
        <a href="https://github.com/agentclash/agentclash" target="_blank" rel="noopener noreferrer" className="flex items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium text-zinc-400 transition-colors hover:bg-zinc-800/50 hover:text-zinc-100">
          <Github className="size-4" />
          GitHub
        </a>
      </div>

      <div className="space-y-6">
        {sections.map((section) => (
          <div key={section.title} className="px-1">
            <h4 className="mb-2 px-2 text-xs font-semibold text-zinc-100">
              {section.title}
            </h4>
            <div className="space-y-0.5">
              {section.items.map((item) => {
                const active = currentHref === item.href;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={`block rounded-md px-2 py-1.5 text-sm transition-colors ${
                      active
                        ? "bg-emerald-500/10 font-medium text-emerald-400"
                        : "text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-100"
                    }`}
                  >
                    {item.title}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
