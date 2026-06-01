"use client";

import { useEffect, useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Eye, Loader2 } from "lucide-react";

import { getDatasetVersion } from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { DatasetVersion } from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import {
  ExamplePayloadPreview,
  TagBadges,
} from "../dataset-ui-shared";

interface VersionDetailSheetProps {
  workspaceId: string;
  datasetId: string;
  version: DatasetVersion;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function VersionDetailSheet({
  workspaceId,
  datasetId,
  version,
  open,
  onOpenChange,
}: VersionDetailSheetProps) {
  const { getAccessToken } = useAccessToken();
  const [loading, setLoading] = useState(false);
  const [exampleCount, setExampleCount] = useState(version.example_count);

  useEffect(() => {
    if (!open) return;
    void (async () => {
      setLoading(true);
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const detail = await getDatasetVersion(
          api,
          workspaceId,
          datasetId,
          version.id,
        );
        setExampleCount(detail.examples.length);
      } catch (err) {
        toast.error(
          err instanceof ApiError ? err.message : "Failed to load version detail",
        );
      } finally {
        setLoading(false);
      }
    })();
  }, [datasetId, getAccessToken, open, version.id, workspaceId]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>
            Version v{version.version_number}
            {version.label ? ` · ${version.label}` : ""}
          </SheetTitle>
          <SheetDescription className="flex items-center gap-2">
            {loading ? (
              <Loader2 className="size-3.5 animate-spin" aria-hidden />
            ) : null}
            Immutable snapshot with {exampleCount} example
            {exampleCount === 1 ? "" : "s"}.
          </SheetDescription>
        </SheetHeader>
        <VersionDetailBody
          workspaceId={workspaceId}
          datasetId={datasetId}
          versionId={version.id}
          open={open}
        />
      </SheetContent>
    </Sheet>
  );
}

export function VersionDetailTrigger({
  workspaceId,
  datasetId,
  version,
}: {
  workspaceId: string;
  datasetId: string;
  version: DatasetVersion;
}) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        aria-label={`View version v${version.version_number}`}
        onClick={() => setOpen(true)}
      >
        <Eye className="size-3.5" />
      </Button>
      <VersionDetailSheet
        workspaceId={workspaceId}
        datasetId={datasetId}
        version={version}
        open={open}
        onOpenChange={setOpen}
      />
    </>
  );
}

function VersionDetailBody({
  workspaceId,
  datasetId,
  versionId,
  open,
}: {
  workspaceId: string;
  datasetId: string;
  versionId: string;
  open: boolean;
}) {
  const { getAccessToken } = useAccessToken();
  const [loading, setLoading] = useState(true);
  const [checksum, setChecksum] = useState("");
  const [createdAt, setCreatedAt] = useState("");
  const [examples, setExamples] = useState<
    Awaited<ReturnType<typeof getDatasetVersion>>["examples"]
  >([]);

  useEffect(() => {
    if (!open) return;
    void (async () => {
      setLoading(true);
      try {
        const token = await getAccessToken();
        const api = createApiClient(token);
        const detail = await getDatasetVersion(
          api,
          workspaceId,
          datasetId,
          versionId,
        );
        setChecksum(detail.version.manifest_checksum);
        setCreatedAt(detail.version.created_at);
        setExamples(detail.examples);
      } catch (err) {
        toast.error(
          err instanceof ApiError ? err.message : "Failed to load version examples",
        );
      } finally {
        setLoading(false);
      }
    })();
  }, [datasetId, getAccessToken, open, versionId, workspaceId]);

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="mt-6 space-y-4">
      <dl className="grid gap-3 text-sm sm:grid-cols-2">
        <div>
          <dt className="text-muted-foreground">Checksum</dt>
          <dd className="mt-1 break-all font-[family-name:var(--font-mono)] text-xs">
            {checksum}
          </dd>
        </div>
        <div>
          <dt className="text-muted-foreground">Created</dt>
          <dd className="mt-1">{new Date(createdAt).toLocaleString()}</dd>
        </div>
      </dl>

      <div className="space-y-3">
        {examples.map((example) => (
          <div
            key={example.id}
            className="rounded-lg border border-border bg-card/20 p-3"
          >
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">
                {example.external_id ?? example.id.slice(0, 8)}
              </span>
              <TagBadges tags={example.tags} />
            </div>
            <ExamplePayloadPreview
              input={example.input}
              expected={example.expected}
              metadata={example.metadata}
            />
          </div>
        ))}
      </div>
    </div>
  );
}
