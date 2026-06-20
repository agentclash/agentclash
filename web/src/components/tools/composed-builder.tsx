"use client";

import { useRef } from "react";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Plus, Workflow } from "lucide-react";
import { ParametersEditor } from "./parameters-editor";
import { StepCard } from "./step-card";
import {
  declaredParamNames,
  newStepId,
  paramsToSchema,
  reorder,
  schemaToParams,
} from "./lib/definition";
import type { ComposedDefinition, ComposedStep, ToolPrimitive } from "./lib/types";

export function ComposedBuilder({
  def,
  onChange,
  primitives,
  tools,
}: {
  def: ComposedDefinition;
  onChange: (def: ComposedDefinition) => void;
  primitives: ToolPrimitive[];
  tools: { slug: string; name: string }[];
}) {
  const dragIndex = useRef<number | null>(null);
  const params = schemaToParams(def.parameters);
  const paramNames = declaredParamNames(def);

  function setSteps(steps: ComposedStep[]) {
    onChange({ ...def, steps });
  }
  function addStep() {
    const step: ComposedStep = {
      id: newStepId(def.steps),
      ref: { type: "primitive", name: "" },
      inputs: {},
    };
    setSteps([...def.steps, step]);
  }
  function updateStep(index: number, step: ComposedStep) {
    setSteps(def.steps.map((s, i) => (i === index ? step : s)));
  }
  function removeStep(index: number) {
    setSteps(def.steps.filter((_, i) => i !== index));
  }
  function move(from: number, to: number) {
    setSteps(reorder(def.steps, from, to));
  }

  return (
    <div className="space-y-6">
      <ParametersEditor
        params={params}
        onChange={(next) => onChange({ ...def, parameters: paramsToSchema(next) })}
      />

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium">Steps</h3>
          <Button type="button" variant="outline" size="sm" onClick={addStep}>
            <Plus data-icon="inline-start" className="size-3.5" />
            Add step
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">
          Each step runs a primitive or another saved tool. Map its inputs from
          the tool&apos;s parameters (
          <code className="font-[family-name:var(--font-mono)]">{"${params.x}"}</code>)
          or earlier steps (
          <code className="font-[family-name:var(--font-mono)]">{"${steps.s1.output}"}</code>).
          Drag to reorder.
        </p>

        {def.steps.length === 0 ? (
          <EmptyState
            icon={<Workflow className="size-9" />}
            title="No steps yet"
            description="Add the first step to start composing."
          />
        ) : (
          <div className="space-y-0">
            {def.steps.map((step, i) => (
              <div key={step.id}>
                {i > 0 && (
                  <div className="flex justify-center py-1" aria-hidden>
                    <div className="h-4 w-px bg-border" />
                  </div>
                )}
                <StepCard
                  step={step}
                  index={i}
                  total={def.steps.length}
                  earlierStepIds={def.steps.slice(0, i).map((s) => s.id)}
                  paramNames={paramNames}
                  primitives={primitives}
                  toolOptions={tools}
                  onChange={(s) => updateStep(i, s)}
                  onRemove={() => removeStep(i)}
                  onMoveUp={() => move(i, i - 1)}
                  onMoveDown={() => move(i, i + 1)}
                  onDragStart={() => {
                    dragIndex.current = i;
                  }}
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={() => {
                    if (dragIndex.current !== null) move(dragIndex.current, i);
                    dragIndex.current = null;
                  }}
                />
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
