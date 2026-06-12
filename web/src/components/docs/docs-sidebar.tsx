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
        <div className="flex cursor-not-allowed items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium text-white/25">
          <MessageSquare className="size-4" />
          Community
        </div>
        <Link
          href="/blog"
          className="flex items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium text-white/45 transition-colors hover:bg-white/[0.04] hover:text-white/80"
        >
          <BookOpen className="size-4" />
          Blog
        </Link>
        <Link
          href="/changelog"
          className="flex items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium text-white/45 transition-colors hover:bg-white/[0.04] hover:text-white/80"
        >
          <Map className="size-4" />
          Changelog
        </Link>
        <a
          href="https://github.com/agentclash/agentclash"
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-3 rounded-md px-2 py-1.5 text-sm font-medium text-white/45 transition-colors hover:bg-white/[0.04] hover:text-white/80"
        >
          <Github className="size-4" />
          GitHub
        </a>
      </div>

      <div className="space-y-6">
        {sections.map((section) => (
          <div key={section.title} className="px-1">
            <h4 className="mb-2 px-2 text-2xs font-semibold uppercase tracking-[0.16em] text-white/35">
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
                        ? "bg-white/[0.06] font-medium text-white/90"
                        : "text-white/45 hover:bg-white/[0.04] hover:text-white/80"
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
