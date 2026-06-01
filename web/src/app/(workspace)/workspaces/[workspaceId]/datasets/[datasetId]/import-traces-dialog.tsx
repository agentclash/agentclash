"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Loader2, Upload } from "lucide-react";

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
  const [open, setOpen] = useState(false);
  const [platform, setPlatform] =
    useState<DatasetTraceSourcePlatform>("agentclash");
  const [runId, setRunId] = useState("");
  const [runAgentId, setRunAgentId] = useState("");
  const [payloadJson, setPayloadJson] = useState("");
  const [payloadError, setPayloadError] = useState<string>();
  const [submitting, setSubmitting] = useState(false);

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
      });
      toast.success(`Imported ${result.candidates.length} trace candidates`);
      setOpen(false);
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Import failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button size="sm" variant="outline" />}>
        <Upload data-icon="inline-start" className="size-4" />
        Import traces
      </DialogTrigger>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Import traces</DialogTitle>
          <DialogDescription>
            Import production traces as reviewable candidates before promotion.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <label className="mb-1.5 block text-sm font-medium">Platform</label>
            <select
              value={platform}
              onChange={(e) =>
                setPlatform(e.target.value as DatasetTraceSourcePlatform)
              }
              className={inputClass}
            >
              {PLATFORMS.map((p) => (
                <option key={p.value} value={p.value}>
                  {p.label}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Run ID{" "}
              <span className="font-normal text-muted-foreground">
                (optional)
              </span>
            </label>
            <input
              value={runId}
              onChange={(e) => setRunId(e.target.value)}
              className={inputClass}
            />
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Run agent ID{" "}
              <span className="font-normal text-muted-foreground">
                (optional)
              </span>
            </label>
            <input
              value={runAgentId}
              onChange={(e) => setRunAgentId(e.target.value)}
              className={inputClass}
            />
          </div>
          <JsonField
            label="Payload"
            description="Optional JSON payload for external trace formats."
            value={payloadJson}
            onChange={setPayloadJson}
            error={payloadError}
            rows={6}
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
