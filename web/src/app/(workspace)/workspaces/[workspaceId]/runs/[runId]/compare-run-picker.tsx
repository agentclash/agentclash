"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { createApiClient } from "@/lib/api/client";
import type { Run } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogTrigger,
} from "@/components/ui/dialog";
import { GitCompare, Loader2 } from "lucide-react";
import { runStatusVariant } from "../status-variant";

interface CompareRunPickerProps {
  currentRunId: string;
  workspaceId: string;
}

export function CompareRunPicker({
  currentRunId,
  workspaceId,
}: CompareRunPickerProps) {
  const { getAccessToken } = useAccessToken();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(false);

  const fetchRuns = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await api.get<{
        items: Run[];
        total: number;
      }>(`/v1/workspaces/${workspaceId}/runs`, {
        params: { limit: 50, offset: 0 },
      });
      setRuns(res.items.filter((r) => r.id !== currentRunId));
    } catch {
      // Silently fail
    } finally {
      setLoading(false);
    }
  }, [getAccessToken, workspaceId, currentRunId]);

  useEffect(() => {
    if (open) fetchRuns();
  }, [open, fetchRuns]);

  function handleSelect(candidateId: string) {
    setOpen(false);
    router.push(
      `/workspaces/${workspaceId}/compare?baseline=${currentRunId}&candidate=${candidateId}`,
    );
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button variant="outline" size="sm" />
        }
      >
        <GitCompare className="size-4 mr-1.5" />
        Compare with&hellip;
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Compare with another run</DialogTitle>
          <DialogDescription>
            Select a run to compare against the current run.
          </DialogDescription>
        </DialogHeader>
        <div className="max-h-64 overflow-y-auto -mx-4 px-4">
          {loading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : runs.length === 0 ? (
            <p className="text-sm text-muted-foreground py-4 text-center">
              No other runs found in this workspace.
            </p>
          ) : (
            <div className="space-y-1">
              {runs.map((run) => (
                <button
                  key={run.id}
                  onClick={() => handleSelect(run.id)}
                  className="w-full flex items-center justify-between rounded-md px-3 py-2 text-sm hover:bg-muted/50 transition-colors text-left"
                >
                  <span className="font-medium truncate mr-2">{run.name}</span>
                  <Badge
                    variant={runStatusVariant[run.status] ?? "outline"}
                  >
                    {run.status}
                  </Badge>
                </button>
              ))}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
