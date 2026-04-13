"use client";

import type { OrgMember } from "@/lib/api/types";
import { useOrgContext } from "../org-context";
import { useOrgFetch } from "../use-org-fetch";
import { OrgMembersClient } from "./org-members-client";
import { Loader2 } from "lucide-react";

export function OrgMembersLoader() {
  const { orgId, isAdmin, currentUserId } = useOrgContext();
  const { data: members, total, loading } = useOrgFetch<OrgMember>(
    `/v1/organizations/${orgId}/memberships`,
  );

  if (loading || !members) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <OrgMembersClient
      orgId={orgId}
      isAdmin={isAdmin}
      currentUserId={currentUserId}
      initialMembers={members}
      initialTotal={total}
    />
  );
}
