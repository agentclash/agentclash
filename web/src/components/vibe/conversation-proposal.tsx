"use client";

import { useState } from "react";
import { ArrowRight, ChevronRight, FileText } from "lucide-react";
import {
  editableEvaluation,
  type Artifact,
  type EvidenceSet,
  type Expectation,
} from "@/lib/vibe";
import { VibeButton } from "./vibe-button";

export function ConversationProposal({
  artifact,
  evidence,
  busy,
  blocked,
  comparison,
  onRun,
  onEdit,
  onDirty,
  onInstructions,
  onPreview,
}: {
  artifact: Artifact;
  evidence?: EvidenceSet;
  busy: boolean;
  blocked?: boolean;
  comparison?: boolean;
  onRun: () => void;
  onEdit: (fields: Record<string, unknown>) => Promise<boolean>;
  onDirty: (dirty: boolean) => void;
  onInstructions: () => void;
  onPreview: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [rules, setRules] = useState<Expectation[]>(
    artifact.conversation_evaluation?.expectations || [],
  );
  const evaluated = editableEvaluation(artifact.blueprint);
  const [promptChecks, setPromptChecks] = useState(evaluated);
  const rows: { key: string; title: string; input?: string }[] =
    artifact.conversation_evaluation?.expectations.map((q) => ({
      key: q.id,
      title: q.statement,
    })) ||
    evaluated?.scenarios?.map((q, i) => ({
      key: String(i),
      title: q.expected,
      input: q.input,
    })) ||
    evaluated?.examples.map((input, i) => ({
      key: String(i),
      title: evaluated.success_criteria,
      input,
    })) ||
    [];
  const isProvided = artifact.kind === "conversation_evaluation";
  return (
    <section aria-label="Proposed check" className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-xl font-semibold tracking-tight">
          Here’s what I’ll check
        </h2>
        <span className="flex items-center gap-2 text-xs vibe-muted">
          <FileText size={14} />
          {isProvided
            ? `${evidence?.conversations.length || "Saved"} whole chats · provided replies`
            : "Text test · instructions + model"}
        </span>
      </div>
      {rows.length > 0 && (
        <div className="vibe-panel">
          {rows.map((row) => (
            <details key={row.key} className="vibe-result-row">
              <summary>
                <span className="min-w-0 flex-1 text-sm leading-6">
                  {row.title}
                </span>
                <ChevronRight size={16} className="vibe-chevron vibe-muted" />
              </summary>
              <div className="border-t border-[var(--vibe-border)] p-5 text-sm">
                {"input" in row ? (
                  <>
                    <p className="mb-2 text-xs vibe-muted">Example message</p>
                    <p className="vibe-transcript">{row.input}</p>
                  </>
                ) : (
                  <p className="vibe-muted">
                    Checked across every message, including follow-ups. If the
                    chat does not provide enough evidence, this expectation
                    stays unresolved.
                  </p>
                )}
              </div>
            </details>
          ))}
        </div>
      )}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <VibeButton
          variant="quiet"
          disabled={busy}
          onClick={() => setEditing(!editing)}
        >
          Edit expectations
        </VibeButton>
        <VibeButton
          variant="primary"
          disabled={busy || blocked || editing}
          onClick={onRun}
        >
          {comparison
            ? "Compare using the same examples"
            : isProvided
              ? "Check these chats"
              : `Run ${rows.length || "the"} examples`}
          <ArrowRight />
        </VibeButton>
      </div>
      {editing &&
        (isProvided ? (
          <div className="space-y-4">
            {rules.map((q, i) => (
              <label className="block text-xs vibe-muted" key={q.id}>
                Expectation {i + 1}
                <textarea
                  className="vibe-textarea mt-2"
                  aria-label={`Expectation ${i + 1}`}
                  value={q.statement}
                  onChange={(e) => {
                    setRules((old) =>
                      old.map((r, n) =>
                        n === i ? { ...r, statement: e.target.value } : r,
                      ),
                    );
                    onDirty(true);
                  }}
                />
              </label>
            ))}
            <div className="flex gap-2">
              <VibeButton
                disabled={busy || rules.some((q) => !q.statement.trim())}
                onClick={async () => {
                  if (
                    await onEdit({
                      artifact_id: artifact.id,
                      expectations: rules,
                    })
                  ) {
                    setEditing(false);
                    onDirty(false);
                  }
                }}
              >
                Save expectations
              </VibeButton>
              <VibeButton
                variant="quiet"
                onClick={() => {
                  setRules(artifact.conversation_evaluation!.expectations);
                  setEditing(false);
                  onDirty(false);
                }}
              >
                Discard edits
              </VibeButton>
            </div>
            <p className="text-xs vibe-muted">
              Changes create a new check. Earlier results keep their original
              rules.
            </p>
          </div>
        ) : promptChecks ? (
          <div className="space-y-4">
            {(
              promptChecks.scenarios ||
              promptChecks.examples.map((input) => ({
                input,
                expected: promptChecks.success_criteria,
              }))
            ).map((q, i) => (
              <fieldset className="space-y-3" key={i}>
                <legend className="mb-2 text-sm">Example {i + 1}</legend>
                {(["input", "expected"] as const).map((field) => (
                  <label className="block text-xs vibe-muted" key={field}>
                    {field === "input" ? "Message" : "Expected behavior"}
                    <textarea
                      aria-label={`Example ${i + 1} ${field}`}
                      className="vibe-textarea mt-1"
                      value={q[field]}
                      onChange={(e) => {
                        const scenarios =
                          promptChecks.scenarios ||
                          promptChecks.examples.map((input) => ({
                            input,
                            expected: promptChecks.success_criteria,
                          }));
                        setPromptChecks({
                          ...promptChecks,
                          scenarios: scenarios.map((r, n) =>
                            n === i ? { ...r, [field]: e.target.value } : r,
                          ),
                        });
                        onDirty(true);
                      }}
                    />
                  </label>
                ))}
              </fieldset>
            ))}
            <label className="block text-xs vibe-muted">
              Shared expected behavior
              <textarea
                aria-label="Shared expected behavior"
                className="vibe-textarea mt-2"
                value={promptChecks.success_criteria}
                onChange={(e) => {
                  setPromptChecks({
                    ...promptChecks,
                    success_criteria: e.target.value,
                  });
                  onDirty(true);
                }}
              />
            </label>
            <div className="flex gap-2">
              <VibeButton
                disabled={busy}
                onClick={async () => {
                  if (
                    await onEdit({
                      artifact_id: artifact.id,
                      evaluation: promptChecks,
                    })
                  ) {
                    setEditing(false);
                    onDirty(false);
                  }
                }}
              >
                Save expectations
              </VibeButton>
              <VibeButton
                variant="quiet"
                onClick={() => {
                  setPromptChecks(evaluated);
                  setEditing(false);
                  onDirty(false);
                }}
              >
                Discard edits
              </VibeButton>
            </div>
          </div>
        ) : (
          <p className="text-sm vibe-muted">
            This imported evaluation uses custom checks. Export it from Settings
            to edit its full definition.
          </p>
        ))}
      {!isProvided && (
        <div className="flex flex-wrap gap-2 border-t border-[var(--vibe-border)] pt-3">
          <VibeButton variant="quiet" onClick={onInstructions}>
            Agent instructions
          </VibeButton>
          <VibeButton
            variant="quiet"
            disabled={busy || blocked}
            onClick={onPreview}
          >
            Try a message
          </VibeButton>
          <p className="w-full text-xs vibe-muted">
            Text responses here. Your live tools and data aren’t connected.
          </p>
        </div>
      )}
    </section>
  );
}
