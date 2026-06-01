"use client";

import { useState } from "react";
import { Menu } from "lucide-react";
import type { DocNavSection } from "@/lib/docs";
import { DocsSidebar } from "@/components/docs/docs-sidebar";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";

export function DocsMobileNav({
  sections,
  currentHref,
}: {
  sections: DocNavSection[];
  currentHref: string;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        className="inline-flex items-center gap-2 rounded-xl border border-white/[0.08] px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-white/55 transition-colors hover:border-white/15 hover:text-white/75 lg:hidden"
        aria-label="Open docs navigation"
      >
        <Menu className="size-4" />
        Menu
      </SheetTrigger>
      <SheetContent side="left" className="w-[min(100vw-2rem,20rem)] border-white/[0.08] bg-[#060606] p-0">
        <SheetHeader className="border-b border-white/[0.08] px-4 py-4 text-left">
          <SheetTitle className="text-sm font-semibold text-white/90">
            Documentation
          </SheetTitle>
        </SheetHeader>
        <div className="overflow-y-auto px-3 pt-4" onClick={() => setOpen(false)}>
          <DocsSidebar sections={sections} currentHref={currentHref} />
        </div>
      </SheetContent>
    </Sheet>
  );
}
