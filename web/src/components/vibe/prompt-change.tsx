"use client";

import { useState } from "react";
import { Copy } from "lucide-react";
import { instructionDiff, diffContext } from "./instruction-diff";
import { VibeButton } from "./vibe-button";

export function PromptChange({
  before,
  after,
}: {
  before: string;
  after: string;
}) {
  const [copied, setCopied] = useState(false);
  const [issue, setIssue] = useState("");
  const changes = instructionDiff(before, after);
  return (
    <details className="vibe-panel text-sm">
      <summary>Review the instruction change</summary>
      <div className="space-y-4 border-t border-[var(--vibe-border)] p-5">
        <p className="vibe-transcript leading-7" aria-label="Instruction changes">
          {changes.map((change, i) => change.kind === "removed" ? (
            <del key={i} className="vibe-diff-removed"><span className="sr-only">Removed: </span>{change.text}</del>
          ) : change.kind === "added" ? (
            <ins key={i} className="vibe-diff-added"><span className="sr-only">Added: </span>{change.text}</ins>
          ) : <span key={i}>{diffContext(change.text, i === 0, i === changes.length - 1)}</span>)}
        </p>
        <details>
          <summary className="cursor-pointer text-xs vibe-muted">Full updated instructions</summary>
          <pre className="vibe-transcript mt-3 text-sm">{after}</pre>
        </details>
        <p className="text-xs vibe-muted">
          Your earlier version and results are kept. Running a comparison tests
          these instructions here; it does not change your live app.
        </p>
        <VibeButton
          variant="quiet"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(after);
              setCopied(true);
              setIssue("");
            } catch {
              setIssue("Select and copy the full updated instructions above.");
            }
          }}
        >
          <Copy />
          {copied ? "Copied instructions" : "Copy updated instructions"}
        </VibeButton>
        {issue && (
          <p role="status" className="text-xs vibe-muted">
            {issue}
          </p>
        )}
      </div>
    </details>
  );
}
