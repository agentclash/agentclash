"use client";

import { AlertTriangle, CheckCircle2 } from "lucide-react";
import { MOCK_STRATEGY_OPTIONS, operationLabel, typeLabel } from "./lib/friendly";
import { schemaToParams } from "./lib/definition";
import type { ToolDefinition, ValidationIssue } from "./lib/types";

/**
 * A plain-language summary of what the tool does, plus the current validation
 * state. The exact persisted JSON is tucked behind a disclosure for the rare
 * reader who wants it — non-engineers never have to look at it.
 */
export function DefinitionPreview({
  definition,
  issues,
}: {
  definition: ToolDefinition;
  issues: ValidationIssue[];
}) {
  const params = schemaToParams(definition.parameters);

  return (
    <div className="space-y-3">
      {issues.length === 0 ? (
        <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
          <CheckCircle2 className="size-4 text-emerald-500" />
          Ready to save.
        </div>
      ) : (
        <div className="space-y-1 rounded-lg border border-destructive/30 bg-destructive/5 p-3">
          <div className="flex items-center gap-2 text-xs font-medium text-destructive">
            <AlertTriangle className="size-4" />
            {issues.length} thing{issues.length > 1 ? "s" : ""} to finish
          </div>
          <ul className="space-y-0.5">
            {issues.map((issue, i) => (
              <li key={i} className="text-xs text-destructive/90">
                {issue.message}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="rounded-lg border border-border bg-muted/30 p-3 text-xs">
        <div className="font-medium">What this tool does</div>
        <p className="mt-1 text-muted-foreground">{describe(definition)}</p>

        <div className="mt-3 font-medium">Inputs from the agent</div>
        {params.length === 0 ? (
          <p className="mt-1 text-muted-foreground">None.</p>
        ) : (
          <ul className="mt-1 space-y-0.5 text-muted-foreground">
            {params.map((p) => (
              <li key={p.name}>
                <span className="text-foreground">{p.name || "(unnamed)"}</span> ·{" "}
                {typeLabel(p.type)}
                {p.required ? " · required" : ""}
              </li>
            ))}
          </ul>
        )}
      </div>

      <details className="group">
        <summary className="cursor-pointer text-xs text-muted-foreground transition-colors hover:text-foreground">
          Show technical definition (JSON)
        </summary>
        <pre className="mt-2 max-h-80 overflow-auto rounded-lg border border-border bg-muted/30 p-3 font-[family-name:var(--font-mono)] text-[11px] leading-relaxed">
          {JSON.stringify(definition, null, 2)}
        </pre>
      </details>
    </div>
  );
}

function describe(def: ToolDefinition): string {
  if (def.tool_type === "primitive") {
    const impl = def.implementation;
    if (impl.mode === "mock") {
      const strategy = MOCK_STRATEGY_OPTIONS.find((s) => s.value === impl.mock?.strategy);
      return `Returns a canned response — ${
        strategy ? strategy.label.toLowerCase() : "for testing"
      }. Nothing real is called.`;
    }
    if (!impl.primitive) return "Performs one operation — pick which one above.";
    return `Performs one operation: ${operationLabel(impl.primitive)}.`;
  }

  if (def.steps.length === 0) return "Runs a sequence of steps — add the first one above.";
  const parts = def.steps.map((s, i) => {
    const what =
      s.ref.type === "primitive"
        ? s.ref.name
          ? operationLabel(s.ref.name)
          : "(unset)"
        : s.ref.name || "(unset tool)";
    return `${i + 1}. ${what}`;
  });
  return `Runs ${def.steps.length} step${def.steps.length > 1 ? "s" : ""} in order — ${parts.join(", ")}.`;
}
