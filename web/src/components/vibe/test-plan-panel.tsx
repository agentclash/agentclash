"use client";

import { useState } from "react";
import { Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { Artifact, Capability, Journey, Requirement } from "@/lib/vibe";
import { SafeMarkdown } from "./safe-markdown";
import { Requirements } from "./requirements";

export function TestPlanPanel({
  artifact,
  journey,
  capabilities,
  requirements,
  busy,
  onClose,
  onPreview,
  onRequirement,
  onEdit,
  onDirtyChange,
}: {
  artifact: Artifact;
  journey?: Journey;
  capabilities: Capability[];
  requirements: Requirement[];
  busy: boolean;
  onClose: () => void;
  onEdit?: (
    scenarios: { input: string; expected: string }[],
  ) => Promise<boolean>;
  onDirtyChange?: (dirty: boolean) => void;
  onPreview: () => void;
  onRequirement: (
    id: string,
    status: "accepted" | "rejected" | "superseded",
    statement?: string,
  ) => void;
}) {
  const storedPlan = artifact.test_plan;
  const [scenarios, setScenarios] = useState(storedPlan?.scenarios || []);
  const dirty =
    JSON.stringify(scenarios) !== JSON.stringify(storedPlan?.scenarios || []);
  function change(next: typeof scenarios) {
    setScenarios(next);
    onDirtyChange?.(
      JSON.stringify(next) !== JSON.stringify(storedPlan?.scenarios || []),
    );
  }
  if (!storedPlan) return null;
  const handoff = capabilities.find((c) => c.id === "python_sdk");
  const plan = {
    ...storedPlan,
    local_test_code: handoff?.example || storedPlan.local_test_code,
    next_steps: handoff?.next_steps || storedPlan.next_steps,
  };
  function download() {
    const text = [
      `# ${plan!.title}`,
      "Test plan only. No tests have run.",
      plan!.objective,
      journey?.stack
        ? `Stack (provided by you): ${journey.stack}`
        : "Stack: not provided",
      "## Scenarios",
      ...plan!.scenarios.map(
        (s, i) => `${i + 1}. Input: ${s.input}\n\nExpected: ${s.expected}`,
      ),
      "## Evidence needed",
      ...plan!.evidence_needed,
      "## Next steps",
      ...plan!.next_steps,
      plan!.local_test_code
        ? `\n## Local example (not executed)\n\n\`\`\`python\n${plan!.local_test_code}\n\`\`\``
        : "",
      "## Capability and documentation references",
      ...capabilities.map(
        (c) => `${c.label}: ${c.description}${c.url ? `\n${c.url}` : ""}`,
      ),
    ].join("\n\n");
    const url = URL.createObjectURL(
      new Blob([text], { type: "text/markdown" }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = "agentclash-test-plan.md";
    a.click();
    URL.revokeObjectURL(url);
  }
  return (
    <section aria-label="Test plan" className="space-y-5">
      <div>
        <p className="text-xs text-builder-fg-muted">Test plan · Not run</p>
        <h1 className="mt-2 text-xl font-semibold">{plan.title}</h1>
      </div>
      <p className="text-sm leading-6">
        Your existing agent is not connected. Review these situations, then
        export the plan to test in your own environment.
      </p>
      <SafeMarkdown>{plan.objective}</SafeMarkdown>
      <section aria-label="Planned scenarios" className="space-y-4">
        {scenarios.map((scenario, i) => (
          <fieldset
            key={i}
            className="space-y-3 rounded-xl border border-builder-border p-4"
          >
            <legend className="px-2 text-sm font-medium">
              Situation {i + 1} · Not run
            </legend>
            <label className="block text-sm">
              Message or task
              <textarea
                aria-label={`Planned situation ${i + 1} input`}
                value={scenario.input}
                readOnly={!onEdit}
                disabled={busy}
                onChange={(e) =>
                  change(
                    scenarios.map((s, n) =>
                      n === i ? { ...s, input: e.target.value } : s,
                    ),
                  )
                }
                className="mt-2 min-h-20 w-full rounded-lg border border-builder-border bg-builder-surface p-3 leading-6"
              />
            </label>
            <label className="block text-sm">
              What should happen
              <textarea
                aria-label={`Planned situation ${i + 1} expected behavior`}
                value={scenario.expected}
                readOnly={!onEdit}
                disabled={busy}
                onChange={(e) =>
                  change(
                    scenarios.map((s, n) =>
                      n === i ? { ...s, expected: e.target.value } : s,
                    ),
                  )
                }
                className="mt-2 min-h-20 w-full rounded-lg border border-builder-border bg-builder-surface p-3 leading-6"
              />
            </label>
          </fieldset>
        ))}
      </section>
      {dirty && (
        <div className="flex gap-3">
          <Button
            disabled={
              busy ||
              scenarios.some(
                (s) =>
                  !s.input.trim() || s.expected.trim().split(/\s+/).length < 4,
              )
            }
            onClick={async () => {
              if (await onEdit?.(scenarios)) onDirtyChange?.(false);
            }}
          >
            Apply changes
          </Button>
          <Button
            variant="ghost"
            disabled={busy}
            onClick={() => change(storedPlan.scenarios)}
          >
            Discard edits
          </Button>
        </div>
      )}
      <div className="flex flex-wrap gap-3">
        <Button disabled={dirty} onClick={download}>
          <Download size={14} /> Export test plan
        </Button>
        <Button variant="outline" onClick={onClose}>
          Ask for changes
        </Button>
      </div>
      <details className="rounded-xl border border-builder-border p-4 text-sm">
        <summary className="cursor-pointer font-medium">
          What you need to run these checks
        </summary>
        {journey?.stack && (
          <p className="mt-3">
            <strong>Your setup:</strong> {journey.stack}
          </p>
        )}
        {journey?.evidence && (
          <p className="mt-3">
            <strong>Available evidence:</strong> {journey.evidence}
          </p>
        )}
        <h3 className="mt-4 font-medium">Evidence needed</h3>
        <ul className="mt-2 list-disc space-y-2 pl-5">
          {plan.evidence_needed.map((s, i) => (
            <li key={i}>{s}</li>
          ))}
        </ul>
        <h3 className="mt-4 font-medium">Next steps</h3>
        <ol className="mt-2 list-decimal space-y-2 pl-5">
          {plan.next_steps.map((s, i) => (
            <li key={i}>
              <SafeMarkdown>{s}</SafeMarkdown>
            </li>
          ))}
        </ol>
        {plan.local_test_code && (
          <details className="mt-4">
            <summary className="cursor-pointer">
              Local example · Not executed
            </summary>
            <p className="mt-2 text-xs">
              Configure your invocation and credentials locally. Text presence
              checks alone do not verify tools, voice or actions.
            </p>
            <pre className="mt-3 overflow-auto whitespace-pre-wrap rounded-lg bg-builder-surface p-3 text-xs">
              {plan.local_test_code}
            </pre>
          </details>
        )}
        {capabilities
          .filter((c) => c.url)
          .map((c) => (
            <p key={c.id} className="mt-3">
              <a
                href={c.url}
                target="_blank"
                rel="noopener noreferrer"
                className="underline"
              >
                {c.label} documentation ↗
              </a>
            </p>
          ))}
      </details>
      <details className="text-sm">
        <summary className="cursor-pointer">
          Assumptions and confirmed requirements
        </summary>
        <Requirements
          requirements={requirements}
          busy={busy || dirty}
          onRequirement={onRequirement}
        />
      </details>
      <div className="border-t border-builder-border pt-4">
        <Button variant="ghost" disabled={busy || dirty} onClick={onPreview}>
          Try a text simulation
        </Button>
        <p className="mt-2 text-xs text-builder-fg-muted">
          A separate preview of replies. Your real agent remains unconnected.
        </p>
      </div>
    </section>
  );
}
