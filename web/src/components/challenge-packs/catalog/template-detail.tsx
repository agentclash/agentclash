"use client";

import { Badge } from "@/components/ui/badge";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { useApiQuery } from "@/lib/api/swr";
import { catalogPackPath } from "../lib/api";
import type { CatalogPack, CatalogPackDetail } from "../lib/types";
import { SpecCardView } from "../pieces/spec-card-view";
import { UseTemplateButton } from "./use-template-button";

/**
 * Right-side drawer showing a template's readable spec card (rendered instantly
 * from the list summary) plus its runnable YAML (fetched lazily on open) and the
 * Use-template action. Reuses the builder's SpecCardView so the "what it
 * measures" panel is identical everywhere.
 */
export function TemplateDetail({
  workspaceId,
  pack,
  open,
  onOpenChange,
}: {
  workspaceId: string;
  pack: CatalogPack | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { data: detail } = useApiQuery<CatalogPackDetail>(
    pack && open ? catalogPackPath(pack.slug) : null,
  );

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="flex w-full flex-col gap-0 p-0 sm:max-w-xl">
        {pack && (
          <>
            <SheetHeader className="space-y-1.5 border-b border-border px-4 py-3 text-left">
              <SheetTitle>{pack.name}</SheetTitle>
              {pack.description && <SheetDescription>{pack.description}</SheetDescription>}
              <div className="flex flex-wrap gap-1.5 pt-1">
                <Badge variant="outline">{pack.family}</Badge>
                {pack.difficulty && <Badge variant="outline">{pack.difficulty}</Badge>}
                <Badge variant="outline">{pack.execution_mode}</Badge>
              </div>
            </SheetHeader>

            <div className="min-h-0 flex-1">
              <SpecCardView card={pack.spec_card} yaml={detail?.yaml} />
            </div>

            <div className="flex items-center justify-end gap-2 border-t border-border px-4 py-3">
              <UseTemplateButton workspaceId={workspaceId} slug={pack.slug} />
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  );
}
