"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { GitCompare, Sparkles } from "lucide-react";

import type { AgentTryout } from "@/lib/api/types";
import { useApiListQuery } from "@/lib/api/swr";
import { WorkspaceListLoading } from "@/components/app-shell/workspace-loading";
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
import { LaunchTryoutDialog } from "./launch-tryout-dialog";
import {
  formatTryoutCost,
  formatTryoutLatency,
  tryoutIsActive,
  tryoutModelLabel,
  tryoutStatusVariant,
} from "./status";

const MAX_COMPARE = 4;

export function AgentTryoutsClient({ workspaceId }: { workspaceId: string }) {
  const router = useRouter();
  const [selected, setSelected] = useState<string[]>([]);

  const { data, error, isLoading } = useApiListQuery<AgentTryout>(
    `/v1/workspaces/${workspaceId}/agent-tryouts`,
    undefined,
    {
      refreshInterval: (response) =>
        response?.items.some((item) => tryoutIsActive(item.status)) ? 2500 : 0,
    },
  );
  const items = data?.items ?? [];

  function toggleSelected(id: string) {
    setSelected((current) =>
      current.includes(id)
        ? current.filter((value) => value !== id)
        : current.length < MAX_COMPARE
          ? [...current, id]
          : current,
    );
  }

  if (isLoading && !data) {
    return <WorkspaceListLoading rows={6} />;
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">Agent Tryouts</h1>
          <p className="text-sm text-muted-foreground">
            Hand an agent a real office task, watch it work, then compare models
            and promote the winner to a repeatable eval.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {selected.length >= 2 ? (
            <Button
              size="sm"
              variant="outline"
              onClick={() =>
                router.push(
                  `/workspaces/${workspaceId}/agent-tryouts/compare?ids=${selected.join(",")}`,
                )
              }
            >
              <GitCompare data-icon="inline-start" className="size-4" />
              Compare ({selected.length})
            </Button>
          ) : null}
          <LaunchTryoutDialog workspaceId={workspaceId} />
        </div>
      </div>

      {error ? (
        <div className="rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          Failed to load agent tryouts.
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<Sparkles className="size-10" />}
          title="No tryouts yet"
          description="Launch a tryout to see an agent draft a slide deck, build a spreadsheet, triage an inbox, or write a status report — with evals attached."
        />
      ) : (
        <div className="rounded-lg border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-8">
                  <span className="sr-only">Select for comparison</span>
                </TableHead>
                <TableHead>Template</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Model</TableHead>
                <TableHead>Cost</TableHead>
                <TableHead>Latency</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((tryout) => (
                <TableRow key={tryout.id}>
                  <TableCell>
                    <input
                      type="checkbox"
                      aria-label="Select for comparison"
                      checked={selected.includes(tryout.id)}
                      disabled={
                        !selected.includes(tryout.id) &&
                        selected.length >= MAX_COMPARE
                      }
                      onChange={() => toggleSelected(tryout.id)}
                    />
                  </TableCell>
                  <TableCell className="font-medium">
                    <Link
                      href={`/workspaces/${workspaceId}/agent-tryouts/${tryout.id}`}
                      className="hover:underline"
                    >
                      {tryout.template_slug}
                    </Link>
                    {tryout.parent_tryout_id ? (
                      <Badge variant="outline" className="ml-2">
                        rerun
                      </Badge>
                    ) : null}
                  </TableCell>
                  <TableCell>
                    <Badge variant={tryoutStatusVariant(tryout.status)}>
                      {tryout.status}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {tryoutModelLabel(tryout)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatTryoutCost(tryout)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatTryoutLatency(tryout.latency_ms)}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {new Date(tryout.created_at).toLocaleString()}
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
