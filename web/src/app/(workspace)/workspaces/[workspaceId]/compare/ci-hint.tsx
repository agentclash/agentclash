"use client";

import { useState } from "react";
import { ChevronDown, ChevronRight, Terminal, Copy, Check } from "lucide-react";

interface CiHintProps {
  baselineRunId: string;
  candidateRunId: string;
}

export function CiHint({ baselineRunId, candidateRunId }: CiHintProps) {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);

  const curlCommand = `curl -s -X POST "$API_URL/v1/release-gates/evaluate" \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "baseline_run_id": "${baselineRunId}",
    "candidate_run_id": "${candidateRunId}",
    "policy": {}
  }'`;

  const responseExample = `{
  "baseline_run_id": "${baselineRunId}",
  "candidate_run_id": "${candidateRunId}",
  "release_gate": {
    "verdict": "pass | warn | fail | insufficient_evidence",
    "reason_code": "within_thresholds",
    "summary": "All metrics within acceptable thresholds",
    "evidence_status": "sufficient"
  }
}`;

  function handleCopy() {
    navigator.clipboard.writeText(curlCommand);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="mt-4">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
      >
        {expanded ? (
          <ChevronDown className="size-3" />
        ) : (
          <ChevronRight className="size-3" />
        )}
        <Terminal className="size-3" />
        CI/CD Integration
      </button>

      {expanded && (
        <div className="mt-2 rounded-lg border border-border bg-card p-4 space-y-4">
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <p className="text-xs font-medium">
                Evaluate in your pipeline
              </p>
              <button
                onClick={handleCopy}
                className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                {copied ? (
                  <Check className="size-3 text-emerald-400" />
                ) : (
                  <Copy className="size-3" />
                )}
                {copied ? "Copied" : "Copy"}
              </button>
            </div>
            <pre className="rounded-md bg-muted/50 px-3 py-2 text-xs font-[family-name:var(--font-mono)] overflow-x-auto whitespace-pre">
              {curlCommand}
            </pre>
          </div>

          <div>
            <p className="text-xs font-medium mb-1.5">Expected response</p>
            <pre className="rounded-md bg-muted/50 px-3 py-2 text-xs font-[family-name:var(--font-mono)] overflow-x-auto whitespace-pre text-muted-foreground">
              {responseExample}
            </pre>
          </div>

          <p className="text-xs text-muted-foreground">
            Pass an empty <code className="text-foreground">{"{}"}</code> as the
            policy to use the default thresholds. Check the{" "}
            <code className="text-foreground">verdict</code> field to gate your
            deployment: <code className="text-emerald-400">&quot;pass&quot;</code>{" "}
            means safe to ship.
          </p>
        </div>
      )}
    </div>
  );
}
