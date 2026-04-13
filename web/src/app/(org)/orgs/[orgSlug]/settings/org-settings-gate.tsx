"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type { Organization } from "@/lib/api/types";
import { useOrgContext } from "../org-context";
import { OrgGeneralSettings } from "./org-general-settings";
import { Loader2, AlertTriangle } from "lucide-react";

export function OrgSettingsGate({ orgSlug }: { orgSlug: string }) {
  const { orgId, isAdmin } = useOrgContext();
  const { getAccessToken } = useAccessToken();
  const router = useRouter();
  const [org, setOrg] = useState<Organization | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isAdmin) {
      router.replace(`/orgs/${orgSlug}/members`);
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const token = await getAccessToken();
        if (!token) return;
        const api = createApiClient(token);
        const data = await api.get<Organization>(
          `/v1/organizations/${orgId}`,
        );
        if (!cancelled) setOrg(data);
      } catch {
        if (!cancelled) setError("Failed to load organization settings.");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [orgId, isAdmin, orgSlug, router, getAccessToken]);

  if (!isAdmin) return null;

  if (error) {
    return (
      <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-6 text-center text-sm text-destructive flex items-center justify-center gap-2">
        <AlertTriangle className="size-4" />
        {error}
      </div>
    );
  }

  if (!org) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="size-5 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div>
      <h1 className="text-lg font-semibold tracking-tight mb-6">
        General Settings
      </h1>
      <OrgGeneralSettings org={org} onOrgUpdated={setOrg} orgSlug={orgSlug} />
    </div>
  );
}
