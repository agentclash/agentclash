"use client";

import { Download, X } from "lucide-react";
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
}: {
  artifact: Artifact;
  journey?: Journey;
  capabilities: Capability[];
  requirements: Requirement[];
  busy: boolean;
  onClose: () => void;
  onPreview: () => void;
  onRequirement: (
    id: string,
    status: "accepted" | "rejected" | "superseded",
    statement?: string,
  ) => void;
}) {
  const storedPlan = artifact.test_plan;
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
    <aside
      aria-label="Test plan"
      className="fixed inset-0 z-30 flex flex-col bg-builder-panel p-5 sm:inset-y-0 sm:left-auto sm:w-[410px] sm:border-l sm:border-builder-border lg:relative lg:z-auto lg:w-[380px] lg:shrink-0"
    >
      <div className="mb-5 flex justify-between gap-3">
        <div>
          <p className="font-mono text-[10px] uppercase tracking-widest text-builder-fg-muted">
            Test plan · Not run
          </p>
          <h2 className="mt-2 font-semibold tracking-tight">{plan.title}</h2>
        </div>
        <Button
          variant="ghost"
          size="icon"
          onClick={onClose}
          aria-label="Close test plan"
        >
          <X size={16} />
        </Button>
      </div>
      <div className="min-h-0 flex-1 space-y-5 overflow-y-auto text-xs leading-5">
        <p>
          Your live agent is not connected. Review these scenarios and ask for
          changes in Design. Export the plan to test in your own environment.
        </p>
        <SafeMarkdown>{plan.objective}</SafeMarkdown>
        {journey?.stack && (
          <p>
            <strong>Your reported stack:</strong> {journey.stack}
          </p>
        )}
        {journey?.evidence && (
          <p>
            <strong>Available evidence:</strong> {journey.evidence}
          </p>
        )}
        <section aria-label="Planned scenarios">
          <h3 className="font-medium">Scenarios to test</h3>
          {plan.scenarios.map((s, i) => (
            <div
              key={i}
              className="mt-3 rounded-lg border border-builder-border p-3"
            >
              <SafeMarkdown>{s.input}</SafeMarkdown>
              <p className="mt-2 font-medium">Expected behavior</p>
              <SafeMarkdown>{s.expected}</SafeMarkdown>
            </div>
          ))}
        </section>
        <section>
          <h3 className="font-medium">Evidence needed</h3>
          <ul className="mt-2 list-disc space-y-2 pl-4">
            {plan.evidence_needed.map((s, i) => (
              <li key={i}>{s}</li>
            ))}
          </ul>
        </section>
        <section>
          <h3 className="font-medium">Next steps</h3>
          <ol className="mt-2 list-decimal space-y-2 pl-4">
            {plan.next_steps.map((s, i) => (
              <li key={i}>
                <SafeMarkdown>{s}</SafeMarkdown>
              </li>
            ))}
          </ol>
        </section>
        {plan.local_test_code && (
          <section>
            <h3 className="font-medium">Local example · Not executed</h3>
            <p>
              Configure your invocation before running this example. Contains
              checks text presence only; verify JSON and tool traces with your
              own assertions.
            </p>
            <pre className="mt-2 overflow-auto whitespace-pre-wrap rounded-lg bg-builder-surface p-3 font-mono text-[11px]">
              {plan.local_test_code}
            </pre>
          </section>
        )}
        <section>
          <h3 className="font-medium">What this can establish</h3>
          {capabilities.map((c) => (
            <p key={c.id} className="mt-3">
              <strong>{c.label}:</strong> {c.description}{" "}
              {c.url && (
                <a
                  href={c.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="underline"
                >
                  Documentation ↗
                </a>
              )}
            </p>
          ))}
        </section>
        <Requirements
          requirements={requirements}
          busy={busy}
          onRequirement={onRequirement}
        />
      </div>
      <div className="mt-5 space-y-2 border-t border-builder-border pt-4">
        <Button onClick={download} variant="outline" className="w-full">
          <Download size={14} /> Export test plan
        </Button>
        <Button
          onClick={onPreview}
          disabled={busy}
          variant="ghost"
          className="w-full"
        >
          Try a prompt-only preview
        </Button>
        <p className="text-[11px] leading-5 text-builder-fg-muted">
          A prompt preview is a separate surrogate. It does not test your
          connected system.
        </p>
      </div>
    </aside>
  );
}
