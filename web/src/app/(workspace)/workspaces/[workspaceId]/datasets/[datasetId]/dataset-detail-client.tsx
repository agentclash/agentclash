"use client";

import type { Dataset, DatasetExample, DatasetVersion } from "@/lib/api/types";
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
}: {
  dataset: Dataset;
  examples: DatasetExample[];
  versions: DatasetVersion[];
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
      </Tabs>
    </div>
  );
}
