"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Search, Command } from "lucide-react";
import type { DocSearchItem } from "@/lib/docs";

export function DocsSearch() {
  const [query, setQuery] = useState("");
  const [searchItems, setSearchItems] = useState<DocSearchItem[]>([]);
  const [searchState, setSearchState] = useState<
    "idle" | "loading" | "ready" | "error"
  >("idle");
  const [isOpen, setIsOpen] = useState(false);

  const normalized = query.trim().toLowerCase();
  const tokens = normalized.split(/\s+/).filter(Boolean);
  const matches =
    tokens.length === 0
      ? []
      : searchItems
          .filter((item) =>
            tokens.every((token) => item.searchText.includes(token)),
          )
          .slice(0, 12);

  useEffect(() => {
    if (searchState !== "loading") return;

    let cancelled = false;

    fetch("/docs/search.json", {
      headers: {
        Accept: "application/json",
      },
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`Failed to load docs search index: ${response.status}`);
        }
        return response.json() as Promise<DocSearchItem[]>;
      })
      .then((items) => {
        if (cancelled) return;
        setSearchItems(items);
        setSearchState("ready");
      })
      .catch(() => {
        if (cancelled) return;
        setSearchState("error");
      });

    return () => {
      cancelled = true;
    };
  }, [searchState]);

  const ensureSearchLoaded = () => {
    if (searchState === "idle" || searchState === "error") {
      setSearchState("loading");
    }
  };

  return (
    <div className="relative w-full max-w-[24rem]">
      <div className="relative flex items-center">
        <Search className="pointer-events-none absolute left-3 size-4 text-zinc-400" />
        <input
          value={query}
          onChange={(event) => {
            ensureSearchLoaded();
            setQuery(event.target.value);
            setIsOpen(true);
          }}
          onFocus={() => {
            ensureSearchLoaded();
            setIsOpen(true);
          }}
          onBlur={() => {
            // small delay to allow clicking a result
            setTimeout(() => setIsOpen(false), 200);
          }}
          placeholder="Search AgentClash docs..."
          className="h-9 w-full rounded-md border border-zinc-800 bg-zinc-900/50 pl-10 pr-10 text-sm text-zinc-200 outline-none transition-colors placeholder:text-zinc-500 focus:border-emerald-500/50 focus:bg-zinc-900"
        />
        <div className="pointer-events-none absolute right-3 flex items-center gap-1 text-[10px] text-zinc-500">
          <Command className="size-3" />
          <span>K</span>
        </div>
      </div>

      {isOpen && tokens.length > 0 && (
        <div className="absolute top-12 left-0 right-0 z-50 rounded-lg border border-zinc-800 bg-zinc-900 p-2 shadow-2xl">
          <div className="space-y-1">
            {matches.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className="block rounded-md px-3 py-2 transition-colors hover:bg-zinc-800"
                onClick={() => setIsOpen(false)}
              >
                <span className="block text-sm font-medium text-zinc-200">{item.title}</span>
                <span className="mt-1 block truncate text-xs text-zinc-500">
                  {item.description}
                </span>
              </Link>
            ))}
            {searchState === "loading" && (
              <p className="px-3 py-2 text-sm text-zinc-500">Loading docs index...</p>
            )}
            {searchState === "error" && (
              <p className="px-3 py-2 text-sm text-zinc-500">Search is temporarily unavailable.</p>
            )}
            {searchState === "ready" && matches.length === 0 && (
              <p className="px-3 py-2 text-sm text-zinc-500">No results found.</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}