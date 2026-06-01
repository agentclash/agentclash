"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { FileUp, Loader2, Upload } from "lucide-react";

import { importDatasetTraces } from "@/lib/api/datasets";
import { createApiClient } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import type { DatasetTraceSourcePlatform } from "@/lib/api/types";
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
import {
  EMPTY_REDACTION_FIELDS,
  RedactionEditor,
  buildRedactionFromFields,
  type RedactionFieldState,
} from "../dataset-ui-shared";

const PLATFORMS: { value: DatasetTraceSourcePlatform; label: string }[] = [
  { value: "agentclash", label: "AgentClash" },
  { value: "otel", label: "OpenTelemetry" },
  { value: "braintrust", label: "Braintrust" },
  { value: "langsmith", label: "LangSmith" },
  { value: "phoenix", label: "Phoenix" },
];

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface ImportTracesDialogProps {
  workspaceId: string;
  datasetId: string;
}

export function ImportTracesDialog({
  workspaceId,
  datasetId,
}: ImportTracesDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [platform, setPlatform] =
    useState<DatasetTraceSourcePlatform>("agentclash");
  const [runId, setRunId] = useState("");
  const [runAgentId, setRunAgentId] = useState("");
  const [artifactId, setArtifactId] = useState("");
  const [payloadJson, setPayloadJson] = useState("");
  const [payloadError, setPayloadError] = useState<string>();
  const [redactionFields, setRedactionFields] =
    useState<RedactionFieldState>(EMPTY_REDACTION_FIELDS);
  const [submitting, setSubmitting] = useState(false);

  function reset() {
    setPlatform("agentclash");
    setRunId("");
    setRunAgentId("");
    setArtifactId("");
    setPayloadJson("");
    setPayloadError(undefined);
    setRedactionFields(EMPTY_REDACTION_FIELDS);
  }

  async function handlePayloadFile(file: File) {
    try {
      const text = await file.text();
      JSON.parse(text);
      setPayloadJson(text);
      setPayloadError(undefined);
    } catch {
      setPayloadError("Payload file must contain valid JSON");
    }
  }

  async function handleImport() {
    let payload: unknown | undefined;
    if (payloadJson.trim()) {
      try {
        payload = JSON.parse(payloadJson);
      } catch {
        setPayloadError("Invalid JSON");
        return;
      }
    }

    setSubmitting(true);
    try {
      const token = await getAccessToken();
      const api = createApiClient(token);
      const result = await importDatasetTraces(api, workspaceId, datasetId, {
        source_platform: platform,
        payload,
        run_id: runId.trim() || undefined,
        run_agent_id: runAgentId.trim() || undefined,
        artifact_id: artifactId.trim() || undefined,
        redaction: buildRedactionFromFields(redactionFields),
      });
      toast.success(`Imported ${result.candidates.length} trace candidates`);
      setOpen(false);
      reset();
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Import failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (next) reset();
      }}
    >
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <Upload data-icon="inline-start" className="size-4" />
        Import traces
      </DialogTrigger>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>Import traces</DialogTitle>
          <DialogDescription>
            Import production traces as reviewable candidates before promotion.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 max-h-[70vh] overflow-y-auto">
          <div>
            <label className="mb-1.5 block text-sm font-medium">Platform</label>
            <select
              value={platform}
              onChange={(e) =>
                setPlatform(e.target.value as DatasetTraceSourcePlatform)
              }
              className={inputClass}
            >
              {PLATFORMS.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <TextField label="Run ID" value={runId} onChange={setRunId} />
            <TextField
              label="Run agent ID"
              value={runAgentId}
              onChange={setRunAgentId}
            />
            <TextField
              label="Artifact ID"
              value={artifactId}
              onChange={setArtifactId}
              className="sm:col-span-2"
            />
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Payload file{" "}
              <span className="font-normal text-muted-foreground">
                (optional JSON)
              </span>
            </label>
            <input
              ref={fileInputRef}
              type="file"
              accept="application/json,.json"
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) void handlePayloadFile(file);
              }}
              className="hidden"
            />
            <button
              type="button"
              onClick={() => fileInputRef.current?.click()}
              className="mb-2 flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-border px-4 py-4 text-sm text-muted-foreground hover:border-foreground/30 hover:text-foreground"
            >
              <FileUp className="size-4" />
              Upload JSON payload
            </button>
          </div>
          <JsonField
            label="Payload"
            description="Optional JSON payload for external trace formats."
            value={payloadJson}
            onChange={setPayloadJson}
            error={payloadError}
            rows={6}
          />
          <RedactionEditor
            fields={redactionFields}
            onFieldsChange={setRedactionFields}
          />
        </div>
        <DialogFooter>
          <Button onClick={handleImport} disabled={submitting}>
            {submitting && (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            )}
            Import
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function TextField({
  label,
  value,
  onChange,
  className,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  className?: string;
}) {
  return (
    <div className={className}>
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
