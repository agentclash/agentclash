"use client";

// Per-case multi-turn conversation flow editor. UserSimulatorSpec.phases is a
// strictly ordered sequence (scripted -> llm -> human) with trigger/until
// transitions, so this is an ordered list editor, not a free-form canvas.

import { Plus, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import type {
  UserSimulatorActor,
  UserSimulatorPhase,
  UserSimulatorSpec,
  UserSimulatorTrigger,
} from "../lib/types";
import { BuilderSelect } from "../ui/builder-select";
import { controlClass, Field, FieldRow, monoControlClass } from "../ui/form";

const ACTORS: { value: UserSimulatorActor; label: string }[] = [
  { value: "scripted", label: "Scripted — fixed messages" },
  { value: "llm", label: "LLM — simulated persona" },
  { value: "human", label: "Human — wait for a real reviewer" },
];

const TRIGGERS: UserSimulatorTrigger[] = [
  "always",
  "on_assistant_mismatch",
  "on_validator_fail",
  "on_judge_below",
  "on_agent_loop",
  "on_max_llm_turns",
  "manual",
  "never",
];

const ON_TIMEOUT = ["stop", "fail"];

function defaultSimulator(): UserSimulatorSpec {
  return {
    schema_version: 1,
    kind: "hybrid",
    phases: [{ id: "open", actor: "scripted", turns: [{ message: "" }] }],
  };
}

export function CaseFlowEditor({
  value,
  onChange,
}: {
  value?: UserSimulatorSpec;
  onChange: (next: UserSimulatorSpec) => void;
}) {
  if (!value) {
    return (
      <div className="rounded-md border border-dashed border-builder-border p-3">
        <p className="mb-2 text-xs text-builder-fg-muted">
          Multi-turn mode: this case needs a conversation flow.
        </p>
        <Button size="xs" variant="outline" onClick={() => onChange(defaultSimulator())}>
          <Plus className="size-3.5" /> Add conversation flow
        </Button>
      </div>
    );
  }

  const phases = value.phases ?? [];
  const setPhase = (i: number, patch: Partial<UserSimulatorPhase>) =>
    onChange({ ...value, phases: phases.map((p, j) => (j === i ? { ...p, ...patch } : p)) });
  const addPhase = () =>
    onChange({
      ...value,
      phases: [
        ...phases,
        { id: `phase-${phases.length + 1}`, actor: "scripted", turns: [{ message: "" }] },
      ],
    });
  const removePhase = (i: number) =>
    onChange({ ...value, phases: phases.filter((_, j) => j !== i) });

  return (
    <div className="space-y-3 rounded-md border border-builder-border bg-builder-surface p-3">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium uppercase tracking-wide text-builder-fg-subtle">
          Conversation ({phases.length} phase{phases.length === 1 ? "" : "s"})
        </span>
        <Button size="xs" variant="outline" onClick={addPhase}>
          <Plus className="size-3.5" /> Add phase
        </Button>
      </div>

      {phases.map((phase, i) => (
        <div key={i} className="space-y-3 rounded-md border border-builder-border bg-builder-panel p-3">
          <div className="flex items-center gap-2">
            <input
              className={monoControlClass}
              value={phase.id}
              onChange={(e) => setPhase(i, { id: e.target.value })}
              placeholder="phase id"
            />
            <BuilderSelect
              ariaLabel="Actor"
              className="w-56"
              value={phase.actor}
              onChange={(v) => setPhase(i, { actor: v as UserSimulatorActor })}
              options={ACTORS}
            />
            <Button
              size="icon-sm"
              variant="ghost"
              onClick={() => removePhase(i)}
              aria-label="Remove phase"
            >
              <Trash2 className="size-4" />
            </Button>
          </div>

          {i > 0 && (
            <FieldRow label="Trigger" hint="when this phase activates">
              <BuilderSelect
                ariaLabel="Trigger"
                value={phase.trigger ?? "always"}
                onChange={(v) => setPhase(i, { trigger: v as UserSimulatorTrigger })}
                options={TRIGGERS.map((t) => ({ value: t, label: t }))}
              />
            </FieldRow>
          )}

          {phase.actor === "scripted" && (
            <ScriptedTurns
              turns={(phase.turns ?? []).map((t) => t.message)}
              onChange={(messages) => setPhase(i, { turns: messages.map((message) => ({ message })) })}
            />
          )}

          {phase.actor === "llm" && (
            <div className="space-y-3">
              <Field label="Persona">
                <textarea
                  className={controlClass}
                  rows={2}
                  value={phase.persona ?? ""}
                  onChange={(e) => setPhase(i, { persona: e.target.value })}
                  placeholder="Frustrated customer who accepts a clear timeline if treated with empathy"
                />
              </Field>
              <FieldRow label="Max turns">
                <input
                  className={controlClass}
                  type="number"
                  min="1"
                  value={phase.max_turns ?? ""}
                  onChange={(e) =>
                    setPhase(i, { max_turns: e.target.value ? Number(e.target.value) : undefined })
                  }
                  placeholder="3"
                />
              </FieldRow>
              <FieldRow label="Model" hint="optional override">
                <input
                  className={monoControlClass}
                  value={phase.model ?? ""}
                  onChange={(e) => setPhase(i, { model: e.target.value })}
                />
              </FieldRow>
              <FieldRow label="Exit when" hint='comma-separated, e.g. assistant_emitted:timeline'>
                <input
                  className={monoControlClass}
                  value={(phase.until ?? []).join(", ")}
                  onChange={(e) =>
                    setPhase(i, {
                      until: e.target.value
                        .split(",")
                        .map((s) => s.trim())
                        .filter(Boolean),
                    })
                  }
                />
              </FieldRow>
            </div>
          )}

          {phase.actor === "human" && (
            <div className="space-y-3">
              <FieldRow label="Timeout (ms)">
                <input
                  className={controlClass}
                  type="number"
                  min="0"
                  value={phase.timeout_ms ?? ""}
                  onChange={(e) =>
                    setPhase(i, { timeout_ms: e.target.value ? Number(e.target.value) : undefined })
                  }
                  placeholder="900000"
                />
              </FieldRow>
              <FieldRow label="On timeout">
                <BuilderSelect
                  ariaLabel="On timeout"
                  value={phase.on_timeout ?? "stop"}
                  onChange={(v) => setPhase(i, { on_timeout: v })}
                  options={ON_TIMEOUT.map((t) => ({ value: t, label: t }))}
                />
              </FieldRow>
            </div>
          )}
        </div>
      ))}
    </div>
  );
}

function ScriptedTurns({
  turns,
  onChange,
}: {
  turns: string[];
  onChange: (turns: string[]) => void;
}) {
  return (
    <Field label="User messages">
      <div className="space-y-2">
        {turns.map((message, i) => (
          <div key={i} className="flex items-start gap-2">
            <textarea
              className={controlClass}
              rows={2}
              value={message}
              onChange={(e) => onChange(turns.map((t, j) => (j === i ? e.target.value : t)))}
              placeholder="I need a refund for order {{order_id}} right now."
            />
            <Button
              size="icon-sm"
              variant="ghost"
              onClick={() => onChange(turns.filter((_, j) => j !== i))}
              aria-label="Remove message"
            >
              <Trash2 className="size-4" />
            </Button>
          </div>
        ))}
        <Button size="xs" variant="outline" onClick={() => onChange([...turns, ""])}>
          <Plus className="size-3.5" /> Add message
        </Button>
      </div>
    </Field>
  );
}
