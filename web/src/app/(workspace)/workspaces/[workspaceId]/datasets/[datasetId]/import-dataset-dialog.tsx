"use client";

import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { toast } from "sonner";
import { FileUp, Loader2, Upload } from "lucide-react";

import { importDataset } from "@/lib/api/datasets";
import { ApiError } from "@/lib/api/errors";
import type {
  DatasetImportMode,
  DatasetImportResponse,
  DatasetInteropFormat,
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

const FORMATS: { value: DatasetInteropFormat; label: string }[] = [
  { value: "jsonl", label: "JSONL" },
  { value: "openai", label: "OpenAI JSONL" },
  { value: "csv", label: "CSV" },
  { value: "braintrust", label: "Braintrust" },
  { value: "langsmith", label: "LangSmith" },
  { value: "phoenix", label: "Phoenix" },
];

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

interface ImportDatasetDialogProps {
  workspaceId: string;
  datasetId: string;
}

export function ImportDatasetDialog({
  workspaceId,
  datasetId,
}: ImportDatasetDialogProps) {
  const router = useRouter();
  const { getAccessToken } = useAccessToken();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [format, setFormat] = useState<DatasetInteropFormat>("jsonl");
  const [mode, setMode] = useState<DatasetImportMode>("add");
  const [dryRun, setDryRun] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [preview, setPreview] = useState<DatasetImportResponse | null>(null);

  function reset() {
    setFile(null);
    setFormat("jsonl");
    setMode("add");
    setDryRun(false);
    setPreview(null);
  }

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) reset();
  }

  async function handleImport() {
    if (!file) return;
    setSubmitting(true);
    try {
      const token = await getAccessToken();
      if (!token) return;
      const result = await importDataset({
        token,
        workspaceId,
        datasetId,
        file,
        format,
        mode,
        dryRun,
      });
      if (dryRun) {
        setPreview(result);
        toast.success(`Preview: ${result.imported_count} rows`);
        return;
      }
      toast.success(`Imported ${result.imported_count} examples`);
      setOpen(false);
      router.refresh();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Import failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger render={<Button variant="outline" size="sm" />}>
        <Upload data-icon="inline-start" className="size-4" />
        Import
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Import examples</DialogTitle>
          <DialogDescription>
            Upload a file in a supported eval dataset format.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <label className="mb-1.5 block text-sm font-medium">File</label>
            <input
              ref={fileInputRef}
              type="file"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              className="hidden"
            />
            <button
              type="button"
              onClick={() => fileInputRef.current?.click()}
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground hover:border-foreground/30 hover:text-foreground"
            >
              <FileUp className="size-4" />
              {file ? file.name : "Click to select a file"}
            </button>
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium">Format</label>
            <select
              value={format}
              onChange={(e) =>
                setFormat(e.target.value as DatasetInteropFormat)
              }
              className={inputClass}
            >
              {FORMATS.map((f) => (
                <option key={f.value} value={f.value}>
                  {f.label}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium">Mode</label>
            <select
              value={mode}
              onChange={(e) => setMode(e.target.value as DatasetImportMode)}
              className={inputClass}
            >
              <option value="add">Add to existing examples</option>
              <option value="replace">Replace active examples</option>
            </select>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={dryRun}
              onChange={(e) => setDryRun(e.target.checked)}
            />
            Dry run (preview only)
          </label>
          {preview && (
            <div className="rounded-md border border-border bg-muted/30 p-3 text-xs">
              <p className="font-medium">
                Preview: {preview.imported_count} rows
              </p>
              {preview.errors && preview.errors.length > 0 && (
                <p className="mt-1 text-destructive">
                  {preview.errors.length} row errors
                </p>
              )}
            </div>
          )}
        </div>
        <DialogFooter>
          <Button
            onClick={handleImport}
            disabled={!file || submitting}
          >
            {submitting && (
              <Loader2 data-icon="inline-start" className="size-4 animate-spin" />
            )}
            {dryRun ? "Preview import" : "Import"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
