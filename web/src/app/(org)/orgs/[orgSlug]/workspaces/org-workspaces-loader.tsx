"use client";

import type { OrgWorkspace } from "@/lib/api/types";
import { useOrgContext } from "../org-context";
import { useOrgFetch } from "../use-org-fetch";
import { OrgWorkspacesClient } from "./org-workspaces-client";
import { Loader2 } from "lucide-react";

export function OrgWorkspacesLoader() {
  const { orgId, isAdmin } = useOrgContext();
  const { data: workspaces, total, loading } = useOrgFetch<OrgWorkspace>(
    `/v1/organizations/${orgId}/workspaces`,
  );

  if (loading || !workspaces) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <OrgWorkspacesClient
      orgId={orgId}
      isAdmin={isAdmin}
      initialWorkspaces={workspaces}
      initialTotal={total}
    />
  );
}
