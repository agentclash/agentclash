"use client";

import { useState } from "react";
import { Copy } from "lucide-react";
import { instructionDiff, diffContext } from "./instruction-diff";
import { VibeButton } from "./vibe-button";

export function PromptChange({
  before,
  after,
  defaultOpen = false,
}: {
  before: string;
  after: string;
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const [copied, setCopied] = useState(false);
  const [issue, setIssue] = useState("");
  const changes = instructionDiff(before, after);
  return (
    <details className="vibe-panel text-sm" open={open} onToggle={event => setOpen(event.currentTarget.open)}>
      <summary>Review the instruction change</summary>
      <div className="space-y-4 border-t border-[var(--vibe-border)] p-4">
        <p className="whitespace-pre-wrap text-sm [overflow-wrap:anywhere]" aria-label="Instruction changes">
          {changes.map((change, i) => change.kind === "removed" ? (
            <del key={i} className="vibe-diff-removed"><span className="sr-only">Removed: </span>{change.text}</del>
          ) : change.kind === "added" ? (
            <ins key={i} className="vibe-diff-added"><span className="sr-only">Added: </span>{change.text}</ins>
          ) : <span key={i}>{diffContext(change.text, i === 0, i === changes.length - 1)}</span>)}
        </p>
        <details>
          <summary className="cursor-pointer vibe-muted">Full updated instructions</summary>
          <pre className="mt-3 whitespace-pre-wrap font-sans [overflow-wrap:anywhere]">{after}</pre>
        </details>
        <p className="vibe-muted">
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
          <p role="status" className="vibe-muted">
            {issue}
          </p>
        )}
      </div>
    </details>
  );
}

export function CopyNewInstructions({ after, busy = false }: { after: string; busy?: boolean }) {
  const [copied, setCopied] = useState(""), [fallback, setFallback] = useState("");
  return <section aria-label="Use the new instructions" className="space-y-3">
    <VibeButton variant="primary" disabled={busy} onClick={async () => {
      try { await navigator.clipboard.writeText(after); setCopied(after); setFallback(""); }
      catch { setFallback(after); }
    }}><Copy aria-hidden="true" />{copied === after ? "Copy the new instructions · copied" : "Copy the new instructions"}</VibeButton>
    <p className="text-sm vibe-muted">Paste them into your app. We don&apos;t change your live app.</p>
    {fallback === after && <><p role="status">Select and copy the instructions below.</p><textarea className="vibe-textarea" aria-label="New instructions to copy" readOnly value={after} onFocus={event => event.currentTarget.select()} /></>}
  </section>;
}
