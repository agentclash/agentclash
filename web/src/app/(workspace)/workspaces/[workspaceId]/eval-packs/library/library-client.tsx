"use client";

import { LibraryBig } from "lucide-react";

import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
import { TemplateGallery } from "@/components/eval-packs/catalog/template-gallery";
import { catalogPath } from "@/components/eval-packs/lib/api";
import type { CatalogPack } from "@/components/eval-packs/lib/types";
import { EmptyState } from "@/components/ui/empty-state";
import { useApiListQuery } from "@/lib/api/swr";

export function LibraryClient({ workspaceId }: { workspaceId: string }) {
  const { data, error, isLoading } = useApiListQuery<CatalogPack>(catalogPath());
  const packs = data?.items ?? [];

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-lg font-semibold tracking-tight">Pack Library</h1>
        <p className="mt-0.5 text-sm text-muted-foreground">
          Ready-to-run eval packs. Add one to your workspace, run it, then customize it in the
          builder.
        </p>
      </div>

      {isLoading && !data ? (
        <WorkspaceListLoading rows={6} />
      ) : error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load the pack library.
        </div>
      ) : packs.length === 0 ? (
        <EmptyState icon={<LibraryBig className="size-10" />} title="No templates available" />
      ) : (
        <TemplateGallery workspaceId={workspaceId} packs={packs} />
      )}
    </div>
  );
}
