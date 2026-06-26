import { Bot, Check, Sparkles } from "lucide-react";

import { cn } from "@/lib/utils";

const MICRO = "font-mono text-2xs uppercase tracking-[0.22em]";

/*
 * "Design your agent" — the front of the playground loop.
 *
 * A user authors the agent's system prompt, picks the abilities (tools, drawn
 * from the real tool library) it's allowed to use, and a model. The resulting
 * draft is what gets run, graded, coached, and re-run — so the score delta is
 * attributable to *their* choices, which is the whole point of the playground.
 *
 * Controlled + presentational: state lives in the parent (welcome flow / preview).
 */

export type AgentTool = {
  slug: string;
  name: string;
  category: string;
  /** Short verb-y blurb, e.g. "search the web". */
  blurb?: string;
};

export type AgentModelChoice = {
  key: string;
  label: string;
  hint?: string;
};

export type AgentDraft = {
  name: string;
  instructions: string;
  toolSlugs: string[];
  modelKey: string;
};

function Label({ children, hint }: { children: React.ReactNode; hint?: string }) {
  return (
    <div className="mb-2.5 flex items-baseline justify-between gap-3">
      <span className={cn(MICRO, "text-white/45")}>{children}</span>
      {hint ? <span className="text-2xs text-white/30">{hint}</span> : null}
    </div>
  );
}

function ToolChip({
  tool,
  selected,
  onToggle,
}: {
  tool: AgentTool;
  selected: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={selected}
      className={cn(
        "group flex items-center gap-2 rounded-sm border px-2.5 py-1.5 text-left transition",
        selected
          ? "border-white/40 bg-white/[0.06] text-white"
          : "border-white/10 bg-transparent text-white/55 hover:border-white/25 hover:text-white/80",
      )}
    >
      <span
        className={cn(
          "flex size-4 shrink-0 items-center justify-center rounded-[3px] border transition",
          selected ? "border-white/50 bg-white text-black" : "border-white/20 text-transparent",
        )}
      >
        <Check className="size-3" strokeWidth={3} />
      </span>
      <span className="min-w-0">
        <span className="block truncate text-sm leading-5">{tool.name}</span>
        {tool.blurb ? (
          <span className="block truncate text-2xs text-white/35">{tool.blurb}</span>
        ) : null}
      </span>
    </button>
  );
}

export function AgentDesigner({
  value,
  onChange,
  tools,
  models,
  taskLabel,
  onRun,
  running,
  bare,
}: {
  value: AgentDraft;
  onChange: (next: AgentDraft) => void;
  tools: AgentTool[];
  models: AgentModelChoice[];
  taskLabel?: string;
  onRun?: () => void;
  running?: boolean;
  /**
   * Render without an outer card border/background so the designer can sit
   * inside another container (e.g. the tryout composer) as one unified box.
   */
  bare?: boolean;
}) {
  const grouped = tools.reduce<Record<string, AgentTool[]>>((acc, tool) => {
    (acc[tool.category] ??= []).push(tool);
    return acc;
  }, {});
  const categories = Object.keys(grouped);
  const selected = new Set(value.toolSlugs);

  const toggleTool = (slug: string) => {
    const next = new Set(selected);
    if (next.has(slug)) next.delete(slug);
    else next.add(slug);
    onChange({ ...value, toolSlugs: [...next] });
  };

  const padX = bare ? "px-2 sm:px-3" : "px-5 sm:px-6";

  return (
    <div
      className={cn(
        bare
          ? "border-t border-white/[0.07]"
          : "rounded-lg border border-white/10 bg-white/[0.015]",
      )}
    >
      {/* Identity header */}
      <div
        className={cn(
          "flex items-center gap-3",
          padX,
          bare ? "pb-2 pt-3" : "border-b border-white/[0.07] py-4",
        )}
      >
        <span className="flex size-9 shrink-0 items-center justify-center rounded-full border border-white/15 bg-white/[0.04]">
          <Bot className="size-4 text-white/70" />
        </span>
        <div className="min-w-0 flex-1">
          <input
            value={value.name}
            onChange={(event) => onChange({ ...value, name: event.target.value })}
            placeholder="Name your agent"
            className="w-full bg-transparent text-base font-medium text-white placeholder:text-white/30 focus:outline-none"
          />
          <p className={cn(MICRO, "mt-0.5 text-white/35")}>
            {taskLabel ? `Built for: ${taskLabel}` : "Your agent"}
          </p>
        </div>
      </div>

      <div className={cn("space-y-6", padX, bare ? "pb-3 pt-1" : "py-5")}>
        {/* System prompt — the centerpiece */}
        <div>
          <Label hint="read before every run">Instructions</Label>
          <textarea
            value={value.instructions}
            onChange={(event) => onChange({ ...value, instructions: event.target.value })}
            rows={6}
            placeholder="Tell the agent how to behave. Be specific — vague prompts fail the bar."
            className="w-full resize-y rounded-md border border-white/10 bg-black/20 p-3.5 text-sm leading-6 text-white/85 placeholder:text-white/25 focus:border-white/25 focus:bg-black/30 focus:outline-none"
          />
          <p className="mt-1.5 text-2xs leading-5 text-white/30">
            This is your agent&apos;s system prompt. The judge grades what it produces against
            your bar — tighten this and the score moves.
          </p>
        </div>

        {/* Abilities — drawn from the tool library */}
        <div>
          <Label hint={`${selected.size} selected`}>Abilities</Label>
          <div className="space-y-3">
            {categories.map((category) => (
              <div key={category}>
                <p className="mb-2 text-2xs uppercase tracking-[0.16em] text-white/25">
                  {category}
                </p>
                <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                  {grouped[category].map((tool) => (
                    <ToolChip
                      key={tool.slug}
                      tool={tool}
                      selected={selected.has(tool.slug)}
                      onToggle={() => toggleTool(tool.slug)}
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Model */}
        <div>
          <Label hint="the brain">Model</Label>
          <div className="flex flex-wrap gap-2">
            {models.map((model) => {
              const active = model.key === value.modelKey;
              return (
                <button
                  key={model.key}
                  type="button"
                  onClick={() => onChange({ ...value, modelKey: model.key })}
                  aria-pressed={active}
                  className={cn(
                    "rounded-sm border px-3 py-1.5 text-sm transition",
                    active
                      ? "border-white/40 bg-white/[0.06] text-white"
                      : "border-white/10 text-white/55 hover:border-white/25 hover:text-white/80",
                  )}
                  title={model.hint}
                >
                  {model.label}
                </button>
              );
            })}
          </div>
        </div>
      </div>

      {onRun ? (
        <div className="flex items-center justify-between gap-3 border-t border-white/[0.07] px-5 py-4 sm:px-6">
          <p className="text-xs leading-5 text-white/40">
            You&apos;ll watch it work, then a judge grades it against your bar.
          </p>
          <button
            type="button"
            onClick={onRun}
            disabled={running}
            className="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-sm bg-white px-4 text-sm font-medium text-black transition hover:bg-white/90 disabled:opacity-60"
          >
            <Sparkles className="size-4" />
            {running ? "Running…" : "Run your agent"}
          </button>
        </div>
      ) : null}
    </div>
  );
}
