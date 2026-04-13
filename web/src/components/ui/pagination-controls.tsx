"use client";

import { Button } from "@/components/ui/button";
import { ChevronLeft, ChevronRight } from "lucide-react";

interface PaginationControlsProps {
  offset: number;
  total: number;
  pageSize: number;
  onPrev: () => void;
  onNext: () => void;
}

export function PaginationControls({
  offset,
  total,
  pageSize,
  onPrev,
  onNext,
}: PaginationControlsProps) {
  const page = Math.floor(offset / pageSize) + 1;
  const totalPages = Math.ceil(total / pageSize);

  if (totalPages <= 1) return null;

  return (
    <div className="flex items-center justify-between">
      <p className="text-sm text-muted-foreground">
        Page {page} of {totalPages}
      </p>
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="icon-sm"
          disabled={offset === 0}
          onClick={onPrev}
        >
          <ChevronLeft className="size-4" />
        </Button>
        <Button
          variant="outline"
          size="icon-sm"
          disabled={offset + pageSize >= total}
          onClick={onNext}
        >
          <ChevronRight className="size-4" />
        </Button>
      </div>
    </div>
  );
}
