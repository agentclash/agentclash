"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  editableEvaluation,
  type Artifact,
  type Capability,
  type EvaluationProposal,
  type Models,
  type Requirement,
  exportAgent,
} from "@/lib/vibe";

export function ChecksEditor({
  artifact,
  capabilities,
  requirements,
  models,
  busy,
  blocked,
  onEdit,
  onDirtyChange,
  onRun,
}: {
  artifact: Artifact;
  capabilities: Capability[];
  models: Models;
  requirements: Requirement[];
  busy: boolean;
  blocked: boolean;
  onEdit: (evaluation: EvaluationProposal) => Promise<boolean>;
  onDirtyChange: (dirty: boolean) => void;
  onRun: () => void;
}) {
  const initial = editableEvaluation(artifact.blueprint);
  const [evaluation, setEvaluation] = useState(initial);
  const dirty = JSON.stringify(initial) !== JSON.stringify(evaluation);
  const policy = capabilities.find(
    (c) => c.id === "text_preview",
  )?.criteria_instructions;
  const prefix =
    policy && evaluation?.success_criteria.startsWith(policy) ? policy : "";
  const scenarios =
    evaluation?.scenarios ??
    evaluation?.examples.map((input) => ({ input, expected: "" })) ??
    [];
  const count = evaluation ? scenarios.length : undefined;
  function change(next: EvaluationProposal) {
    setEvaluation(next);
    onDirtyChange(JSON.stringify(next) !== JSON.stringify(initial));
  }
  return (
    <section aria-label="Review checks" className="space-y-5">
      <div>
        <h1 className="text-xl font-semibold">
          Does your agent respond the way you expect?
        </h1>
        <p className="mt-2 text-sm leading-6 text-builder-fg-muted">
          Each test sends an example message to your agent and compares its
          reply with what you expected. Review the examples below, then run the
          tests. These tests start fresh; your chat conversation is not
          included.
        </p>
      </div>
      {evaluation ? (
        <>
          {scenarios.map((scenario, i) => (
            <fieldset
              key={i}
              className="space-y-3 rounded-xl border border-builder-border p-4"
            >
              <legend className="px-2 text-sm font-medium">
                Situation {i + 1}
              </legend>
              <label className="block text-sm">
                Message or task
                <textarea
                  aria-label={`Situation ${i + 1} message or task`}
                  value={scenario.input}
                  disabled={busy || blocked}
                  onChange={(e) =>
                    change(
                      evaluation.scenarios
                        ? {
                            ...evaluation,
                            scenarios: scenarios.map((s, n) =>
                              n === i ? { ...s, input: e.target.value } : s,
                            ),
                          }
                        : {
                            ...evaluation,
                            examples: evaluation.examples.map((s, n) =>
                              n === i ? e.target.value : s,
                            ),
                          },
                    )
                  }
                  className="mt-2 min-h-20 w-full rounded-lg border border-builder-border bg-builder-surface p-3 text-sm leading-6"
                />
              </label>
              {evaluation.scenarios ? (
                <label className="block text-sm">
                  What should happen
                  <textarea
                    aria-label={`Situation ${i + 1} expected behavior`}
                    value={scenario.expected}
                    disabled={busy || blocked}
                    onChange={(e) =>
                      change({
                        ...evaluation,
                        scenarios: scenarios.map((s, n) =>
                          n === i ? { ...s, expected: e.target.value } : s,
                        ),
                      })
                    }
                    className="mt-2 min-h-20 w-full rounded-lg border border-builder-border bg-builder-surface p-3 text-sm leading-6"
                  />
                </label>
              ) : (
                <p className="text-sm text-builder-fg-muted">
                  Uses the shared expected behavior below.
                </p>
              )}
            </fieldset>
          ))}
          <details
            open={!evaluation.scenarios}
            className="rounded-xl border border-builder-border p-4 text-sm"
          >
            <summary className="cursor-pointer font-medium">
              {evaluation.scenarios
                ? "Rules for all situations"
                : "Shared expected behavior · applies to every situation"}
            </summary>
            <label className="mt-3 block">
              Shared rules
              <textarea
                aria-label="Shared expected behavior"
                value={evaluation.success_criteria.slice(prefix.length)}
                disabled={busy || blocked}
                onChange={(e) =>
                  change({
                    ...evaluation,
                    success_criteria: prefix + e.target.value,
                  })
                }
                className="mt-2 min-h-28 w-full rounded-lg border border-builder-border bg-builder-surface p-3 text-sm leading-6"
              />
            </label>
            <p className="mt-2 text-builder-fg-muted">
              These are proposed test rules. Equivalent wording can pass;
              missing evidence must not be treated as a proven answer.
            </p>
            {policy && (
              <details className="mt-3">
                <summary className="cursor-pointer">
                  Preview evidence rules
                </summary>
                <p className="mt-2 whitespace-pre-wrap">{policy.trim()}</p>
              </details>
            )}
          </details>
          <details
            aria-label="Criteria sources"
            className="text-sm text-builder-fg-muted"
          >
            <summary className="cursor-pointer">
              Where these rules came from
            </summary>
            <p className="mt-3">
              Rules without a linked requirement are proposals. Running checks
              does not confirm business facts.
            </p>
            {!dirty &&
              requirements
                .filter((r) =>
                  artifact.criteria_requirement_ids?.includes(r.id),
                )
                .map((r) => (
                  <p key={r.id} className="mt-2">
                    Linked requirement ·{" "}
                    {r.status === "accepted"
                      ? "Confirmed by you"
                      : r.status === "proposed"
                        ? "Proposed"
                        : "Changed since these checks"}
                    : {r.statement}
                  </p>
                ))}
          </details>
        </>
      ) : (
        <div className="rounded-xl border border-builder-border p-5 text-sm leading-6">
          <p>
            This imported evaluation has a custom structure. Its original checks
            are preserved. Review the full definition below; edit it in the
            workspace builder.
          </p>
          <details className="mt-3">
            <summary className="cursor-pointer">Full check definition</summary>
            <pre className="mt-3 max-h-96 overflow-auto whitespace-pre-wrap break-all text-xs">
              {JSON.stringify(artifact.blueprint, null, 2)}
            </pre>
          </details>
          <Button
            variant="outline"
            size="sm"
            className="mt-3"
            onClick={() => exportAgent(artifact, models)}
          >
            Export original evaluation
          </Button>
        </div>
      )}
      {dirty ? (
        <div className="space-y-3">
          <p className="text-sm text-builder-fg-muted">
            Applying these changes creates a new test version. Earlier results
            keep their original expectations.
          </p>
          <div className="flex gap-3">
            <Button
              disabled={
                busy ||
                blocked ||
                !evaluation ||
                !evaluation.success_criteria.slice(prefix.length).trim() ||
                scenarios.some(
                  (s) =>
                    !s.input.trim() ||
                    (!!evaluation.scenarios && !s.expected.trim()),
                )
              }
              onClick={async () => {
                if (evaluation && (await onEdit(evaluation)))
                  onDirtyChange(false);
              }}
            >
              Apply changes
            </Button>
            <Button
              variant="ghost"
              disabled={busy}
              onClick={() => {
                setEvaluation(initial);
                onDirtyChange(false);
              }}
            >
              Discard edits
            </Button>
          </div>
        </div>
      ) : (
        <div className="space-y-2">
          <Button onClick={onRun} disabled={busy || blocked}>
            Run these{count ? ` ${count}` : ""} tests
          </Button>
          <p className="text-xs leading-5 text-builder-fg-muted">
            Uses this agent and the expected replies shown above. Results show
            what passed, what failed, and what could not be determined.
          </p>
        </div>
      )}
    </section>
  );
}
