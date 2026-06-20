"use client";

import { CheckCircle2, AlertTriangle } from "lucide-react";
import type { ToolDefinition, ValidationIssue } from "./lib/types";

/**
 * Read-only, forward-rendered view of the compiled tool definition plus the
 * current validation state. One-way (builder → definition) by design; this is
 * the artifact that gets persisted.
 */
export function DefinitionPreview({
  definition,
  issues,
}: {
  definition: ToolDefinition;
  issues: ValidationIssue[];
}) {
  return (
    <div className="space-y-3">
      {issues.length === 0 ? (
        <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
          <CheckCircle2 className="size-4 text-emerald-500" />
          Definition is valid.
        </div>
      ) : (
        <div className="space-y-1 rounded-lg border border-destructive/30 bg-destructive/5 p-3">
          <div className="flex items-center gap-2 text-xs font-medium text-destructive">
            <AlertTriangle className="size-4" />
            {issues.length} issue{issues.length > 1 ? "s" : ""} to fix
          </div>
          <ul className="space-y-0.5">
            {issues.map((issue, i) => (
              <li key={i} className="text-xs text-destructive/90">
                <code className="font-[family-name:var(--font-mono)]">{issue.path}</code>
                {" — "}
                {issue.message}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div>
        <div className="mb-1.5 text-xs font-medium text-muted-foreground">
          Compiled definition
        </div>
        <pre className="max-h-96 overflow-auto rounded-lg border border-border bg-muted/30 p-3 font-[family-name:var(--font-mono)] text-[11px] leading-relaxed">
          {JSON.stringify(definition, null, 2)}
        </pre>
      </div>
    </div>
  );
}
