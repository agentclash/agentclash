"use client";

import { useState } from "react";
import { ArrowRight, ChevronRight, Copy, Pencil } from "lucide-react";
import { type Artifact } from "@/lib/vibe";
import { suiteCases, changedCases } from "./test-suite-cases";
import { TestMeaning } from "./conversation-guidance";
import { VibeButton } from "./vibe-button";

export const exampleAgentDescription = `My agent helps customers with returns. Unopened items can be returned within 30 days. Opened items and purchases older than 30 days aren't eligible. It should ask for missing purchase age or item condition, and never claim it has processed a refund.`;

const instructionsHelp =
  "Show me the exact instructions used by the AI agent in my app, and which model it uses. Include its existing rules. Do not rewrite them. I want to test those instructions in Vibe Evals.";

export function TestSuitePanel({
  primary = true,
  artifact,
  busy,
  blocked,
  comparison,
  onRun,
  onEdit,
  onDirty,
  onSave,
  onSettings,
  modelLabel,
  pendingPolicy = false,
  checkingChanges = false,
  rules = [],
}: {
  primary?: boolean;
  artifact: Artifact;
  busy: boolean;
  blocked: boolean;
  comparison: boolean;
  onRun: () => void;
  onEdit: (fields: Record<string, unknown>) => Promise<boolean>;
  onDirty: (dirty: boolean) => void;
  onSave: () => void;
  onSettings: () => void;
  modelLabel?: string;
  pendingPolicy?: boolean;
  checkingChanges?: boolean;
  rules?: { id: string; statement: string }[];
}) {
  const tests = suiteCases(artifact.blueprint);
  const [preparingOnly, setPreparingOnly] = useState(false);
  const [focusedCase, setFocusedCase] = useState<string>();
  const [editing, setEditing] = useState(false);
  const [agentOpen, setAgentOpen] = useState(false);
  const [instructions, setInstructions] = useState(artifact.agent_prompt);
  const [draft, setDraft] = useState(tests);
  const [copied, setCopied] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);
  const cases = tests;
  const changes = changedCases(tests, draft);
  const ready = !!artifact.agent_prompt.trim();
  const validationBlocked = !!artifact.validation && artifact.validation.status !== "supported";
  if (!cases.length)
    return <p>This pack needs the full pack builder to display its tests. Your original file is preserved.</p>;

  return (
    <section aria-label="Your tests" className="space-y-5">
      <div>
        <h2 className="text-2xl font-semibold tracking-tight">
          {validationBlocked
            ? `${cases.length} ${cases.length === 1 ? "test needs" : "tests need"} checking`
            : pendingPolicy
              ? `${cases.length} ${cases.length === 1 ? "test from" : "tests from"} your previous policy`
            : comparison
            ? "Ready to test the fix"
            : `${cases.length} ${cases.length === 1 ? "test is" : "tests are"} ready`}
        </h2>
        <p className="mt-2 text-sm vibe-muted">
          {validationBlocked
            ? "These test changes aren’t ready to run."
            : pendingPolicy
              ? "Your previous tests are unchanged. This update hasn’t been applied."
            : comparison
            ? "Same requests. Same rules. We'll see what changed."
            : artifact.summary}
        </p>
      </div>
      {!editing && !agentOpen && <TestMeaning />}
      {rules.length > 0 && !editing && !agentOpen && (
        <details className="text-sm vibe-muted">
          <summary className="w-fit cursor-pointer rounded-sm focus-visible:outline-2 focus-visible:outline-offset-4">
            Rules being tested
          </summary>
          <ul className="mt-3 list-disc space-y-2 pl-5 leading-6">
            {rules.map(rule => <li key={rule.id}>{rule.statement}</li>)}
          </ul>
        </details>
      )}
      {!agentOpen && !editing && (
        <div className="vibe-panel">
          {cases.map((test, i) => (
            <details className="vibe-result-row" key={test.key}>
              <summary>
                <span className="w-5 shrink-0 text-xs vibe-muted">{i + 1}</span>
                <span className="min-w-0 flex-1 text-sm leading-6">
                  {test.input}
                </span>
                <ChevronRight className="vibe-chevron vibe-muted" size={16} />
              </summary>
              <div className="border-t border-[var(--vibe-border)] px-5 py-4 text-sm">
                <p className="mb-1 text-xs vibe-muted">What should happen</p>
                <p>{test.expected}</p>
                {test.editable && <VibeButton variant="quiet" className="mt-2 !px-0" disabled={busy} onClick={() => { setFocusedCase(test.key); setEditing(true); }}>Edit this expectation</VibeButton>}
                {!test.editable && <p className="mt-2 text-xs vibe-muted">To change custom grading rules, edit the original pack and import it again.</p>}
              </div>
            </details>
          ))}
        </div>
      )}
      {editing && draft && (
        <div className="vibe-panel space-y-5 p-5">
          {checkingChanges && <p role="status" className="text-sm vibe-muted">Checking these test changes…</p>}
          {draft.map((test, i) => test.editable && (!focusedCase || focusedCase === test.key) && (
            <div key={test.key} className="space-y-3">
              {(focusedCase ? ["expected"] as const : ["input", "expected"] as const).map((field) => (
                <label key={field} className="block text-xs vibe-muted">
                  {field === "input"
                    ? `Test ${i + 1} · message`
                    : "What should happen"}
                  <textarea
                    className="vibe-textarea mt-1"
                    disabled={busy || checkingChanges}
                    value={test[field]}
                    onChange={(e) => {
                      const value = e.target.value;
                      setDraft(previous => previous.map(row => row.key === test.key ? { ...row, [field]: value } : row));
                      onDirty(true);
                    }}
                  />
                </label>
              ))}
            </div>
          ))}
          <div className="flex flex-wrap gap-2">
            <VibeButton
              variant={primary ? "primary" : "secondary"}
              disabled={
                busy ||
                !changes.length || draft.some(
                  (t) => !t.input.trim() || !t.expected.trim(),
                )
              }
              onClick={async () => {
                if (
                  await onEdit({
                    artifact_id: artifact.id,
                    case_changes: changes,
                  })
                ) {
                  setEditing(false);
                  onDirty(false);
                }
              }}
            >
              Save test changes
            </VibeButton>
            <VibeButton
              variant="quiet"
              disabled={busy}
              onClick={() => {
                setDraft(tests);
                setEditing(false);
                onDirty(false);
              }}
            >
              Cancel
            </VibeButton>
          </div>
        </div>
      )}
      {agentOpen && (
        <form
          className="vibe-panel space-y-4 p-5"
          onSubmit={async (e) => {
            e.preventDefault();
            if (busy || !instructions.trim()) return;
            if (
              await onEdit({
                artifact_id: artifact.id,
                agent_prompt: instructions,
              })
            ) {
              setAgentOpen(false);
              onDirty(false);
            }
          }}
        >
          <div>
            <h3 className="font-medium">Add your agent’s instructions</h3>
            <p className="mt-1 text-sm vibe-muted">
              We’ll run these with {modelLabel || "the selected model"}. This
              tests text replies; your app’s tools and data aren’t connected.
            </p>
          </div>
          <label className="block text-sm">
            Agent instructions
            <textarea
              aria-label="Agent instructions"
              className="vibe-textarea mt-2 min-h-40"
              value={instructions}
              onChange={(e) => {
                setInstructions(e.target.value);
                onDirty(true);
              }}
              placeholder="Paste the instructions your agent actually uses…"
              autoFocus
            />
          </label>
          <div className="flex flex-wrap gap-2">
            <VibeButton
              variant={primary ? "primary" : "secondary"}
              type="submit"
              disabled={busy || !instructions.trim()}
            >
              Use these instructions <ArrowRight />
            </VibeButton>
            <VibeButton
              variant="quiet"
              disabled={busy}
              onClick={() => {
                setAgentOpen(false);
                setInstructions(artifact.agent_prompt);
                onDirty(false);
              }}
            >
              Cancel
            </VibeButton>
          </div>
          <details className="text-sm vibe-muted">
            <summary className="w-fit cursor-pointer">
              Where do I find these?
            </summary>
            <p className="mt-3">
              Ask the coding tool you used to build your agent:
            </p>
            <p className="vibe-transcript mt-2 rounded-lg border border-[var(--vibe-border)] p-3">
              {instructionsHelp}
            </p>
            <VibeButton
              variant="quiet"
              className="mt-2"
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(instructionsHelp);
                  setCopied(true);
                  setCopyFailed(false);
                } catch {
                  setCopyFailed(true);
                }
              }}
            >
              <Copy />
              {copied ? "Copied" : "Copy request"}
            </VibeButton>
            {copyFailed && <p role="status">Select and copy the text above.</p>}
          </details>
        </form>
      )}
      {!agentOpen && !editing && (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <VibeButton
              variant={primary ? "primary" : "secondary"}
              disabled={busy || blocked || validationBlocked}
              onClick={() => (preparingOnly && !ready ? onSave() : ready ? onRun() : setAgentOpen(true))}
            >
              {ready
                ? pendingPolicy
                  ? "Run previous tests"
                  : comparison
                  ? "Test the suggested fix"
                  : `Run ${cases.length} ${cases.length === 1 ? "test" : "tests"}`
                : preparingOnly ? "Keep these tests" : "Add your agent"}
              <ArrowRight />
            </VibeButton>
            {tests.some(t => t.editable) && <VibeButton
              variant="quiet"
              disabled={busy}
              onClick={() => { setFocusedCase(undefined); setEditing(true); }}
            >
              <Pencil />
              Edit tests
            </VibeButton>}
          </div>
          {!ready && <div className="text-sm vibe-muted">
            {preparingOnly ? <><p>Keep these examples for your team. Add an agent when you’re ready to run them.</p><VibeButton variant="quiet" className="!px-0" disabled={busy} onClick={() => setAgentOpen(true)}>Add an agent later</VibeButton></>
              : <VibeButton variant="quiet" className="!px-0" disabled={busy} onClick={() => setPreparingOnly(true)}>I don’t have an agent yet</VibeButton>}
          </div>}
          {ready && (
            <p className="text-xs vibe-muted">
              Agent instructions · {modelLabel || "Selected model"}
            </p>
          )}
          <details className="text-sm vibe-muted">
            <summary className="w-fit cursor-pointer py-1">
              More options
            </summary>
            <div className="mt-2 flex flex-wrap gap-2">
              {ready && (
                <VibeButton
                  variant="quiet"
                  disabled={busy || blocked}
                  onClick={() => setAgentOpen(true)}
                >
                  Change agent instructions
                </VibeButton>
              )}
              {!(preparingOnly && !ready) && <VibeButton
                variant="quiet"
                onClick={onSave}
                disabled={busy || blocked || validationBlocked}
              >
                Keep these tests
              </VibeButton>}
              {ready && (
                <VibeButton
                  variant="quiet"
                  disabled={busy || blocked}
                  onClick={onSettings}
                >
                  Change model
                </VibeButton>
              )}
            </div>
          </details>
        </>
      )}
    </section>
  );
}
