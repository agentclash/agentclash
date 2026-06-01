"use client";

import { useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { Download, Loader2 } from "lucide-react";

import { exportDatasetBlob } from "@/lib/api/datasets";
import { ApiError } from "@/lib/api/errors";
import type { DatasetInteropFormat, DatasetVersion } from "@/lib/api/types";
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

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface ExportDatasetButtonProps {
  workspaceId: string;
  datasetId: string;
  versions: DatasetVersion[];
}

export function ExportDatasetButton({
  workspaceId,
  datasetId,
  versions,
}: ExportDatasetButtonProps) {
  const { getAccessToken } = useAccessToken();
  const [open, setOpen] = useState(false);
  const [format, setFormat] = useState<DatasetInteropFormat>("jsonl");
  const [versionId, setVersionId] = useState("");
  const [exporting, setExporting] = useState(false);

  async function handleExport() {
    setExporting(true);
    try {
      const token = await getAccessToken();
      if (!token) return;
      const { blob, filename } = await exportDatasetBlob({
        token,
        workspaceId,
        datasetId,
        format,
        versionId: versionId || undefined,
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      a.click();
      URL.revokeObjectURL(url);
      toast.success("Export downloaded");
      setOpen(false);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Export failed");
    } finally {
      setExporting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" size="sm" />}>
        <Download data-icon="inline-start" className="size-4" />
        Export
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Export examples</DialogTitle>
          <DialogDescription>
            Download active examples or a pinned version.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <label className="mb-1.5 block text-sm font-medium">Format</label>
            <select
              value={format}
              onChange={(e) =>
                setFormat(e.target.value as DatasetInteropFormat)
              }
              className={inputClass}
            >
              <option value="jsonl">JSONL</option>
              <option value="openai">OpenAI JSONL</option>
              <option value="csv">CSV</option>
              <option value="braintrust">Braintrust</option>
              <option value="langsmith">LangSmith</option>
              <option value="phoenix">Phoenix</option>
            </select>
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium">
              Version <span className="font-normal text-muted-foreground">(optional)</span>
            </label>
            <select
              value={versionId}
              onChange={(e) => setVersionId(e.target.value)}
              className={inputClass}
            >
              <option value="">Active examples</option>
              {versions.map((v) => (
                <option key={v.id} value={v.id}>
                  v{v.version_number}
                  {v.label ? ` — ${v.label}` : ""}
                </option>
              ))}
            </select>
          </div>
        </div>
        <DialogFooter>
          <Button onClick={handleExport} disabled={exporting}>
            {exporting && (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            )}
            Download
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
