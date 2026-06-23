"use client";

// Per-case multi-turn conversation flow editor. UserSimulatorSpec.phases is a
// strictly ordered sequence (scripted -> llm -> human) with trigger/until
// transitions, so this is an ordered list editor, not a free-form canvas.

import { Plus, Trash2 } from "lucide-react";

import { Field, controlClass } from "@/components/tools/field";
import { Button } from "@/components/ui/button";
import type {
  UserSimulatorActor,
  UserSimulatorPhase,
  UserSimulatorSpec,
  UserSimulatorTrigger,
} from "../lib/types";

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
      <div className="rounded-md border border-dashed border-border p-3">
        <p className="mb-2 text-xs text-muted-foreground">
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
      phases: [...phases, { id: `phase-${phases.length + 1}`, actor: "scripted", turns: [{ message: "" }] }],
    });
  const removePhase = (i: number) =>
    onChange({ ...value, phases: phases.filter((_, j) => j !== i) });

  return (
    <div className="space-y-3 rounded-md border border-border bg-muted/30 p-3">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
          Conversation ({phases.length} phase{phases.length === 1 ? "" : "s"})
        </span>
        <Button size="xs" variant="outline" onClick={addPhase}>
          <Plus className="size-3.5" /> Add phase
        </Button>
      </div>

      {phases.map((phase, i) => (
        <div key={i} className="space-y-3 rounded-md border border-border bg-background p-3">
          <div className="flex items-center gap-2">
            <input
              className={controlClass}
              value={phase.id}
              onChange={(e) => setPhase(i, { id: e.target.value })}
              placeholder="phase id"
            />
            <select
              className={controlClass}
              value={phase.actor}
              onChange={(e) => setPhase(i, { actor: e.target.value as UserSimulatorActor })}
            >
              {ACTORS.map((a) => (
                <option key={a.value} value={a.value}>
                  {a.label}
                </option>
              ))}
            </select>
            <Button size="icon-sm" variant="ghost" onClick={() => removePhase(i)} aria-label="Remove phase">
              <Trash2 className="size-4" />
            </Button>
          </div>

          {i > 0 && (
            <Field label="Trigger" hint="when this phase activates">
              <select
                className={controlClass}
                value={phase.trigger ?? "always"}
                onChange={(e) => setPhase(i, { trigger: e.target.value as UserSimulatorTrigger })}
              >
                {TRIGGERS.map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
              </select>
            </Field>
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
              <div className="grid grid-cols-2 gap-3">
                <Field label="Max turns">
                  <input
                    className={controlClass}
                    type="number"
                    min="1"
                    value={phase.max_turns ?? ""}
                    onChange={(e) =>
                      setPhase(i, {
                        max_turns: e.target.value ? Number(e.target.value) : undefined,
                      })
                    }
                    placeholder="3"
                  />
                </Field>
                <Field label="Model" hint="optional override">
                  <input
                    className={controlClass}
                    value={phase.model ?? ""}
                    onChange={(e) => setPhase(i, { model: e.target.value })}
                  />
                </Field>
              </div>
              <Field label="Exit when" hint='comma-separated, e.g. assistant_emitted:timeline'>
                <input
                  className={controlClass}
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
              </Field>
            </div>
          )}

          {phase.actor === "human" && (
            <div className="grid grid-cols-2 gap-3">
              <Field label="Timeout (ms)">
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
              </Field>
              <Field label="On timeout">
                <select
                  className={controlClass}
                  value={phase.on_timeout ?? "stop"}
                  onChange={(e) => setPhase(i, { on_timeout: e.target.value })}
                >
                  <option value="stop">stop</option>
                  <option value="fail">fail</option>
                </select>
              </Field>
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
