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
import {
  EMPTY_MAPPING_FIELDS,
  ImportPreviewPanel,
  MappingEditor,
  parseMappingInput,
  type MappingFieldState,
} from "../dataset-ui-shared";

const FORMATS: { value: DatasetInteropFormat; label: string }[] = [
  { value: "jsonl", label: "JSONL" },
  { value: "openai", label: "OpenAI JSONL" },
  { value: "csv", label: "CSV" },
  { value: "braintrust", label: "Braintrust" },
  { value: "langsmith", label: "LangSmith" },
  { value: "phoenix", label: "Phoenix" },
];

const inputClass =
  "block w-full rounded-lg border border-input bg-transparent [&>option]:bg-popover [&>option]:text-popover-foreground px-3 py-2 text-sm focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring/50";

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
  const [mappingMode, setMappingMode] = useState<"simple" | "advanced">(
    "simple",
  );
  const [mappingFields, setMappingFields] =
    useState<MappingFieldState>(EMPTY_MAPPING_FIELDS);
  const [mappingJson, setMappingJson] = useState("");
  const [mappingError, setMappingError] = useState<string>();

  function reset() {
    setFile(null);
    setFormat("jsonl");
    setMode("add");
    setDryRun(false);
    setPreview(null);
    setMappingMode("simple");
    setMappingFields(EMPTY_MAPPING_FIELDS);
    setMappingJson("");
    setMappingError(undefined);
  }

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) reset();
  }

  async function handleImport() {
    if (!file) return;

    const { mapping, error } = parseMappingInput(
      mappingMode,
      mappingFields,
      mappingJson,
    );
    if (error) {
      setMappingError(error);
      return;
    }
    setMappingError(undefined);

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
        mapping,
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
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Import examples</DialogTitle>
          <DialogDescription>
            Upload a file in a supported eval dataset format.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 max-h-[70vh] overflow-y-auto">
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
              {FORMATS.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
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
          <MappingEditor
            mode={mappingMode}
            onModeChange={setMappingMode}
            fields={mappingFields}
            onFieldsChange={setMappingFields}
            json={mappingJson}
            onJsonChange={setMappingJson}
            jsonError={mappingError}
          />
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={dryRun}
              onChange={(e) => setDryRun(e.target.checked)}
            />
            Dry run (preview only)
          </label>
          {preview ? (
            <ImportPreviewPanel
              importedCount={preview.imported_count}
              preview={preview.preview}
              errors={preview.errors}
            />
          ) : null}
        </div>
        <DialogFooter>
          <Button onClick={handleImport} disabled={!file || submitting}>
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
