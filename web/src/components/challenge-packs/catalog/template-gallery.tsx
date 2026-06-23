"use client";

import { useMemo, useState } from "react";

import type { CatalogPack } from "../lib/types";
import { CATALOG_CATEGORIES, UNCATEGORIZED_LABEL } from "./catalog-taxonomy";
import { TemplateCard } from "./template-card";
import { TemplateDetail } from "./template-detail";

/**
 * Browsable grid of catalog templates, grouped by category. Filtering is
 * in-memory (the catalog is a small, global, static list) and grouping is driven
 * purely by each pack's `category`, so growing the catalog needs no UI changes.
 */
export function TemplateGallery({
  workspaceId,
  packs,
}: {
  workspaceId: string;
  packs: CatalogPack[];
}) {
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState<CatalogPack | null>(null);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return packs;
    return packs.filter((p) =>
      [p.name, p.description ?? "", p.family, ...(p.tags ?? [])].some((s) =>
        s.toLowerCase().includes(q),
      ),
    );
  }, [packs, query]);

  const groups = useMemo(
    () =>
      CATALOG_CATEGORIES.map((cat) => ({
        ...cat,
        packs: filtered.filter((p) => p.category === cat.key),
      })).filter((g) => g.packs.length > 0),
    [filtered],
  );

  const uncategorized = useMemo(
    () => filtered.filter((p) => !CATALOG_CATEGORIES.some((c) => c.key === p.category)),
    [filtered],
  );

  return (
    <div className="space-y-8">
      <input
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Search templates…"
        className="w-full max-w-sm rounded-md border border-border bg-transparent px-3 py-1.5 text-sm outline-none focus:border-foreground/40"
      />

      {filtered.length === 0 ? (
        <p className="text-sm text-muted-foreground">No templates match your search.</p>
      ) : (
        <>
          {groups.map((group) => (
            <section key={group.key} className="space-y-3">
              <div>
                <h2 className="text-sm font-semibold">{group.label}</h2>
                <p className="text-xs text-muted-foreground">{group.description}</p>
              </div>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {group.packs.map((pack) => (
                  <TemplateCard key={pack.slug} pack={pack} onSelect={() => setSelected(pack)} />
                ))}
              </div>
            </section>
          ))}

          {uncategorized.length > 0 && (
            <section className="space-y-3">
              <h2 className="text-sm font-semibold">{UNCATEGORIZED_LABEL}</h2>
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                {uncategorized.map((pack) => (
                  <TemplateCard key={pack.slug} pack={pack} onSelect={() => setSelected(pack)} />
                ))}
              </div>
            </section>
          )}
        </>
      )}

      <TemplateDetail
        workspaceId={workspaceId}
        pack={selected}
        open={selected !== null}
        onOpenChange={(open) => {
          if (!open) setSelected(null);
        }}
      />
    </div>
  );
}
