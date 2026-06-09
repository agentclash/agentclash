"use client";

import { useEffect, useState } from "react";
import { Download, FileText, Loader2 } from "lucide-react";

import type {
  AgentTryout,
  AgentTryoutArtifact,
  ListAgentTryoutArtifactsResponse,
} from "@/lib/api/types";
import { useApiQuery } from "@/lib/api/swr";
import { Button } from "@/components/ui/button";
import { tryoutIsActive } from "../status";

export function ArtifactsSection({
  workspaceId,
  tryout,
}: {
  workspaceId: string;
  tryout: AgentTryout;
}) {
  const { data } = useApiQuery<ListAgentTryoutArtifactsResponse>(
    `/v1/workspaces/${workspaceId}/agent-tryouts/${tryout.id}/artifacts`,
    undefined,
    {
      // Outputs land when the run finishes — keep polling while it's active.
      refreshInterval: tryoutIsActive(tryout.status) ? 3000 : 0,
    },
  );
  const artifacts = data?.items ?? [];

  return (
    <section>
      <h2 className="mb-3 text-sm font-semibold tracking-tight">Output</h2>
      {artifacts.length === 0 ? (
        <div className="rounded-lg border border-border p-4 text-sm text-muted-foreground">
          {tryoutIsActive(tryout.status)
            ? "The agent is still working — output files will appear here when it finishes."
            : tryout.status === "completed"
              ? "This tryout completed without producing any downloadable output files."
              : "No output was captured for this tryout."}
        </div>
      ) : (
        <ul className="space-y-4">
          {artifacts.map((artifact) => (
            <ArtifactCard key={artifact.id} artifact={artifact} />
          ))}
        </ul>
      )}
    </section>
  );
}

const PREVIEWABLE = ["text/", "application/json"];
const MAX_PREVIEW_BYTES = 512 * 1024;

function isPreviewable(artifact: AgentTryoutArtifact): boolean {
  const ct = artifact.content_type ?? "";
  if (!PREVIEWABLE.some((prefix) => ct.startsWith(prefix))) return false;
  return (artifact.size_bytes ?? 0) <= MAX_PREVIEW_BYTES;
}

function ArtifactCard({ artifact }: { artifact: AgentTryoutArtifact }) {
  const label = artifact.path || artifact.key || artifact.artifact_type;
  const previewable = isPreviewable(artifact) && Boolean(artifact.download_url);
  const isJson = (artifact.content_type ?? "").startsWith("application/json");

  const [preview, setPreview] = useState<string>();
  const [previewError, setPreviewError] = useState(false);
  const [loading, setLoading] = useState(previewable);

  useEffect(() => {
    if (!previewable || !artifact.download_url) return;
    let cancelled = false;
    fetch(artifact.download_url)
      .then((res) => (res.ok ? res.text() : Promise.reject(new Error())))
      .then((text) => {
        if (cancelled) return;
        setPreview(isJson ? prettyJson(text) : text);
        setLoading(false);
      })
      .catch(() => {
        if (cancelled) return;
        setPreviewError(true);
        setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [previewable, artifact.download_url, isJson]);

  return (
    <li className="overflow-hidden rounded-lg border border-border">
      <div className="flex items-center justify-between gap-3 border-b border-border bg-muted/20 px-4 py-2.5">
        <div className="flex min-w-0 items-center gap-2">
          <FileText className="size-4 shrink-0 text-muted-foreground" />
          <span className="truncate text-sm font-medium">{label}</span>
          {typeof artifact.size_bytes === "number" ? (
            <span className="shrink-0 text-xs text-muted-foreground">
              {formatBytes(artifact.size_bytes)}
            </span>
          ) : null}
        </div>
        {artifact.download_url ? (
          <Button
            size="xs"
            variant="outline"
            render={
              <a href={artifact.download_url} download={label} target="_blank" rel="noreferrer" />
            }
          >
            <Download data-icon="inline-start" className="size-3.5" />
            Download
          </Button>
        ) : null}
      </div>
      {previewable ? (
        loading ? (
          <div className="flex justify-center py-6">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          </div>
        ) : previewError ? (
          <p className="px-4 py-3 text-sm text-muted-foreground">
            Preview unavailable — use Download to view this file.
          </p>
        ) : (
          <pre className="max-h-96 overflow-auto bg-muted/10 p-4 text-xs leading-relaxed whitespace-pre-wrap">
            {preview}
          </pre>
        )
      ) : (
        <p className="px-4 py-3 text-sm text-muted-foreground">
          {artifact.content_type ?? "binary"} — download to view.
        </p>
      )}
    </li>
  );
}

function prettyJson(text: string): string {
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
