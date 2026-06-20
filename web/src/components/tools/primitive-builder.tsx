"use client";

import { useMemo } from "react";
import { cn } from "@/lib/utils";
import { ArgsEditor } from "./args-editor";
import { controlClass } from "./field";
import { JsonValueField } from "./json-value-field";
import { ParametersEditor } from "./parameters-editor";
import { declaredParamNames, paramsToSchema, schemaToParams } from "./lib/definition";
import type {
  MockStrategy,
  PrimitiveDefinition,
  PrimitiveMode,
  ToolPrimitive,
} from "./lib/types";

const MODES: { value: PrimitiveMode; label: string; hint: string }[] = [
  { value: "delegate", label: "Call a primitive", hint: "Wrap one base operation (HTTP, shell, file, data)." },
  { value: "mock", label: "Mock response", hint: "Return a canned response for rehearsing evals." },
];

const STRATEGIES: MockStrategy[] = ["static", "lookup", "echo"];

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
  const byKind = useMemo(() => {
    const groups: Record<string, ToolPrimitive[]> = {};
    for (const p of delegatable) (groups[p.kind] ??= []).push(p);
    return groups;
  }, [delegatable]);

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
        <h3 className="text-sm font-medium">Implementation</h3>
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
        <div className="space-y-3">
          <div>
            <label className="mb-1.5 block text-sm font-medium">Primitive</label>
            <select
              value={impl.primitive ?? ""}
              onChange={(e) => setImpl({ primitive: e.target.value })}
              className={controlClass}
            >
              <option value="">Select a primitive…</option>
              {Object.entries(byKind).map(([kind, items]) => (
                <optgroup key={kind} label={kind}>
                  {items.map((p) => (
                    <option key={p.name} value={p.name}>
                      {p.name}
                    </option>
                  ))}
                </optgroup>
              ))}
            </select>
            {selectedPrimitive && (
              <p className="mt-1 text-xs text-muted-foreground">{selectedPrimitive.description}</p>
            )}
          </div>
          <div>
            <h4 className="mb-1.5 text-sm font-medium">Arguments</h4>
            <ArgsEditor
              primitive={selectedPrimitive}
              args={impl.args ?? {}}
              onChange={(args) => setImpl({ args })}
              paramNames={declaredParamNames(def)}
              allowSecrets={impl.primitive === "http_request"}
            />
          </div>
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
  return (
    <div className="space-y-3">
      <div>
        <label className="mb-1.5 block text-sm font-medium">Strategy</label>
        <select
          value={mock.strategy}
          onChange={(e) => onChange({ ...mock, strategy: e.target.value as MockStrategy })}
          className={controlClass}
        >
          {STRATEGIES.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      </div>

      {mock.strategy === "static" && (
        <JsonValueField
          label="Response"
          hint="Returned verbatim on every call."
          value={mock.response}
          onChange={(response) => onChange({ ...mock, response })}
        />
      )}
      {mock.strategy === "lookup" && (
        <>
          <div>
            <label className="mb-1.5 block text-sm font-medium">Lookup key</label>
            <input
              value={mock.lookup_key ?? ""}
              onChange={(e) => onChange({ ...mock, lookup_key: e.target.value })}
              placeholder="e.g. order_id"
              className={`${controlClass} font-[family-name:var(--font-mono)] text-xs`}
            />
          </div>
          <JsonValueField
            label="Responses"
            hint='Map of key value → response, with "*" as the fallback.'
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
          hint="Echoes the call input, merged over this template."
          value={mock.template}
          onChange={(template) => onChange({ ...mock, template })}
        />
      )}
    </div>
  );
}
