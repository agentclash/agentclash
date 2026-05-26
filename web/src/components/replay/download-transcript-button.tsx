"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Download, Loader2 } from "lucide-react";
import { toast } from "sonner";
import type { TranscriptTurn } from "@/lib/api/types";
import type { TranscriptPdfMeta } from "./transcript-pdf-document";

interface DownloadTranscriptButtonProps {
  turns: TranscriptTurn[];
  meta: Omit<TranscriptPdfMeta, "generatedAt">;
}

function slugify(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}

export function DownloadTranscriptButton({
  turns,
  meta,
}: DownloadTranscriptButtonProps) {
  const [generating, setGenerating] = useState(false);

  const handleDownload = async () => {
    if (generating || turns.length === 0) return;
    setGenerating(true);
    try {
      // Dynamic import keeps @react-pdf/renderer (large) out of the main
      // bundle — it's only loaded when a user actually exports.
      const [{ pdf }, { TranscriptPdfDocument }] = await Promise.all([
        import("@react-pdf/renderer"),
        import("./transcript-pdf-document"),
      ]);

      const generatedAt = new Date().toISOString().slice(0, 16).replace("T", " ");
      const blob = await pdf(
        <TranscriptPdfDocument turns={turns} meta={{ ...meta, generatedAt }} />,
      ).toBlob();

      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `transcript-${slugify(meta.agentLabel) || "conversation"}-${meta.runAgentId.slice(0, 8)}.pdf`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      // Delay revocation so a busy download queue has fetched the blob first.
      setTimeout(() => URL.revokeObjectURL(url), 100);
    } catch (err) {
      console.error("transcript PDF generation failed", err);
      toast.error("Failed to generate transcript PDF");
    } finally {
      setGenerating(false);
    }
  };

  return (
    <Button
      variant="outline"
      size="xs"
      onClick={handleDownload}
      disabled={generating || turns.length === 0}
    >
      {generating ? (
        <Loader2 className="size-3.5 animate-spin" />
      ) : (
        <Download className="size-3.5" />
      )}
      {generating ? "Generating…" : "Export PDF"}
    </Button>
  );
}
