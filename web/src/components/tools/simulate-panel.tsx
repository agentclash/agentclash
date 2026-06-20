"use client";

import { useState } from "react";
import { Play } from "lucide-react";
import { controlClass } from "./field";
import { simulate } from "./lib/simulate";
import type { ToolDefinition } from "./lib/types";

/**
 * A client-side dry run: the user enters sample parameter values and sees how
 * placeholders resolve for each primitive call / step. No sandbox execution.
 */
export function SimulatePanel({
  definition,
  paramNames,
}: {
  definition: ToolDefinition;
  paramNames: string[];
}) {
  const [sample, setSample] = useState<Record<string, string>>({});
  const result = simulate(definition, sample);

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <Play className="size-3.5" />
        Enter example inputs to preview what each step would receive. Nothing actually runs.
      </div>

      {paramNames.length === 0 ? (
        <p className="text-xs text-muted-foreground">This tool takes no inputs.</p>
      ) : (
        <div className="space-y-2">
          {paramNames.map((name) => (
            <div key={name} className="grid grid-cols-[10rem_1fr] items-center gap-2">
              <code className="font-[family-name:var(--font-mono)] text-xs text-muted-foreground">
                {name}
              </code>
              <input
                value={sample[name] ?? ""}
                onChange={(e) => setSample((p) => ({ ...p, [name]: e.target.value }))}
                placeholder="sample value"
                className={`${controlClass} text-xs`}
              />
            </div>
          ))}
        </div>
      )}

      <div>
        <div className="mb-1.5 text-xs font-medium text-muted-foreground">What gets sent</div>
        {result.note ? (
          <p className="rounded-lg border border-border bg-muted/30 p-3 text-xs text-muted-foreground">
            {result.note}
          </p>
        ) : result.args ? (
          <pre className="overflow-auto rounded-lg border border-border bg-muted/30 p-3 font-[family-name:var(--font-mono)] text-[11px] leading-relaxed">
            {JSON.stringify(result.args, null, 2)}
          </pre>
        ) : (
          <div className="space-y-2">
            {(result.steps ?? []).map((step) => (
              <div key={step.id} className="rounded-lg border border-border bg-muted/30 p-2">
                <div className="mb-1 flex items-center gap-2 text-xs">
                  <span className="font-medium">{step.id}</span>
                  <code className="font-[family-name:var(--font-mono)] text-muted-foreground">
                    {step.ref}
                  </code>
                </div>
                <pre className="overflow-auto font-[family-name:var(--font-mono)] text-[11px] leading-relaxed">
                  {JSON.stringify(step.inputs, null, 2)}
                </pre>
              </div>
            ))}
            {(result.steps ?? []).length === 0 && (
              <p className="text-xs text-muted-foreground">Add steps to simulate.</p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
