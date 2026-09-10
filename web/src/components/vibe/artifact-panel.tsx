"use client";
import { useState } from "react";
import { Check, Download, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { editableEvaluation, type EvaluationProposal } from "@/lib/vibe";
import type {
  Artifact,
  Capability,
  Model,
  Models,
  Requirement,
} from "@/lib/vibe";
import { Requirements } from "./requirements";

export function ModelSelect({
  label,
  value,
  models,
  onChange,
  disabled,
}: {
  label: string;
  value: string;
  models: Model[];
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  return (
    <label className="inline-flex items-center gap-2 text-xs text-builder-fg-muted">
      <span>{label}</span>
      <select
        aria-label={`${label} model`}
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
        className="max-w-48 rounded-md border border-builder-border bg-builder-panel px-2 py-1.5 text-builder-fg outline-none focus-visible:ring-2 focus-visible:ring-builder-border-strong"
      >
        {!models.some((m) => m.id === value) && (
          <option value={value}>{value.split("/").pop()}</option>
        )}
        {models.map((m) => (
          <option key={m.id} value={m.id}>
            {m.name || m.id.split("/").pop()}
          </option>
        ))}
      </select>
    </label>
  );
}
export function ArtifactPanel({
  artifact,
  requirements,
  capabilities = [],
  models,
  choices,
  anonymous,
  busy,
  onClose,
  onAccept,
  onEdit,
  onDirtyChange,
  onEvaluationEdit,
  onRequirement,
  onModels,
  onCheck,
  onPlay,
  onSave,
}: {
  artifact: Artifact;
  requirements: Requirement[];
  capabilities?: Capability[];
  models: Models;
  choices: Model[];
  anonymous: boolean;
  busy: boolean;
  onClose: () => void;
  onAccept: () => void;
  onEdit: (prompt: string) => void;
  onDirtyChange?: (dirty: boolean) => void;
  onEvaluationEdit?: (evaluation: EvaluationProposal) => void;
  onRequirement: (
    id: string,
    status: "accepted" | "rejected" | "superseded",
    statement?: string,
  ) => void;
  onModels: (models: Models) => void;
  onCheck: () => void;
  onPlay: (content: string) => void;
  onSave: () => void;
}) {
  const [prompt, setPrompt] = useState(artifact.agent_prompt);
  const [test, setTest] = useState("");
  const initialEvaluation = editableEvaluation(artifact.blueprint);
  const [evaluation, setEvaluation] = useState(initialEvaluation);
  const previewRules = capabilities.find(
    (c) => c.id === "text_preview",
  )?.instructions;
  // Keep the full prompt in state, including while config is still loading.
  // Only the exact server-owned prefix moves into the read-only disclosure.
  const promptPrefix =
    previewRules && prompt.startsWith(previewRules) ? previewRules : "";
  const editablePrompt = prompt.slice(promptPrefix.length);
  const promptDirty = prompt !== artifact.agent_prompt;
  const evaluationDirty =
    JSON.stringify(evaluation) !== JSON.stringify(initialEvaluation);
  const dirty = promptDirty || evaluationDirty;
  function changeEvaluation(next: EvaluationProposal) {
    setEvaluation(next);
    onDirtyChange?.(
      promptDirty || JSON.stringify(next) !== JSON.stringify(initialEvaluation),
    );
  }
  function download() {
    const url = URL.createObjectURL(
      new Blob(
        [
          JSON.stringify(
            {
              format: "agentclash-vibe-v1",
              agent_prompt: artifact.agent_prompt,
              evaluation: artifact.blueprint,
              models,
            },
            null,
            2,
          ),
        ],
        { type: "application/json" },
      ),
    );
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "agentclash-agent-draft.json";
    anchor.click();
    URL.revokeObjectURL(url);
  }
  return (
    <aside
      aria-label="Agent draft"
      className="fixed inset-0 z-30 flex flex-col bg-builder-panel p-5 sm:inset-y-0 sm:left-auto sm:w-[410px] sm:border-l sm:border-builder-border lg:relative lg:z-auto lg:w-[380px] lg:shrink-0"
    >
      <div className="mb-5 flex items-start justify-between">
        <div>
          <p className="font-mono text-[10px] uppercase tracking-widest text-builder-fg-muted">
            Your agent · {artifact.accepted ? "Accepted" : "Draft"}
          </p>
          <h2 className="mt-2 font-semibold tracking-tight">
            {artifact.title}
          </h2>
        </div>
        <Button
          variant="ghost"
          size="icon"
          onClick={onClose}
          aria-label="Close draft"
        >
          <X size={16} />
        </Button>
      </div>
      <div className="min-h-0 flex-1 space-y-5 overflow-y-auto">
        <div className="space-y-2 text-xs leading-5 text-builder-fg-muted">
          <p>Text preview · No connected tools</p>
          <p>Review the instructions and tests, then accept to try a message.</p>
          {previewRules && (
            <details>
              <summary className="w-fit cursor-pointer hover:text-builder-fg">
                Preview rules
              </summary>
              <p className="mt-2">Applied automatically when you try this preview.</p>
              <p className="mt-2 whitespace-pre-wrap">{previewRules.trim()}</p>
            </details>
          )}
        </div>
        <label className="block text-xs">
          Agent instructions
          <textarea
            aria-label="Agent instructions"
            value={editablePrompt}
            onChange={(e) => {
              const next = promptPrefix + e.target.value;
              setPrompt(next);
              onDirtyChange?.(
                next !== artifact.agent_prompt || evaluationDirty,
              );
            }}
            disabled={busy || evaluationDirty}
            className="mt-2 min-h-56 w-full resize-y rounded-xl border border-builder-border bg-builder-surface p-3 text-sm leading-6 outline-none focus-visible:ring-2 focus-visible:ring-builder-border-strong"
          />
        </label>
        {dirty && (
          <p className="text-xs text-builder-warn">
            Save your edits as a new draft, then accept it before trying,
            checking or keeping this agent.
          </p>
        )}
        {dirty ? (
          <Button
            size="sm"
            variant="outline"
            onClick={() =>
              evaluationDirty && evaluation
                ? onEvaluationEdit?.(evaluation)
                : onEdit(prompt)
            }
            disabled={busy}
          >
            Save as a new draft
          </Button>
        ) : !artifact.accepted ? (
          <Button size="sm" onClick={onAccept} disabled={busy}>
            <Check size={14} /> Accept this draft
          </Button>
        ) : (
          <p className="text-xs text-builder-fg-muted">
            Accepted. Edits create a new version.
          </p>
        )}
        <div>
          <ModelSelect
            label="Agent"
            value={models.target}
            models={choices}
            onChange={(target) => onModels({ ...models, target })}
            disabled={busy}
          />
        </div>
        <Requirements
          requirements={requirements}
          busy={busy}
          onRequirement={onRequirement}
        />
        {evaluation && (
          <section aria-label="Proposed evaluation" className="space-y-3">
            <h3 className="text-xs font-medium">
              Examples and proposed criteria
            </h3>
            <p className="text-xs leading-5 text-builder-fg-muted">
              Review these proposed test rules, including numerical thresholds
              and assumptions. Only requirements marked “Confirmed by you” have
              been confirmed. Insufficient evidence can mean no recommendation.
              Accepting the draft accepts these tests; changing them creates a
              new evaluation.
            </p>
            <div
              aria-label="Criteria sources"
              className="text-xs leading-5 text-builder-fg-muted"
            >
              <p>
                Capability and evidence rules apply to every preview. Additional
                criteria without a linked requirement are proposed assumptions.
              </p>
              {!evaluationDirty &&
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
                          : "Changed since this evaluation"}
                      : {r.statement}
                    </p>
                  ))}
            </div>
            {evaluation.examples.map((example, i) => (
              <label className="block text-xs" key={i}>
                Example {i + 1}
                <textarea
                  aria-label={`Evaluation example ${i + 1}`}
                  value={example}
                  disabled={busy || promptDirty || !onEvaluationEdit}
                  onChange={(e) =>
                    changeEvaluation({
                      ...evaluation,
                      examples: evaluation.examples.map((v, n) =>
                        n === i ? e.target.value : v,
                      ),
                    })
                  }
                  className="mt-2 min-h-20 w-full rounded-lg border border-builder-border bg-builder-surface p-3 text-sm"
                />
              </label>
            ))}
            <label className="block text-xs">
              Expected behavior and criteria
              <textarea
                aria-label="Evaluation criteria"
                value={evaluation.success_criteria}
                disabled={busy || promptDirty || !onEvaluationEdit}
                onChange={(e) =>
                  changeEvaluation({
                    ...evaluation,
                    success_criteria: e.target.value,
                  })
                }
                className="mt-2 min-h-28 w-full rounded-lg border border-builder-border bg-builder-surface p-3 text-sm"
              />
            </label>
            {evaluationDirty && (
              <p className="text-xs text-builder-warn">
                Save these tests as a new draft above, then review and accept
                it. A retest still uses its original evaluation.
              </p>
            )}
          </section>
        )}
        <details>
          <summary className="cursor-pointer text-xs text-builder-fg-muted">
            Evaluation details
          </summary>
          <p className="my-3 text-xs leading-5 text-builder-fg-muted">
            These examples and criteria stay fixed for a fair retest. Editing
            the agent does not edit its tests.
          </p>
          <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-all rounded-lg bg-builder-surface p-3 font-mono text-[11px]">
            {JSON.stringify(artifact.blueprint, null, 2)}
          </pre>
          <div className="mt-3">
            <ModelSelect
              label="Evaluator"
              value={models.evaluator}
              models={choices}
              onChange={(evaluator) => onModels({ ...models, evaluator })}
              disabled={busy || anonymous}
            />
          </div>
          {anonymous && (
            <p className="mt-2 text-[11px] text-builder-fg-muted">
              The free trial keeps the evaluator fixed.
            </p>
          )}
          <Button className="mt-3" variant="ghost" size="sm" onClick={download}>
            <Download size={13} /> Export agent and evaluation
          </Button>
        </details>
        {capabilities.length > 0 && (
          <details className="text-xs leading-5 text-builder-fg-muted">
            <summary className="cursor-pointer">
              Preview capabilities and documentation
            </summary>
            {capabilities.map((c) => (
              <p className="mt-2" key={c.id}>
                <strong>{c.label}:</strong> {c.description}{" "}
                {c.url && (
                  <a
                    className="underline"
                    href={c.url}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Documentation ↗
                  </a>
                )}
              </p>
            ))}
          </details>
        )}
        {
          <details open>
            <summary className="cursor-pointer text-xs text-builder-fg-muted">
              Try a customer message
            </summary>
            <p className="mt-2 text-xs leading-5 text-builder-fg-muted">
              {artifact.accepted
                ? "Send one message to the accepted text preview. This is separate from running an evaluation."
                : "Review the instructions and criteria, then accept the draft to try a customer message. You can revise it in Design first."}
            </p>
            <textarea
              aria-label="Agent playground message"
              value={test}
              onChange={(e) => setTest(e.target.value)}
              placeholder="Send this agent a test message…"
              className="mt-3 min-h-24 w-full rounded-lg border border-builder-border bg-builder-surface p-3 text-sm"
            />
            <Button
              size="sm"
              disabled={busy || dirty || !artifact.accepted || !test.trim()}
              onClick={() => onPlay(test)}
            >
              Send to agent
            </Button>
          </details>
        }
      </div>
      <div className="mt-5 flex gap-2 border-t border-builder-border pt-4">
        <Button
          className="flex-1"
          disabled={busy || dirty || !artifact.accepted}
          onClick={onCheck}
        >
          Run evaluation
        </Button>
        <Button
          variant="outline"
          disabled={busy || dirty || !artifact.accepted}
          onClick={onSave}
        >
          Keep it
        </Button>
      </div>
    </aside>
  );
}
