"use client";

import Editor from "@monaco-editor/react";
import { AlertCircle, CheckCircle2, Loader2 } from "lucide-react";
import { useState } from "react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { usePackDraft } from "../use-pack-draft";

type View = "readable" | "yaml";

export function SpecCard() {
  const { state } = usePackDraft();
  const [view, setView] = useState<View>("readable");
  const compile = state.compile;
  const card = compile?.spec_card;

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center justify-between border-b border-border px-4 py-2.5">
        <span className="text-sm font-medium">Preview</span>
        <div className="flex rounded-md border border-border p-0.5 text-xs">
          {(["readable", "yaml"] as View[]).map((v) => (
            <button
              key={v}
              type="button"
              onClick={() => setView(v)}
              className={cn(
                "rounded px-2 py-0.5 capitalize transition-colors",
                view === v ? "bg-foreground text-background" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {v === "yaml" ? "YAML" : "Readable"}
            </button>
          ))}
        </div>
      </div>

      {state.compiling && !card && (
        <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
          <Loader2 className="mr-2 size-4 animate-spin" /> Building preview…
        </div>
      )}

      {view === "yaml" ? (
        <div className="min-h-0 flex-1">
          <Editor
            height="100%"
            defaultLanguage="yaml"
            theme="vs-dark"
            value={compile?.yaml ?? ""}
            options={{
              readOnly: true,
              minimap: { enabled: false },
              fontSize: 12,
              wordWrap: "on",
              scrollBeyondLastLine: false,
            }}
            loading={
              <div className="flex h-full items-center justify-center text-muted-foreground">
                <Loader2 className="size-5 animate-spin" />
              </div>
            }
          />
        </div>
      ) : (
        card && (
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto p-4">
            <div>
              <div className="text-sm font-semibold">{card.pack_name || "Untitled pack"}</div>
              <div className="mt-1 flex flex-wrap gap-1.5">
                {card.family && <Badge variant="outline">{card.family}</Badge>}
                <Badge variant="outline">{card.execution_mode}</Badge>
                <Badge variant="outline">{card.strategy}</Badge>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-2 text-xs">
              <Stat label="Challenges" value={card.challenge_count} />
              <Stat label="Cases" value={card.case_count} />
              <Stat label="Validators" value={card.validator_count} />
              <Stat label="Judges" value={card.judge_count} />
            </div>

            <div className="rounded-lg border border-border bg-muted/40 p-3 text-sm">
              {card.pass_criteria}
            </div>

            {card.dimensions.length > 0 && (
              <div className="space-y-1.5">
                <div className="text-xs font-medium text-muted-foreground">Dimensions</div>
                {card.dimensions.map((d) => (
                  <div key={d.key} className="rounded-md border border-border px-2.5 py-1.5 text-xs">
                    {d.summary}
                  </div>
                ))}
              </div>
            )}

            <ValidationPanel valid={compile?.valid ?? false} errors={compile?.errors ?? []} />
          </div>
        )
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-md border border-border px-2.5 py-1.5">
      <div className="text-base font-semibold tabular-nums">{value}</div>
      <div className="text-muted-foreground">{label}</div>
    </div>
  );
}

function ValidationPanel({
  valid,
  errors,
}: {
  valid: boolean;
  errors: { field: string; message: string }[];
}) {
  if (valid) {
    return (
      <div className="flex items-center gap-2 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-3 py-2 text-sm text-emerald-600 dark:text-emerald-400">
        <CheckCircle2 className="size-4" /> Ready to publish
      </div>
    );
  }
  return (
    <div className="space-y-1.5 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3">
      <div className="flex items-center gap-2 text-sm font-medium text-amber-600 dark:text-amber-400">
        <AlertCircle className="size-4" /> {errors.length} issue{errors.length === 1 ? "" : "s"} to resolve
      </div>
      <ul className="space-y-1 text-xs text-muted-foreground">
        {errors.map((e, i) => (
          <li key={i}>
            <span className="font-mono text-foreground">{e.field}</span> — {e.message}
          </li>
        ))}
      </ul>
    </div>
  );
}
