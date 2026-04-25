"use client";

import { useRouter, usePathname } from "next/navigation";
import { ChevronsUpDown, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { UserMeWorkspace, UserMeOrganization } from "@/lib/api/types";

interface WorkspaceSwitcherProps {
  currentWorkspaceId: string;
  organizations: UserMeOrganization[];
}

export function WorkspaceSwitcher({
  currentWorkspaceId,
  organizations,
}: WorkspaceSwitcherProps) {
  const router = useRouter();
  const pathname = usePathname();

  function getWorkspacePath(workspaceId: string) {
    return pathname.replace(
      /\/workspaces\/[^/]+/,
      `/workspaces/${workspaceId}`,
    );
  }

  const allWorkspaces: (UserMeWorkspace & { orgName: string })[] = [];
  for (const org of organizations) {
    for (const ws of org.workspaces) {
      allWorkspaces.push({ ...ws, orgName: org.name });
    }
  }

  const current = allWorkspaces.find((ws) => ws.id === currentWorkspaceId);

  function switchWorkspace(workspaceId: string) {
    const newPath = getWorkspacePath(workspaceId);
    router.push(newPath);
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="sm"
            className="gap-1.5 max-w-52 text-foreground/80 hover:text-foreground"
          />
        }
      >
        <span className="truncate text-[0.8125rem]">
          {current?.name ?? "Select workspace"}
        </span>
        <ChevronsUpDown className="size-3 opacity-40" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-60">
        {allWorkspaces.map((ws) => (
          <DropdownMenuItem
            key={ws.id}
            onClick={() => switchWorkspace(ws.id)}
            onMouseEnter={() => void router.prefetch(getWorkspacePath(ws.id))}
            onFocus={() => void router.prefetch(getWorkspacePath(ws.id))}
            className="flex items-center justify-between py-2"
          >
            <div className="flex flex-col gap-0.5">
              <span className="text-sm">{ws.name}</span>
              <span className="text-[0.6875rem] text-muted-foreground/60">
                {ws.orgName}
              </span>
            </div>
            {ws.id === currentWorkspaceId && (
              <Check className="size-3.5 text-foreground/60" />
            )}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
