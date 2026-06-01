"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import {
  listDatasetTraceCandidates,
  promoteDatasetTraceCandidate,
} from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { DatasetTraceCandidate } from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

interface TraceCandidatesPanelProps {
  workspaceId: string;
  datasetId: string;
}

export function TraceCandidatesPanel({
  workspaceId,
  datasetId,
}: TraceCandidatesPanelProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const [candidates, setCandidates] = useState<DatasetTraceCandidate[]>([]);
  const [loading, setLoading] = useState(true);
  const [promotingId, setPromotingId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await listDatasetTraceCandidates(
        api,
        workspaceId,
        datasetId,
        { limit: 100 },
      );
      setCandidates(res.candidates);
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to load trace candidates",
      );
    } finally {
      setLoading(false);
    }
  }, [datasetId, getAccessToken, workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function handlePromote(candidateId: string) {
    setPromotingId(candidateId);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await promoteDatasetTraceCandidate(
        api,
        workspaceId,
        datasetId,
        candidateId,
      );
      toast.success("Candidate promoted to example");
      await load();
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Promotion failed");
    } finally {
      setPromotingId(null);
    }
  }

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (candidates.length === 0) {
    return (
      <EmptyState
        title="No trace candidates"
        description="Import traces to review and promote them into dataset examples."
      />
    );
  }

  return (
    <div className="rounded-lg border border-border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Platform</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>External ID</TableHead>
            <TableHead>Tags</TableHead>
            <TableHead>Created</TableHead>
            <TableHead className="w-24" />
          </TableRow>
        </TableHeader>
        <TableBody>
          {candidates.map((candidate) => (
            <TableRow key={candidate.id}>
              <TableCell className="text-muted-foreground">
                {candidate.source_platform}
              </TableCell>
              <TableCell>
                <Badge
                  variant={
                    candidate.status === "promoted" ? "default" : "secondary"
                  }
                >
                  {candidate.status}
                </Badge>
              </TableCell>
              <TableCell className="font-medium">
                {candidate.external_id ?? candidate.id.slice(0, 8)}
              </TableCell>
              <TableCell className="text-muted-foreground">
                {candidate.tags.length > 0 ? candidate.tags.join(", ") : "—"}
              </TableCell>
              <TableCell className="text-muted-foreground">
                {new Date(candidate.created_at).toLocaleDateString()}
              </TableCell>
              <TableCell>
                {candidate.status === "pending" && (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={promotingId === candidate.id}
                    onClick={() => handlePromote(candidate.id)}
                  >
                    {promotingId === candidate.id ? (
                      <Loader2 className="size-3.5 animate-spin" />
                    ) : (
                      "Promote"
                    )}
                  </Button>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
