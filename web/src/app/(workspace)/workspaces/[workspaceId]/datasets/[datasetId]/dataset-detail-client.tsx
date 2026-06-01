"use client";

import type {
  Dataset,
  DatasetBaseline,
  DatasetExample,
  DatasetRegressionSuiteLink,
  DatasetVersion,
} from "@/lib/api/types";
import { Badge } from "@/components/ui/badge";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export function DatasetDetailClient({
  dataset,
  examples,
  versions,
  baselines,
  regressionLink,
  workspaceId,
}: {
  dataset: Dataset;
  examples: DatasetExample[];
  versions: DatasetVersion[];
  baselines: DatasetBaseline[];
  regressionLink?: DatasetRegressionSuiteLink;
  workspaceId: string;
}) {
  return (
    <div>
      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-lg font-semibold tracking-tight">{dataset.name}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{dataset.slug}</p>
        </div>
        <Badge variant={dataset.input_schema_enforced ? "default" : "secondary"}>
          {dataset.input_schema_enforced ? "schema enforced" : "schema optional"}
        </Badge>
      </div>

      <Tabs defaultValue="examples">
        <TabsList>
          <TabsTrigger value="examples">Examples</TabsTrigger>
          <TabsTrigger value="versions">Versions</TabsTrigger>
          <TabsTrigger value="ci">Regression / CI</TabsTrigger>
        </TabsList>
        <TabsContent value="examples" className="mt-4">
          <div className="rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>External ID</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Source</TableHead>
                  <TableHead>Tags</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {examples.map((example) => (
                  <TableRow key={example.id}>
                    <TableCell className="font-medium">
                      {example.external_id ?? example.id.slice(0, 8)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={example.status === "active" ? "default" : "secondary"}>
                        {example.status}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {example.source}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {example.tags.length > 0 ? example.tags.join(", ") : "None"}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {new Date(example.created_at).toLocaleDateString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </TabsContent>
        <TabsContent value="versions" className="mt-4">
          <div className="rounded-lg border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Version</TableHead>
                  <TableHead>Label</TableHead>
                  <TableHead>Examples</TableHead>
                  <TableHead>Checksum</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {versions.map((version) => (
                  <TableRow key={version.id}>
                    <TableCell className="font-medium">
                      v{version.version_number}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {version.label ?? "Unlabeled"}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {version.example_count}
                    </TableCell>
                    <TableCell className="max-w-[260px] truncate text-muted-foreground">
                      {version.manifest_checksum}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {new Date(version.created_at).toLocaleDateString()}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </TabsContent>
        <TabsContent value="ci" className="mt-4 space-y-6">
          <div className="rounded-lg border border-border p-4">
            <h2 className="text-sm font-semibold">Regression suite link</h2>
            {regressionLink ? (
              <div className="mt-3 space-y-2 text-sm text-muted-foreground">
                <p>
                  Linked suite:{" "}
                  <a
                    className="font-medium text-foreground underline-offset-4 hover:underline"
                    href={`/workspaces/${workspaceId}/regression-suites/${regressionLink.regression_suite_id}`}
                  >
                    {regressionLink.regression_suite_id.slice(0, 8)}
                  </a>
                </p>
                {regressionLink.synced_version_id ? (
                  <p>Last synced version: {regressionLink.synced_version_id.slice(0, 8)}</p>
                ) : null}
                <p>Updated {new Date(regressionLink.updated_at).toLocaleString()}</p>
              </div>
            ) : (
              <p className="mt-3 text-sm text-muted-foreground">
                No regression suite linked yet. Use{" "}
                <code className="rounded bg-muted px-1 py-0.5">agentclash dataset sync-regression-suite</code>{" "}
                to promote examples into a suite for CI runs.
              </p>
            )}
          </div>
          <div className="rounded-lg border border-border">
            <div className="border-b border-border px-4 py-3">
              <h2 className="text-sm font-semibold">Baselines</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Recorded pass rates from completed dataset eval runs used by CI gates.
              </p>
            </div>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Label</TableHead>
                  <TableHead>Pass rate</TableHead>
                  <TableHead>Challenge</TableHead>
                  <TableHead>Run</TableHead>
                  <TableHead>Created</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {baselines.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={5} className="text-muted-foreground">
                      No baselines recorded yet.
                    </TableCell>
                  </TableRow>
                ) : (
                  baselines.map((baseline) => (
                    <TableRow key={baseline.id}>
                      <TableCell className="font-medium">
                        {baseline.label ?? baseline.id.slice(0, 8)}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {baseline.pass_rate != null
                          ? `${Math.round(baseline.pass_rate * 1000) / 10}%`
                          : "—"}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {baseline.challenge_key}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        <a
                          className="underline-offset-4 hover:underline"
                          href={`/workspaces/${workspaceId}/runs/${baseline.run_id}`}
                        >
                          {baseline.run_id.slice(0, 8)}
                        </a>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(baseline.created_at).toLocaleDateString()}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}
