"use client";

import { SidebarTrigger } from "@/components/ui/sidebar";
import { Separator } from "@/components/ui/separator";
import { useAuthStore } from "@/lib/stores/auth";

export function Topbar() {
  const { activeWorkspaceId } = useAuthStore();

  return (
    <header className="flex h-12 shrink-0 items-center gap-2 border-b border-border px-4">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="mr-2 h-4" />
      <div className="flex items-center gap-2">
        <span className="text-xs font-medium text-text-3 font-[family-name:var(--font-mono)]">
          {activeWorkspaceId
            ? `ws:${activeWorkspaceId.slice(0, 8)}`
            : "no workspace"}
        </span>
      </div>
    </header>
  );
}
