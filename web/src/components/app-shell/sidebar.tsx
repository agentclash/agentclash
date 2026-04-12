"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { navSections } from "./nav-items";
import { PanelLeftClose, PanelLeft } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useState } from "react";

interface SidebarProps {
  workspaceId: string;
}

function LogoMark({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      className={className}
      aria-hidden="true"
    >
      <path
        d="M7 4L12 12L7 20"
        stroke="currentColor"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M17 4L12 12L17 20"
        stroke="currentColor"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        opacity="0.4"
      />
    </svg>
  );
}

function SidebarNav({
  workspaceId,
  collapsed,
}: SidebarProps & { collapsed: boolean }) {
  const pathname = usePathname();

  return (
    <nav className={cn("flex-1 overflow-y-auto py-3", collapsed ? "px-1.5" : "px-2.5")}>
      {navSections.map((section) => (
        <div key={section.title} className="mb-5">
          {!collapsed && (
            <p className="mb-1 px-2 text-[0.625rem] font-semibold uppercase tracking-[0.08em] text-muted-foreground/50">
              {section.title}
            </p>
          )}
          <div className="space-y-0.5">
            {section.items.map((item) => {
              const href = item.href(workspaceId);
              const isActive = pathname.startsWith(href);
              const Icon = item.icon;

              const link = (
                <Link
                  key={item.label}
                  href={href}
                  className={cn(
                    "group relative flex items-center rounded-md text-[0.8125rem] transition-colors",
                    collapsed
                      ? "justify-center p-2"
                      : "gap-2.5 px-2 py-1.5",
                    isActive
                      ? "bg-white/[0.08] text-foreground"
                      : "text-muted-foreground hover:bg-white/[0.04] hover:text-foreground/80",
                  )}
                >
                  {isActive && (
                    <span className="absolute left-0 top-1/2 h-4 w-[2px] -translate-y-1/2 rounded-full bg-foreground" />
                  )}
                  <Icon className={cn("shrink-0", collapsed ? "size-[18px]" : "size-4")} />
                  {!collapsed && <span>{item.label}</span>}
                </Link>
              );

              if (collapsed) {
                return (
                  <Tooltip key={item.label}>
                    <TooltipTrigger render={<span />}>
                      {link}
                    </TooltipTrigger>
                    <TooltipContent side="right" sideOffset={8}>
                      {item.label}
                    </TooltipContent>
                  </Tooltip>
                );
              }

              return link;
            })}
          </div>
        </div>
      ))}
    </nav>
  );
}

/** Desktop sidebar — collapsible */
export function Sidebar({ workspaceId }: SidebarProps) {
  const [collapsed, setCollapsed] = useState(() => {
    if (typeof window === "undefined") return false;
    return localStorage.getItem("sidebar-collapsed") === "true";
  });

  function toggle() {
    setCollapsed((prev) => {
      localStorage.setItem("sidebar-collapsed", String(!prev));
      return !prev;
    });
  }

  return (
    <TooltipProvider>
      <aside
        className={cn(
          "hidden md:flex md:flex-col md:border-r md:border-white/[0.06] bg-[#0a0a0a] transition-[width] duration-200",
          collapsed ? "md:w-14" : "md:w-56",
        )}
      >
        {/* Header */}
        <div
          className={cn(
            "flex h-14 items-center border-b border-white/[0.06]",
            collapsed ? "justify-center px-1.5" : "justify-between px-3",
          )}
        >
          <Link
            href={`/workspaces/${workspaceId}`}
            className="flex items-center gap-2 text-foreground/90"
          >
            <LogoMark className="size-6 text-foreground" />
            {!collapsed && (
              <span className="font-[family-name:var(--font-display)] text-[0.9375rem] tracking-tight">
                AgentClash
              </span>
            )}
          </Link>
          {!collapsed && (
            <button
              onClick={toggle}
              className="rounded-md p-1 text-muted-foreground/40 hover:text-muted-foreground transition-colors"
            >
              <PanelLeftClose className="size-4" />
            </button>
          )}
        </div>

        {/* Expand button when collapsed */}
        {collapsed && (
          <div className="flex justify-center py-2">
            <button
              onClick={toggle}
              className="rounded-md p-1.5 text-muted-foreground/40 hover:text-muted-foreground transition-colors"
            >
              <PanelLeft className="size-4" />
            </button>
          </div>
        )}

        <SidebarNav workspaceId={workspaceId} collapsed={collapsed} />
      </aside>
    </TooltipProvider>
  );
}

/** Mobile sidebar — sheet */
export function MobileSidebar({ workspaceId }: SidebarProps) {
  const [open, setOpen] = useState(false);

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger
        render={<Button variant="ghost" size="icon" className="md:hidden" />}
      >
        <PanelLeft className="size-5" />
      </SheetTrigger>
      <SheetContent side="left" className="w-56 p-0 bg-[#0a0a0a]">
        <SheetTitle className="sr-only">Navigation</SheetTitle>
        <div onClick={() => setOpen(false)}>
          {/* Logo */}
          <div className="flex h-14 items-center gap-2 px-3 border-b border-white/[0.06]">
            <LogoMark className="size-6 text-foreground" />
            <span className="font-[family-name:var(--font-display)] text-[0.9375rem] tracking-tight text-foreground/90">
              AgentClash
            </span>
          </div>
          <SidebarNav workspaceId={workspaceId} collapsed={false} />
        </div>
      </SheetContent>
    </Sheet>
  );
}
