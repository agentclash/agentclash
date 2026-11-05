"use client";

import { useMemo, useState, type ReactNode } from "react";
import { Copy } from "lucide-react";
import { lineDiff, type Change } from "./instruction-diff";
import { VibeButton } from "./vibe-button";

// Retain at most three context lines on each side of a change.
function foldedLines(lines: Change[]): (Change | { folded: number })[] {
  const out: (Change | { folded: number })[] = [];
  for (let i = 0; i < lines.length;) {
    if (lines[i].kind !== "same") { out.push(lines[i++]); continue; }
    let end = i;
    while (end < lines.length && lines[end].kind === "same") end++;
    const noChanges = i === 0 && end === lines.length;
    const head = i > 0 || noChanges ? Math.min(3, end - i) : 0;
    const tail = end < lines.length || noChanges ? Math.min(3, end - i - head) : 0;
    for (let j = i; j < i + head; j++) out.push(lines[j]);
    const folded = end - i - head - tail;
    if (folded) out.push({ folded });
    for (let j = end - tail; j < end; j++) out.push(lines[j]);
    i = end;
  }
  return out;
}

export function PromptChange({
  before,
  after,
  defaultOpen = false,
  title = "Proposed change",
  beforeVersion,
  afterVersion,
  actions,
}: {
  before: string;
  after: string;
  defaultOpen?: boolean;
  title?: string;
  beforeVersion?: number;
  afterVersion?: number;
  actions?: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const [copied, setCopied] = useState(false);
  const [issue, setIssue] = useState("");
  const changes = useMemo(() => foldedLines(lineDiff(before, after)), [before, after]);
  return (
    <div className="vibe-prompt-change">
      <details open={open} onToggle={event => setOpen(event.currentTarget.open)}>
        <summary className="vibe-prompt-change-heading">
          <span>{title}</span>
          {beforeVersion !== undefined && afterVersion !== undefined && <span className="vibe-prompt-change-versions">v{beforeVersion} → v{afterVersion}</span>}
        </summary>
        <div className="vibe-line-diff" aria-label="Instruction changes">
          {changes.map((change, i) => "folded" in change ? (
            <div key={i} className="vibe-diff-fold">{change.folded} unchanged {change.folded === 1 ? "line" : "lines"}</div>
          ) : (
            <div key={i} className={`vibe-diff-line vibe-diff-line-${change.kind}`}>
              <span className="vibe-diff-marker" aria-hidden="true">{change.kind === "added" ? "+" : change.kind === "removed" ? "−" : ""}</span>
              {change.kind === "removed" ? <del className="vibe-diff-line-text"><span className="sr-only">Removed: </span>{change.text}</del>
                : change.kind === "added" ? <ins className="vibe-diff-line-text"><span className="sr-only">Added: </span>{change.text}</ins>
                : <span className="vibe-diff-line-text">{change.text}</span>}
            </div>
          ))}
        </div>
        <div className="vibe-prompt-change-more">
          <details>
            <summary>Full updated instructions</summary>
            <pre>{after}</pre>
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
      {actions != null && <footer className="vibe-prompt-change-actions">{actions}</footer>}
    </div>
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
