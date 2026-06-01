"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";

import type {
  Dataset,
  DatasetBaseline,
  DatasetExample,
  DatasetRegressionSuiteLink,
  DatasetVersion,
} from "@/lib/api/types";
import { deleteDataset } from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { DeleteResourceButton } from "@/components/infra/delete-resource-button";

import {
  CreateBaselineDialog,
  EvaluateGateDialog,
  SyncRegressionDialog,
} from "./dataset-ci-dialogs";
import { CreateVersionDialog } from "./create-version-dialog";
import { DatasetResultsPanel } from "./dataset-results-panel";
import { EditDatasetDialog } from "./edit-dataset-dialog";
import { ExampleFormDialog } from "./example-form-dialog";
import { ExportDatasetButton } from "./export-dataset-button";
import { ImportDatasetDialog } from "./import-dataset-dialog";
import { ImportTracesDialog } from "./import-traces-dialog";
import { StartEvalDialog } from "./start-eval-dialog";
import { StartGenerationDialog } from "./start-generation-dialog";
import { TraceCandidatesPanel } from "./trace-candidates-panel";

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
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const datasetEndpoint = `/v1/workspaces/${workspaceId}/datasets/${dataset.id}`;

  async function handleArchiveDataset() {
    if (
      !confirm(
        `Archive dataset "${dataset.name}"? It will no longer appear in lists.`,
      )
    ) {
      return;
    }
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      await deleteDataset(api, workspaceId, dataset.id);
      toast.success("Dataset archived");
      router.push(`/workspaces/${workspaceId}/datasets`);
    } catch (err) {
      toast.error(
        err instanceof ApiError ? err.message : "Failed to archive dataset",
      );
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title={dataset.name}
        breadcrumbs={[
          {
            label: "Datasets",
            href: `/workspaces/${workspaceId}/datasets`,
          },
          { label: dataset.name },
        ]}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <EditDatasetDialog workspaceId={workspaceId} dataset={dataset} />
            <ImportDatasetDialog
              workspaceId={workspaceId}
              datasetId={dataset.id}
            />
            <ExportDatasetButton
              workspaceId={workspaceId}
              datasetId={dataset.id}
              versions={versions}
            />
            <StartGenerationDialog
              workspaceId={workspaceId}
              datasetId={dataset.id}
            />
            <StartEvalDialog
              workspaceId={workspaceId}
              datasetId={dataset.id}
              versions={versions}
            />
            <Button size="sm" variant="ghost" onClick={handleArchiveDataset}>
              Archive
            </Button>
          </div>
        }
      />

      <div className="rounded-lg border border-border bg-card/30 p-4 space-y-3">
        {dataset.description ? (
          <p className="text-sm text-muted-foreground">{dataset.description}</p>
        ) : null}
        <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <MetaRow label="Slug">
            <code className="text-xs font-[family-name:var(--font-mono)]">
              {dataset.slug}
            </code>
          </MetaRow>
          <MetaRow label="Examples">{dataset.active_example_count}</MetaRow>
          <MetaRow label="Versions">{dataset.version_count}</MetaRow>
          <MetaRow label="Schema">
            <Badge
              variant={dataset.input_schema_enforced ? "default" : "secondary"}
            >
              {dataset.input_schema_enforced ? "enforced" : "optional"}
            </Badge>
          </MetaRow>
        </dl>
      </div>

      <Tabs defaultValue="examples">
        <TabsList>
          <TabsTrigger value="examples">Examples</TabsTrigger>
          <TabsTrigger value="versions">Versions</TabsTrigger>
          <TabsTrigger value="traces">Traces</TabsTrigger>
          <TabsTrigger value="results">Results</TabsTrigger>
          <TabsTrigger value="ci">Regression / CI</TabsTrigger>
        </TabsList>

        <TabsContent value="examples" className="mt-4 space-y-4">
          <div className="flex justify-end">
            <ExampleFormDialog
              workspaceId={workspaceId}
              datasetId={dataset.id}
            />
          </div>
          {examples.length === 0 ? (
            <EmptyState
              title="No examples"
              description="Add examples manually or import from a file."
            />
          ) : (
            <div className="rounded-lg border border-border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>External ID</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Source</TableHead>
                    <TableHead>Tags</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead className="w-20" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {examples.map((example) => (
                    <TableRow key={example.id}>
                      <TableCell className="font-medium">
                        {example.external_id ?? example.id.slice(0, 8)}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={
                            example.status === "active" ? "default" : "secondary"
                          }
                        >
                          {example.status}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {example.source}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {example.tags.length > 0
                          ? example.tags.join(", ")
                          : "None"}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(example.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1">
                          <ExampleFormDialog
                            workspaceId={workspaceId}
                            datasetId={dataset.id}
                            example={example}
                            trigger="edit"
                          />
                          {example.status === "active" && (
                            <DeleteResourceButton
                              endpoint={`${datasetEndpoint}/examples/${example.id}`}
                              resourceName="Example"
                            />
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </TabsContent>

        <TabsContent value="versions" className="mt-4 space-y-4">
          <div className="flex justify-end">
            <CreateVersionDialog
              workspaceId={workspaceId}
              datasetId={dataset.id}
            />
          </div>
          {versions.length === 0 ? (
            <EmptyState
              title="No versions"
              description="Snapshot active examples into an immutable version."
            />
          ) : (
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
          )}
        </TabsContent>

        <TabsContent value="traces" className="mt-4 space-y-4">
          <div className="flex justify-end gap-2">
            <ImportTracesDialog
              workspaceId={workspaceId}
              datasetId={dataset.id}
            />
          </div>
          <TraceCandidatesPanel
            workspaceId={workspaceId}
            datasetId={dataset.id}
          />
        </TabsContent>

        <TabsContent value="results" className="mt-4">
          <DatasetResultsPanel
            workspaceId={workspaceId}
            datasetId={dataset.id}
            versions={versions}
          />
        </TabsContent>

        <TabsContent value="ci" className="mt-4 space-y-6">
          <div className="flex flex-wrap gap-2">
            <SyncRegressionDialog
              workspaceId={workspaceId}
              datasetId={dataset.id}
              versions={versions}
              regressionLink={regressionLink}
            />
            <CreateBaselineDialog
              workspaceId={workspaceId}
              datasetId={dataset.id}
            />
            <EvaluateGateDialog
              workspaceId={workspaceId}
              datasetId={dataset.id}
              baselines={baselines}
            />
          </div>

          <div className="rounded-lg border border-border p-4">
            <h2 className="text-sm font-semibold">Regression suite link</h2>
            {regressionLink ? (
              <div className="mt-3 space-y-2 text-sm text-muted-foreground">
                <p>
                  Linked suite:{" "}
                  <Link
                    className="font-medium text-foreground underline-offset-4 hover:underline"
                    href={`/workspaces/${workspaceId}/regression-suites/${regressionLink.regression_suite_id}`}
                  >
                    {regressionLink.regression_suite_id.slice(0, 8)}
                  </Link>
                </p>
                {regressionLink.synced_version_id ? (
                  <p>
                    Last synced version:{" "}
                    {regressionLink.synced_version_id.slice(0, 8)}
                  </p>
                ) : null}
                <p>
                  Updated {new Date(regressionLink.updated_at).toLocaleString()}
                </p>
              </div>
            ) : (
              <p className="mt-3 text-sm text-muted-foreground">
                No regression suite linked yet. Use sync to create or update the
                linked suite.
              </p>
            )}
          </div>

          <div className="rounded-lg border border-border">
            <div className="border-b border-border px-4 py-3">
              <h2 className="text-sm font-semibold">Baselines</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Recorded pass rates from completed dataset eval runs used by CI
                gates.
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
                        <Link
                          className="underline-offset-4 hover:underline"
                          href={`/workspaces/${workspaceId}/runs/${baseline.run_id}`}
                        >
                          {baseline.run_id.slice(0, 8)}
                        </Link>
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

function MetaRow({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="mt-0.5 text-foreground">{children}</dd>
    </div>
  );
}
