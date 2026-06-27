"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Pencil, Plus } from "lucide-react";

import {
  addDatasetExample,
  patchDatasetExample,
} from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type {
  DatasetExample,
  DatasetExampleSource,
  DatasetExampleStatus,
  PatchDatasetExampleInput,
  UpsertDatasetExampleInput,
} from "@/lib/api/types";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { JsonField } from "@/components/ui/json-field";
import { ExamplePayloadPreview } from "../dataset-ui-shared";

const SOURCE_OPTIONS: DatasetExampleSource[] = [
  "manual",
  "import",
  "trace",
  "synthetic",
  "promotion",
];

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent [&>option]:bg-popover [&>option]:text-popover-foreground px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface ExampleFormDialogProps {
  workspaceId: string;
  datasetId: string;
  example?: DatasetExample;
  trigger?: "add" | "edit" | "view";
}

export function ExampleFormDialog({
  workspaceId,
  datasetId,
  example,
  trigger = example ? "edit" : "add",
}: ExampleFormDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const isView = trigger === "view" && example != null;
  const isEdit = trigger === "edit" && example != null;
  const [open, setOpen] = useState(false);
  const [externalId, setExternalId] = useState("");
  const [inputJson, setInputJson] = useState("{}");
  const [expectedJson, setExpectedJson] = useState("");
  const [metadataJson, setMetadataJson] = useState("");
  const [tags, setTags] = useState("");
  const [status, setStatus] = useState<DatasetExampleStatus>("active");
  const [source, setSource] = useState<DatasetExampleSource>("manual");
  const [sourceRunId, setSourceRunId] = useState("");
  const [sourceTraceId, setSourceTraceId] = useState("");
  const [sourcePlatform, setSourcePlatform] = useState("");
  const [artifactId, setArtifactId] = useState("");
  const [inputError, setInputError] = useState<string>();
  const [expectedError, setExpectedError] = useState<string>();
  const [metadataError, setMetadataError] = useState<string>();
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      if (example) {
        setExternalId(example.external_id ?? "");
        setInputJson(JSON.stringify(example.input, null, 2));
        setExpectedJson(
          example.expected != null
            ? JSON.stringify(example.expected, null, 2)
            : "",
        );
        setMetadataJson(
          Object.keys(example.metadata ?? {}).length > 0
            ? JSON.stringify(example.metadata, null, 2)
            : "",
        );
        setTags(example.tags.join(", "));
        setStatus(example.status);
        setSource(example.source);
        setSourceRunId(example.source_run_id ?? "");
        setSourceTraceId(example.source_trace_id ?? "");
        setSourcePlatform(example.source_platform ?? "");
        setArtifactId(example.artifact_id ?? "");
      } else {
        setExternalId("");
        setInputJson("{}");
        setExpectedJson("");
        setMetadataJson("");
        setTags("");
        setStatus("active");
        setSource("manual");
        setSourceRunId("");
        setSourceTraceId("");
        setSourcePlatform("");
        setArtifactId("");
      }
      setInputError(undefined);
      setExpectedError(undefined);
      setMetadataError(undefined);
    }
  }, [open, example]);

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (submitting || isView) return;

    let input: unknown;
    try {
      input = JSON.parse(inputJson);
    } catch {
      setInputError("Invalid JSON");
      return;
    }

    let expected: unknown | undefined;
    if (expectedJson.trim()) {
      try {
        expected = JSON.parse(expectedJson);
      } catch {
        setExpectedError("Invalid JSON");
        return;
      }
    }

    let metadata: Record<string, unknown> | undefined;
    if (metadataJson.trim()) {
      try {
        const parsed = JSON.parse(metadataJson) as unknown;
        if (
          parsed == null ||
          typeof parsed !== "object" ||
          Array.isArray(parsed)
        ) {
          setMetadataError("Metadata must be a JSON object");
          return;
        }
        metadata = parsed as Record<string, unknown>;
      } catch {
        setMetadataError("Invalid JSON");
        return;
      }
    }

    const tagList = tags
      .split(",")
      .map((tag) => tag.trim())
      .filter(Boolean);

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);

      if (isEdit && example) {
        const patch: PatchDatasetExampleInput = {
          input,
          tags: tagList,
          status,
          source,
        };
        if (expected !== undefined) patch.expected = expected;
        if (metadata !== undefined) patch.metadata = metadata;
        if (sourceRunId.trim()) patch.source_run_id = sourceRunId.trim();
        if (sourceTraceId.trim()) patch.source_trace_id = sourceTraceId.trim();
        if (sourcePlatform.trim()) patch.source_platform = sourcePlatform.trim();
        if (artifactId.trim()) patch.artifact_id = artifactId.trim();
        await patchDatasetExample(
          api,
          workspaceId,
          datasetId,
          example.id,
          patch,
        );
        toast.success("Example updated");
      } else {
        const body: UpsertDatasetExampleInput = {
          input,
          tags: tagList.length > 0 ? tagList : undefined,
          status,
          source,
        };
        if (externalId.trim()) body.external_id = externalId.trim();
        if (expected !== undefined) body.expected = expected;
        if (metadata !== undefined) body.metadata = metadata;
        if (sourceRunId.trim()) body.source_run_id = sourceRunId.trim();
        if (sourceTraceId.trim()) body.source_trace_id = sourceTraceId.trim();
        if (sourcePlatform.trim()) body.source_platform = sourcePlatform.trim();
        if (artifactId.trim()) body.artifact_id = artifactId.trim();
        await addDatasetExample(api, workspaceId, datasetId, body);
        toast.success("Example added");
      }

      setOpen(false);
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save example");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          isView || isEdit ? (
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label={
                isView ? "View example" : "Edit example"
              }
            />
          ) : (
            <Button size="sm" />
          )
        }
      >
        {isView ? (
          <span className="text-xs font-medium">View</span>
        ) : isEdit ? (
          <Pencil className="size-3.5" />
        ) : (
          <>
            <Plus data-icon="inline-start" className="size-4" />
            Add example
          </>
        )}
      </DialogTrigger>
      <DialogContent className="sm:max-w-xl">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>
              {isView ? "Example detail" : isEdit ? "Edit example" : "Add example"}
            </DialogTitle>
            <DialogDescription>
              {isView
                ? "Review structured input, expected output, and metadata."
                : isEdit
                  ? "Update input, expected output, metadata, tags, or provenance."
                  : "Create a manual dataset example."}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2 max-h-[60vh] overflow-y-auto">
            {isView && example ? (
              <ExamplePayloadPreview
                input={example.input}
                expected={example.expected}
                metadata={example.metadata}
              />
            ) : (
              <>
                <div>
                  <label className="mb-1.5 block text-sm font-medium">
                    External ID{" "}
                    <span className="font-normal text-muted-foreground">
                      (optional)
                    </span>
                  </label>
                  <input
                    value={externalId}
                    onChange={(e) => setExternalId(e.target.value)}
                    className={inputClass}
                  />
                </div>
                <JsonField
                  label="Input"
                  value={inputJson}
                  onChange={setInputJson}
                  error={inputError}
                  rows={5}
                />
                <JsonField
                  label="Expected"
                  description="Optional expected output for scoring."
                  value={expectedJson}
                  onChange={setExpectedJson}
                  error={expectedError}
                  rows={4}
                />
                <JsonField
                  label="Metadata"
                  description="Optional JSON object with example metadata."
                  value={metadataJson}
                  onChange={setMetadataJson}
                  error={metadataError}
                  rows={4}
                />
                <div>
                  <label className="mb-1.5 block text-sm font-medium">Tags</label>
                  <input
                    value={tags}
                    onChange={(e) => setTags(e.target.value)}
                    placeholder="comma-separated"
                    className={inputClass}
                  />
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <div>
                    <label className="mb-1.5 block text-sm font-medium">
                      Source
                    </label>
                    <select
                      value={source}
                      onChange={(e) =>
                        setSource(e.target.value as DatasetExampleSource)
                      }
                      className={inputClass}
                    >
                      {SOURCE_OPTIONS.map((option) => (
                        <option key={option} value={option}>
                          {option}
                        </option>
                      ))}
                    </select>
                  </div>
                  {isEdit ? (
                    <div>
                      <label className="mb-1.5 block text-sm font-medium">
                        Status
                      </label>
                      <select
                        value={status}
                        onChange={(e) =>
                          setStatus(e.target.value as DatasetExampleStatus)
                        }
                        className={inputClass}
                      >
                        <option value="active">active</option>
                        <option value="archived">archived</option>
                        <option value="muted">muted</option>
                      </select>
                    </div>
                  ) : null}
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <TextField
                    label="Source run ID"
                    value={sourceRunId}
                    onChange={setSourceRunId}
                  />
                  <TextField
                    label="Source trace ID"
                    value={sourceTraceId}
                    onChange={setSourceTraceId}
                  />
                  <TextField
                    label="Source platform"
                    value={sourcePlatform}
                    onChange={setSourcePlatform}
                  />
                  <TextField
                    label="Artifact ID"
                    value={artifactId}
                    onChange={setArtifactId}
                  />
                </div>
              </>
            )}
          </div>
          {!isView ? (
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setOpen(false)}
                disabled={submitting}
              >
                Cancel
              </Button>
              <Button type="submit" disabled={submitting}>
                {submitting ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : isEdit ? (
                  "Save"
                ) : (
                  "Add"
                )}
              </Button>
            </DialogFooter>
          ) : null}
        </form>
      </DialogContent>
    </Dialog>
  );
}

function TextField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div>
      <label className="mb-1.5 block text-sm font-medium">
        {label}{" "}
        <span className="font-normal text-muted-foreground">(optional)</span>
      </label>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={inputClass}
      />
    </div>
  );
}
