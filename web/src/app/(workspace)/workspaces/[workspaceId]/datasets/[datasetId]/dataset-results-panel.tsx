"use client";

import { useCallback, useEffect, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2 } from "lucide-react";

import { listDatasetResults } from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { DatasetEvalResult, DatasetVersion } from "@/lib/api/types";
import { EmptyState } from "@/components/ui/empty-state";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const inputClass =
  "rounded-lg border border-input bg-transparent px-3 py-1.5 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface DatasetResultsPanelProps {
  workspaceId: string;
  datasetId: string;
  versions: DatasetVersion[];
}

export function DatasetResultsPanel({
  workspaceId,
  datasetId,
  versions,
}: DatasetResultsPanelProps) {
  const { getAccessToken } = useAccessToken();
  const [results, setResults] = useState<DatasetEvalResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [versionFilter, setVersionFilter] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const res = await listDatasetResults(api, workspaceId, datasetId, {
        versionId: versionFilter || undefined,
        limit: 100,
      });
      setResults(res.items);
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to load eval results",
      );
    } finally {
      setLoading(false);
    }
  }, [datasetId, getAccessToken, versionFilter, workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <label className="text-sm text-muted-foreground">Version filter</label>
        <select
          value={versionFilter}
          onChange={(e) => setVersionFilter(e.target.value)}
          className={inputClass}
        >
          <option value="">All versions</option>
          {versions.map((v) => (
            <option key={v.id} value={v.id}>
              v{v.version_number}
              {v.label ? ` — ${v.label}` : ""}
            </option>
          ))}
        </select>
      </div>

      {loading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : results.length === 0 ? (
        <EmptyState
          title="No eval results"
          description="Run a dataset eval to see per-example verdicts here."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Example</TableHead>
                <TableHead>Verdict</TableHead>
                <TableHead>Score</TableHead>
                <TableHead>Run</TableHead>
                <TableHead>Judged</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {results.map((result) => (
                <TableRow key={`${result.dataset_example_id}-${result.run_id}`}>
                  <TableCell className="font-medium">
                    {result.dataset_example_id.slice(0, 8)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {result.verdict ?? "—"}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {result.normalized_score != null
                      ? result.normalized_score.toFixed(3)
                      : "—"}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {result.run_id ? result.run_id.slice(0, 8) : "—"}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {result.judged_at
                      ? new Date(result.judged_at).toLocaleString()
                      : "—"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
