"use client";

import { useState } from "react";
import { useAccessToken } from "@workos-inc/authkit-nextjs/components";
import { downloadArtifact } from "@/lib/api/artifacts";
import { Download, Loader2 } from "lucide-react";

interface DownloadArtifactButtonProps {
  artifactId: string;
  label?: string;
}

export function DownloadArtifactButton({
  artifactId,
  label,
}: DownloadArtifactButtonProps) {
  const { getAccessToken } = useAccessToken();
  const [loading, setLoading] = useState(false);

  async function handleDownload() {
    setLoading(true);
    try {
      const token = await getAccessToken();
      if (!token) return;
      await downloadArtifact(token, artifactId);
    } catch {
      // Silently fail — signed URL open is best-effort
    } finally {
      setLoading(false);
    }
  }

  return (
    <button
      onClick={handleDownload}
      disabled={loading}
      className="inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors disabled:opacity-50"
    >
      {loading ? (
        <Loader2 className="size-3 animate-spin" />
      ) : (
        <Download className="size-3" />
      )}
      {label ?? "Download"}
    </button>
  );
}
