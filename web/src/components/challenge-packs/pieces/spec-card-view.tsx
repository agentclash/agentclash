"use client";

import Editor from "@monaco-editor/react";
import { Loader2 } from "lucide-react";
import { useState, type ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { SpecCard } from "../lib/types";

type View = "readable" | "yaml";

/**
 * Pure, presentational renderer for a compiled pack's spec card + YAML. Shared
 * by the builder preview (which wraps it with usePackDraft + a validation
 * footer) and the catalog/library detail view (which passes a static card +
 * yaml). It owns no data fetching and no draft context.
 */
export function SpecCardView({
  card,
  yaml,
  compiling = false,
  footer,
}: {
  card: SpecCard | undefined;
  yaml?: string;
  compiling?: boolean;
  footer?: ReactNode;
}) {
  const [view, setView] = useState<View>("readable");

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

      {compiling && !card && (
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
            value={yaml ?? ""}
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

            {footer}
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
