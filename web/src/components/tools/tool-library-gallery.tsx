"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Check, FlaskConical, KeyRound, Loader2, PencilRuler, Plus, Search, Zap } from "lucide-react";

import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { useApiListQuery, useApiMutator } from "@/lib/api/swr";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { PageHeader } from "@/components/ui/page-header";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

import type { ToolLibraryEntry, ToolRecord } from "./lib/types";

interface FromLibraryResult {
  items: ToolRecord[];
  skipped: { slug: string; reason: string }[];
}

export function ToolLibraryGallery({ workspaceId }: { workspaceId: string }) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const { mutateMany } = useApiMutator();

  const { data, error, isLoading } = useApiListQuery<ToolLibraryEntry>("/v1/tool-library");
  const { data: toolsData } = useApiListQuery<ToolRecord>(`/v1/workspaces/${workspaceId}/tools`);

  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState<string | null>(null);

  const entries = useMemo(() => data?.items ?? [], [data]);
  const addedNames = useMemo(
    () => new Set((toolsData?.items ?? []).map((t) => t.name.toLowerCase())),
    [toolsData],
  );

  const groups = useMemo(() => {
    const q = query.trim().toLowerCase();
    const match = (e: ToolLibraryEntry) =>
      !q ||
      e.name.toLowerCase().includes(q) ||
      e.description.toLowerCase().includes(q) ||
      e.category.toLowerCase().includes(q) ||
      e.tags.some((t) => t.toLowerCase().includes(q));

    // Group by category, preserving the catalog's first-seen category order.
    const map = new Map<string, ToolLibraryEntry[]>();
    for (const e of entries) {
      if (!match(e)) continue;
      const arr = map.get(e.category);
      if (arr) arr.push(e);
      else map.set(e.category, [e]);
    }
    return [...map.entries()];
  }, [entries, query]);

  const customHref = `/workspaces/${workspaceId}/tools/new?build=canvas`;

  async function add(slugs: string[], busyKey: string) {
    if (slugs.length === 0) return;
    setBusy(busyKey);
    try {
      const api = createApiClient((await getAccessToken()) ?? undefined);
      const res = await api.post<FromLibraryResult>(
        `/v1/workspaces/${workspaceId}/tools/from-library`,
        { entries: slugs.map((slug) => ({ slug })) },
      );
      await mutateMany([workspaceResourceKeys.tools(workspaceId)]);
      const added = res.items?.length ?? 0;
      const skipped = res.skipped?.length ?? 0;
      if (added === 0) {
        toast.info(skipped > 0 ? "Already in your workspace" : "Nothing to add");
        return;
      }
      if (slugs.length === 1 && added === 1) {
        toast.success(`Added "${res.items[0].name}"`);
        router.push(`/workspaces/${workspaceId}/tools/${res.items[0].id}`);
        return;
      }
      toast.success(`Added ${added} tool${added === 1 ? "" : "s"}${skipped ? `, skipped ${skipped} already added` : ""}`);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to add tool");
    } finally {
      setBusy(null);
    }
  }

  if (isLoading && !data) return <WorkspaceListLoading rows={6} />;

  return (
    <div>
      <PageHeader
        breadcrumbs={[
          { label: "Tools", href: `/workspaces/${workspaceId}/tools` },
          { label: "Add a tool" },
        ]}
        title="Add a tool"
        description="Pick a ready-made tool and it's added to your workspace, configured and ready. Or build your own from scratch."
        actions={
          <Button variant="outline" render={<Link href={customHref} />}>
            <PencilRuler data-icon="inline-start" className="size-4" />
            Build a custom tool
          </Button>
        }
      />

      <div className="relative mb-5 max-w-md">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
        <input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Search tools — e.g. slack, search, pdf, sql"
          aria-label="Search the tool library"
          className="h-9 w-full rounded-lg border border-border bg-background pl-9 pr-3 text-sm outline-none placeholder:text-muted-foreground/70 focus:border-ring focus:ring-3 focus:ring-ring/30"
        />
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load the tool library.
        </div>
      ) : groups.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No tools match “{query}”. Try a different search, or{" "}
          <Link href={customHref} className="text-foreground underline underline-offset-4">
            build a custom tool
          </Link>
          .
        </p>
      ) : (
        <div className="space-y-8">
          {groups.map(([category, items]) => {
            const addable = items.filter((e) => !addedNames.has(e.name.toLowerCase())).map((e) => e.slug);
            const packKey = `pack:${category}`;
            return (
              <section key={category}>
                <div className="mb-2.5 flex items-center justify-between gap-3">
                  <h2 className="text-sm font-medium tracking-tight text-muted-foreground">
                    {category}
                    <span className="ml-2 text-xs text-muted-foreground/60">{items.length}</span>
                  </h2>
                  {addable.length > 1 && (
                    <Button
                      variant="ghost"
                      size="xs"
                      onClick={() => add(addable, packKey)}
                      disabled={busy !== null}
                    >
                      {busy === packKey ? (
                        <Loader2 className="size-3.5 animate-spin" />
                      ) : (
                        <Plus data-icon="inline-start" className="size-3.5" />
                      )}
                      Add all {addable.length}
                    </Button>
                  )}
                </div>
                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                  {items.map((entry) => (
                    <LibraryCard
                      key={entry.slug}
                      entry={entry}
                      added={addedNames.has(entry.name.toLowerCase())}
                      busy={busy === entry.slug}
                      disabled={busy !== null}
                      onAdd={() => add([entry.slug], entry.slug)}
                    />
                  ))}
                </div>
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}

function LibraryCard({
  entry,
  added,
  busy,
  disabled,
  onAdd,
}: {
  entry: ToolLibraryEntry;
  added: boolean;
  busy: boolean;
  disabled: boolean;
  onAdd: () => void;
}) {
  return (
    <div className="flex flex-col rounded-lg border border-border bg-card p-4 transition-colors hover:border-foreground/20">
      <div className="font-medium tracking-tight">{entry.name}</div>
      <p className="mt-1 line-clamp-2 flex-1 text-sm text-muted-foreground">{entry.description}</p>

      <div className="mt-3 flex flex-wrap items-center gap-1.5">
        {entry.delivery === "live" ? (
          <Badge variant="outline" className="gap-1 text-[11px]">
            <Zap className="size-3" />
            Runs live
          </Badge>
        ) : (
          <Badge variant="secondary" className="gap-1 text-[11px]">
            <FlaskConical className="size-3" />
            Mock
          </Badge>
        )}
        {entry.requires_secret && (
          <Badge variant="outline" className="gap-1 text-[11px] text-muted-foreground">
            <KeyRound className="size-3" />
            Needs API key
          </Badge>
        )}
      </div>

      <div className="mt-3">
        <Button
          variant={added ? "ghost" : "outline"}
          size="sm"
          className={cn("w-full", added && "text-muted-foreground")}
          onClick={onAdd}
          disabled={disabled || added}
        >
          {busy ? (
            <Loader2 className="size-3.5 animate-spin" />
          ) : added ? (
            <>
              <Check data-icon="inline-start" className="size-3.5" />
              Added
            </>
          ) : (
            <>
              <Plus data-icon="inline-start" className="size-3.5" />
              Add to workspace
            </>
          )}
        </Button>
      </div>
    </div>
  );
}
