"use client";

import { useMemo, useState } from "react";
import { Check, Search } from "lucide-react";
import { cn } from "@/lib/utils";
import { controlClass } from "./field";
import { operationGroupLabel, operationLabel } from "./lib/friendly";
import type { ToolPrimitive } from "./lib/types";

/**
 * Picks the base operation a tool performs. Renders each primitive as a card
 * with a friendly name and its description, grouped by kind — far easier to scan
 * than a bare <select> of names like `http_request`. Filterable when long.
 */
export function OperationPicker({
  primitives,
  selected,
  onSelect,
}: {
  primitives: ToolPrimitive[];
  selected: string;
  onSelect: (name: string) => void;
}) {
  const [query, setQuery] = useState("");

  const groups = useMemo(() => {
    const q = query.trim().toLowerCase();
    const filtered = primitives.filter((p) => {
      if (!q) return true;
      return (
        p.name.toLowerCase().includes(q) ||
        operationLabel(p.name).toLowerCase().includes(q) ||
        p.description.toLowerCase().includes(q)
      );
    });
    const byKind = new Map<string, ToolPrimitive[]>();
    for (const p of filtered) {
      if (!byKind.has(p.kind)) byKind.set(p.kind, []);
      byKind.get(p.kind)!.push(p);
    }
    return [...byKind.entries()];
  }, [primitives, query]);

  return (
    <div className="space-y-2">
      {primitives.length > 6 && (
        <div className="relative">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search operations…"
            className={cn(controlClass, "pl-8")}
          />
        </div>
      )}

      <div className="max-h-80 space-y-3 overflow-auto pr-1">
        {groups.length === 0 ? (
          <p className="rounded-lg border border-dashed border-border p-4 text-center text-xs text-muted-foreground">
            No operations match “{query}”.
          </p>
        ) : (
          groups.map(([kind, items]) => (
            <div key={kind} className="space-y-1.5">
              <div className="text-xs font-medium text-muted-foreground">
                {operationGroupLabel(kind)}
              </div>
              <div className="grid gap-1.5 sm:grid-cols-2">
                {items.map((p) => {
                  const active = p.name === selected;
                  return (
                    <button
                      key={p.name}
                      type="button"
                      onClick={() => onSelect(p.name)}
                      className={cn(
                        "flex flex-col rounded-lg border p-2.5 text-left transition-colors",
                        active
                          ? "border-primary bg-primary/5"
                          : "border-border hover:border-foreground/30",
                      )}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-sm font-medium">{operationLabel(p.name)}</span>
                        {active && <Check className="size-3.5 shrink-0 text-primary" />}
                      </div>
                      <span className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
                        {p.description}
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
