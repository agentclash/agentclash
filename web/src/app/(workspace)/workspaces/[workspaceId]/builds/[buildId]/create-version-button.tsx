"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { AgentBuildVersion } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { Loader2, Plus } from "lucide-react";

interface CreateVersionButtonProps {
  buildId: string;
  workspaceId: string;
}

export function CreateVersionButton({
  buildId,
  workspaceId,
}: CreateVersionButtonProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [creating, setCreating] = useState(false);

  async function handleCreate() {
    setCreating(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const version = await api.post<AgentBuildVersion>(
        `/v1/agent-builds/${buildId}/versions`,
        {
          agent_kind: "llm_agent",
          interface_spec: {},
          policy_spec: { instructions: "" },
        },
      );
      toast.success(`Created version ${version.version_number}`);
      router.push(
        `/workspaces/${workspaceId}/builds/${buildId}/versions/${version.id}`,
      );
    } catch (err) {
      if (err instanceof ApiError) {
        toast.error(err.message);
      } else {
        toast.error("Failed to create version");
      }
    } finally {
      setCreating(false);
    }
  }

  return (
    <Button size="sm" onClick={handleCreate} disabled={creating}>
      {creating ? (
        <Loader2 className="size-4 animate-spin" />
      ) : (
        <>
          <Plus data-icon="inline-start" className="size-4" />
          New Version
        </>
      )}
    </Button>
  );
}
