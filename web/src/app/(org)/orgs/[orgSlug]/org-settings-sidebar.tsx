"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";
import { Settings2, Users, Layers, ArrowLeft } from "lucide-react";

interface OrgSettingsSidebarProps {
  orgSlug: string;
  orgName: string;
  isAdmin: boolean;
}

const navItems = (orgSlug: string, isAdmin: boolean) => [
  ...(isAdmin
    ? [
        {
          label: "General",
          href: `/orgs/${orgSlug}/settings`,
          icon: Settings2,
        },
      ]
    : []),
  {
    label: "Members",
    href: `/orgs/${orgSlug}/members`,
    icon: Users,
  },
  {
    label: "Workspaces",
    href: `/orgs/${orgSlug}/workspaces`,
    icon: Layers,
  },
];

export function OrgSettingsSidebar({
  orgSlug,
  orgName,
  isAdmin,
}: OrgSettingsSidebarProps) {
  const pathname = usePathname();
  const items = navItems(orgSlug, isAdmin);

  return (
    <aside className="w-56 shrink-0 border-r border-white/[0.06] bg-[#0a0a0a] p-4 flex flex-col">
      <Link
        href="/dashboard"
        className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors mb-6"
      >
        <ArrowLeft className="size-3" />
        Back to dashboard
      </Link>

      <div className="mb-4">
        <h2 className="text-sm font-semibold truncate">{orgName}</h2>
        <p className="text-xs text-muted-foreground">Organization settings</p>
      </div>

      <nav className="space-y-0.5">
        {items.map((item) => {
          const isActive = pathname === item.href;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors",
                isActive
                  ? "bg-white/[0.08] text-foreground font-medium"
                  : "text-muted-foreground hover:text-foreground hover:bg-white/[0.04]",
              )}
            >
              <item.icon className="size-4" />
              {item.label}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
