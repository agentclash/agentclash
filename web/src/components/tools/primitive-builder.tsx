"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";
import { ArgsEditor } from "./args-editor";
import { controlClass } from "./field";
import { JsonValueField } from "./json-value-field";
import { OperationPicker } from "./operation-picker";
import { ParametersEditor } from "./parameters-editor";
import { MOCK_STRATEGY_OPTIONS } from "./lib/friendly";
import { declaredParamNames, paramsToSchema, schemaToParams } from "./lib/definition";
import type {
  MockStrategy,
  PrimitiveDefinition,
  PrimitiveMode,
  ToolPrimitive,
} from "./lib/types";

const MODES: { value: PrimitiveMode; label: string; hint: string }[] = [
  {
    value: "delegate",
    label: "Do it for real",
    hint: "Perform an actual operation — call an API, run a command, touch a file.",
  },
  {
    value: "mock",
    label: "Return a canned response",
    hint: "Skip the real work and return a fixed answer. Handy for rehearsing evals.",
  },
];

export function PrimitiveBuilder({
  def,
  onChange,
  primitives,
}: {
  def: PrimitiveDefinition;
  onChange: (def: PrimitiveDefinition) => void;
  primitives: ToolPrimitive[];
}) {
  const delegatable = useMemo(
    () => primitives.filter((p) => p.delegatable),
    [primitives],
  );

  const impl = def.implementation;
  const selectedPrimitive =
    delegatable.find((p) => p.name === impl.primitive) ?? null;
  const params = schemaToParams(def.parameters);

  function setImpl(patch: Partial<PrimitiveDefinition["implementation"]>) {
    onChange({ ...def, implementation: { ...def.implementation, ...patch } });
  }

  return (
    <div className="space-y-6">
      <ParametersEditor
        params={params}
        onChange={(next) => onChange({ ...def, parameters: paramsToSchema(next) })}
      />

      <div className="space-y-2">
        <h3 className="text-sm font-medium">When the agent calls this tool…</h3>
        <div className="grid grid-cols-2 gap-2">
          {MODES.map((m) => (
            <button
              key={m.value}
              type="button"
              onClick={() => setImpl({ mode: m.value })}
              className={cn(
                "rounded-lg border p-3 text-left transition-colors",
                impl.mode === m.value
                  ? "border-primary bg-primary/5"
                  : "border-border hover:border-foreground/30",
              )}
            >
              <div className="text-sm font-medium">{m.label}</div>
              <div className="mt-0.5 text-xs text-muted-foreground">{m.hint}</div>
            </button>
          ))}
        </div>
      </div>

      {impl.mode === "delegate" ? (
        <div className="space-y-4">
          <div className="space-y-1.5">
            <h4 className="text-sm font-medium">Operation</h4>
            <p className="text-xs text-muted-foreground">
              The action this tool performs under the hood.
            </p>
            <OperationPicker
              primitives={delegatable}
              selected={impl.primitive ?? ""}
              onSelect={(name) => setImpl({ primitive: name })}
            />
          </div>
          {selectedPrimitive && (
            <div className="space-y-1.5">
              <h4 className="text-sm font-medium">Details</h4>
              <p className="text-xs text-muted-foreground">
                Fill these in with fixed text, or insert an agent input.
              </p>
              <ArgsEditor
                primitive={selectedPrimitive}
                args={impl.args ?? {}}
                onChange={(args) => setImpl({ args })}
                paramNames={declaredParamNames(def)}
                allowSecrets={impl.primitive === "http_request"}
              />
            </div>
          )}
        </div>
      ) : (
        <MockEditor
          mock={impl.mock ?? { strategy: "static" }}
          onChange={(mock) => setImpl({ mock })}
        />
      )}
    </div>
  );
}

function MockEditor({
  mock,
  onChange,
}: {
  mock: NonNullable<PrimitiveDefinition["implementation"]["mock"]>;
  onChange: (mock: NonNullable<PrimitiveDefinition["implementation"]["mock"]>) => void;
}) {
  const activeHint = MOCK_STRATEGY_OPTIONS.find((s) => s.value === mock.strategy)?.hint;

  return (
    <div className="space-y-3">
      <div>
        <label className="mb-1.5 block text-sm font-medium">How should the response be chosen?</label>
        <select
          value={mock.strategy}
          onChange={(e) => onChange({ ...mock, strategy: e.target.value as MockStrategy })}
          className={controlClass}
        >
          {MOCK_STRATEGY_OPTIONS.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </select>
        {activeHint && <p className="mt-1 text-xs text-muted-foreground">{activeHint}</p>}
      </div>

      {mock.strategy === "static" && (
        <JsonValueField
          label="Response"
          hint="Returned on every call. Enter it as JSON, e.g. { &quot;status&quot;: &quot;ok&quot; }."
          value={mock.response}
          onChange={(response) => onChange({ ...mock, response })}
        />
      )}
      {mock.strategy === "lookup" && (
        <>
          <div>
            <label className="mb-1.5 block text-sm font-medium">Which input decides the response?</label>
            <input
              value={mock.lookup_key ?? ""}
              onChange={(e) => onChange({ ...mock, lookup_key: e.target.value })}
              placeholder="e.g. order_id"
              className={`${controlClass} font-[family-name:var(--font-mono)] text-xs`}
            />
          </div>
          <JsonValueField
            label="Responses by value"
            hint='A JSON object mapping each input value to its response. Use "*" for the fallback.'
            value={mock.responses}
            onChange={(responses) =>
              onChange({ ...mock, responses: responses as Record<string, unknown> })
            }
          />
        </>
      )}
      {mock.strategy === "echo" && (
        <JsonValueField
          label="Template (optional)"
          hint="Returns the agent's input, merged over this JSON template."
          value={mock.template}
          onChange={(template) => onChange({ ...mock, template })}
        />
      )}
    </div>
  );
}
