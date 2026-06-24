"use client";

import { useState, useRef } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { uploadArtifact } from "@/lib/api/artifacts";
import { useApiMutator } from "@/lib/api/swr";
import { ApiError } from "@/lib/api/errors";
import { workspaceResourceKeys } from "@/lib/workspace-resource";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogTrigger,
} from "@/components/ui/dialog";
import { JsonField } from "@/components/ui/json-field";
import { Upload, Loader2, CheckCircle2, FileUp } from "lucide-react";

const ARTIFACT_TYPE_PATTERN = /^[a-z0-9][a-z0-9._-]{0,63}$/;

interface UploadArtifactDialogProps {
  workspaceId: string;
  runId?: string;
  runAgentId?: string;
  onUploaded?: () => void;
}

export function UploadArtifactDialog({
  workspaceId,
  runId,
  runAgentId,
  onUploaded,
}: UploadArtifactDialogProps) {
  const { getAccessToken } = useAccessToken();
  const { mutate } = useApiMutator();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [artifactType, setArtifactType] = useState("log");
  const [metadataJson, setMetadataJson] = useState("{}");
  const [typeError, setTypeError] = useState<string>();
  const [metadataError, setMetadataError] = useState<string>();
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [apiError, setApiError] = useState<string>();
  const [success, setSuccess] = useState(false);

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) {
      setFile(null);
      setArtifactType("log");
      setMetadataJson("{}");
      setTypeError(undefined);
      setMetadataError(undefined);
      setApiError(undefined);
      setSuccess(false);
      setProgress(0);
    }
  }

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = e.target.files?.[0] ?? null;
    setFile(selected);
  }

  async function handleUpload() {
    setTypeError(undefined);
    setMetadataError(undefined);
    setApiError(undefined);

    if (!file) return;

    if (!ARTIFACT_TYPE_PATTERN.test(artifactType)) {
      setTypeError(
        "Must be lowercase alphanumeric with dots, underscores, or hyphens (1-64 chars)",
      );
      return;
    }

    let metadata: Record<string, unknown> | undefined;
    if (metadataJson.trim() && metadataJson.trim() !== "{}") {
      try {
        metadata = JSON.parse(metadataJson);
      } catch {
        setMetadataError("Invalid JSON");
        return;
      }
    }

    setUploading(true);
    setProgress(0);
    try {
      const token = await getAccessToken();
      if (!token) return;
      await uploadArtifact({
        token,
        workspaceId,
        file,
        artifactType,
        runId,
        runAgentId,
        metadata,
        onProgress: setProgress,
      });
      setSuccess(true);
      await mutate(workspaceResourceKeys.artifacts(workspaceId));
      onUploaded?.();
    } catch (err) {
      setApiError(
        err instanceof ApiError ? err.message : "Upload failed",
      );
    } finally {
      setUploading(false);
    }
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger render={<Button variant="outline" size="sm" />}>
        <Upload className="size-4 mr-1.5" />
        Upload Artifact
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Upload Artifact</DialogTitle>
          <DialogDescription>
            Attach a file to{" "}
            {runAgentId
              ? "this agent"
              : runId
                ? "this run"
                : "this workspace"}
            . Max 100 MB.
          </DialogDescription>
        </DialogHeader>

        {success ? (
          <div className="flex flex-col items-center py-6 text-center">
            <CheckCircle2 className="size-8 text-emerald-400 mb-2" />
            <p className="text-sm font-medium">Upload complete</p>
            <p className="text-xs text-muted-foreground mt-1">
              {file?.name} uploaded successfully.
            </p>
          </div>
        ) : (
          <div className="space-y-4">
            {/* File picker */}
            <div>
              <label className="mb-1.5 block text-sm font-medium">File</label>
              <input
                ref={fileInputRef}
                type="file"
                onChange={handleFileChange}
                disabled={uploading}
                className="hidden"
              />
              <button
                onClick={() => fileInputRef.current?.click()}
                disabled={uploading}
                className="w-full flex items-center justify-center gap-2 rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground hover:border-foreground/30 hover:text-foreground transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <FileUp className="size-4" />
                {file ? (
                  <span>
                    {file.name}{" "}
                    <span className="text-muted-foreground/60">
                      ({formatSize(file.size)})
                    </span>
                  </span>
                ) : (
                  "Click to select a file"
                )}
              </button>
            </div>

            {/* Artifact type */}
            <div>
              <label className="mb-1.5 block text-sm font-medium">
                Artifact Type
              </label>
              <input
                type="text"
                value={artifactType}
                onChange={(e) => setArtifactType(e.target.value)}
                disabled={uploading}
                placeholder="log"
                className="block w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring/50 disabled:opacity-50 disabled:cursor-not-allowed"
              />
              {typeError && (
                <p className="mt-1 text-xs text-destructive">{typeError}</p>
              )}
              <p className="mt-1 text-xs text-muted-foreground">
                e.g. log, output, challenge_pack_bundle
              </p>
            </div>

            {/* Metadata */}
            <JsonField
              label="Metadata (optional)"
              description="Optional key-value JSON attached to the artifact."
              value={metadataJson}
              onChange={setMetadataJson}
              error={metadataError}
              disabled={uploading}
              rows={3}
            />

            {/* Progress bar */}
            {uploading && (
              <div>
                <div className="flex items-center justify-between text-xs text-muted-foreground mb-1">
                  <span>Uploading...</span>
                  <span>{progress}%</span>
                </div>
                <div className="h-1.5 rounded-full bg-muted overflow-hidden">
                  <div
                    className="h-full bg-primary rounded-full transition-all duration-300"
                    style={{ width: `${progress}%` }}
                  />
                </div>
              </div>
            )}

            {apiError && (
              <div className="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-xs text-destructive">
                {apiError}
              </div>
            )}
          </div>
        )}

        {!success && (
          <DialogFooter>
            <Button
              onClick={handleUpload}
              disabled={!file || uploading}
            >
              {uploading && (
                <Loader2
                  data-icon="inline-start"
                  className="size-4 animate-spin"
                />
              )}
              {uploading ? "Uploading..." : "Upload"}
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  );
}
