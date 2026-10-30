"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { ArrowRight, ChevronRight, Copy, Pencil } from "lucide-react";
import { useVibeEditorDraft } from "@/lib/use-vibe-editor-draft";
import { editorSource } from "@/lib/vibe-build-drafts";
import { downloadJSON } from "@/lib/vibe-export";
import { type Artifact } from "@/lib/vibe";
import { testCount } from "@/lib/vibe-presentation";
import { suiteCases, changedCases } from "./test-suite-cases";
import { TestMeaning } from "./conversation-guidance";
import { ValidationDetails } from "./validation-review";
import { VibeButton } from "./vibe-button";

export const exampleAgentDescription = `My agent helps customers with returns. Unopened items can be returned within 30 days. Opened items and purchases older than 30 days aren't eligible. It should ask for missing purchase age or item condition, and never claim it has processed a refund.`;

// The action that tests a changed version against its parent's result.
export const rerunLabel = (buildJourney: boolean) => buildJourney ? "Apply change and rerun" : "Improve and rerun";

const instructionsHelp =
  "Show me the exact instructions used by the AI agent in my app, and which model it uses. Include its existing rules. Do not rewrite them. I want to test those instructions in Vibe Evals.";

export function TestSuitePanel({
  mode = "combined", onAgent, onAskCase,
  inferenceUnavailable = false,
  selectedCase,
  onSelectCase,
  projectID = "unsaved",
  buildJourney = false,
  parent,
  primary = true,
  artifact,
  busy,
  blocked,
  comparison,
  onRun,
  onResult,
  onResultShown,
  onEdit,
  onDirty,
  onSave, onOriginal,
  onSettings,
  modelLabel,
  pendingPolicy = false,
  checkingChanges = false,
  rules = [],
}: {
  mode?: "combined" | "tests"; onAgent?: () => void; onAskCase?: (key: string) => void;
  inferenceUnavailable?: boolean;
  selectedCase?: string;
  onSelectCase?: (key?: string) => void;
  projectID?: string;
  buildJourney?: boolean;
  parent?: Artifact;
  primary?: boolean;
  artifact: Artifact;
  busy: boolean;
  blocked: boolean;
  comparison: boolean;
  onRun: () => void;
  // Set when these tests already ran: the primary opens that result and running again is secondary.
  onResult?: () => void;
  // Reports whether this panel shows its own "See the result", so the page does not repeat it.
  onResultShown?: (shown: boolean) => void;
  onEdit: (fields: Record<string, unknown>) => Promise<boolean>;
  onDirty: (dirty: boolean) => void;
  onSave: () => void;
  onOriginal?:()=>void;
  onSettings: () => void;
  modelLabel?: string;
  pendingPolicy?: boolean;
  checkingChanges?: boolean;
  rules?: { id: string; statement: string }[];
}) {
  const tests = suiteCases(artifact.blueprint);
  const [prototypeMode,setPrototypeMode]=useState(false);
  const [preparingOnly, setPreparingOnly] = useState(false);
  const [focusedCase, setFocusedCase] = useState<string | undefined>(selectedCase);
  const instructionDraft = useVibeEditorDraft(projectID, artifact.id, "instructions", editorSource({ id: artifact.id, agent_prompt: artifact.agent_prompt }), artifact.agent_prompt);
  const casesDraft = useVibeEditorDraft(projectID, artifact.id, "cases", editorSource({ id: artifact.id, blueprint: artifact.blueprint }), JSON.stringify(tests));
  const editorHeading = useRef<HTMLHeadingElement>(null);
  const [editing, setEditing] = useState(casesDraft.dirty || tests.some(test => test.key === selectedCase && test.editable));
  const [previousCase, setPreviousCase] = useState(selectedCase);
  if (previousCase !== selectedCase) {
    setPreviousCase(selectedCase);
    setFocusedCase(selectedCase);
    setEditing(tests.some(test => test.key === selectedCase && test.editable));
  }
  const [agentOpen, setAgentOpenState] = useState(mode !== "tests" && instructionDraft.dirty);
  const setAgentOpen = (open: boolean) => { if (open && mode === "tests") onAgent?.(); else setAgentOpenState(open); };
  const instructions = instructionDraft.value;
  const setInstructions = instructionDraft.setValue;
  const draft = JSON.parse(casesDraft.value) as typeof tests;
  const setDraft = (next: typeof tests | ((old: typeof tests) => typeof tests)) => casesDraft.setValue(JSON.stringify(typeof next === "function" ? next(draft) : next));
  useEffect(() => { onDirty(instructionDraft.dirty || casesDraft.dirty); }, [instructionDraft.dirty, casesDraft.dirty]);
  const restoreFocus = () => requestAnimationFrame(() => editorHeading.current?.focus());
  const [copied, setCopied] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);
  const cases = tests;
  const changes = changedCases(tests, draft);
  const ready = !!artifact.agent_prompt.trim();
  const ran = ready && !!onResult;
  // The editor and the agent form hide the actions, and with them "See the result".
  const resultShown = ran && !agentOpen && !editing && !artifact.unavailable_reason && cases.length > 0;
  useLayoutEffect(() => { onResultShown?.(resultShown); return () => onResultShown?.(false); }, [onResultShown, resultShown]);
  const added = parent ? tests.filter(test => !suiteCases(parent.blueprint).some(old => old.key===test.key)).length : 0;
  const validationBlocked = !!artifact.validation && artifact.validation.status !== "supported";
  if (artifact.unavailable_reason || !cases.length) return <section className="vibe-panel p-4 space-y-3"><h2>Pack kept · advanced editor needed</h2><p className="text-sm vibe-muted">{artifact.unavailable_reason || "These tests need the full pack builder."} No tests were removed or run.</p>{artifact.original&&onOriginal?<VibeButton onClick={onOriginal}>Download original {artifact.original.filename}</VibeButton>:<VibeButton onClick={()=>downloadJSON("pack-definition.json",artifact.blueprint)}>Download definition</VibeButton>}<VibeButton onClick={onSave}>Save and open advanced editor</VibeButton></section>;

  return (
    <section data-proposal-id={artifact.id} aria-label="Your tests" className="space-y-6">
      {(instructionDraft.conflict || casesDraft.conflict) && <div role="alert"><p>The saved source changed. Review your draft before applying it.</p><VibeButton onClick={() => { instructionDraft.preserve(); casesDraft.preserve(); }}>Keep my draft for this source</VibeButton><VibeButton onClick={() => { instructionDraft.discard(); casesDraft.discard(); restoreFocus(); }}>Discard my draft</VibeButton></div>}
      <div className="vibe-tests-header">
        <div>
        <h2 ref={editorHeading} tabIndex={-1}>
          {validationBlocked
            ? `${cases.length} ${cases.length === 1 ? "test needs" : "tests need"} review`
            : pendingPolicy
              ? `${cases.length} ${cases.length === 1 ? "test from" : "tests from"} your previous policy`
            : comparison
            ? "Ready to test the fix"
            : buildJourney && parent ? "Your updated version is ready" : `${cases.length} ${cases.length === 1 ? "test is" : "tests are"} ready`}
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
        {!agentOpen && !editing && <div className="vibe-tests-actions">
          <div className="flex flex-wrap items-center gap-2">
            {ran && <VibeButton variant={primary ? "primary" : "secondary"} onClick={onResult}>See the result<ArrowRight /></VibeButton>}
            <VibeButton
              variant={primary && !ran ? "primary" : "secondary"}
              disabled={busy || blocked || validationBlocked || (inferenceUnavailable && (ready || !preparingOnly))}
              onClick={() => (preparingOnly && !ready ? onSave() : ready ? onRun() : buildJourney && rules.length ? void onEdit({ artifact_id: artifact.id, build_from_policy: true }) : setAgentOpen(true))}
            >
              {ran ? "Run again" : ready
                ? pendingPolicy
                  ? "Run previous tests"
                  : comparison
                  ? rerunLabel(buildJourney)
                  : buildJourney && parent ? "Update and run" : `Run ${cases.length} ${cases.length === 1 ? "test" : "tests"}`
                : preparingOnly ? "Keep these tests" : buildJourney && rules.length ? "Build from saved rules" : "Add your agent"}
              <ArrowRight />
            </VibeButton>
            {tests.some(t => t.editable) && <VibeButton
              variant="quiet"
              disabled={busy}
              onClick={() => { setFocusedCase(undefined); setEditing(true); onSelectCase?.(); }}
            >
              <Pencil />
              Edit tests
            </VibeButton>}
          </div>
          {ready && (
            <p className="text-sm vibe-muted">
              Text test here · {modelLabel || "Selected model"}. Your live app is not called.
            </p>
          )}
        </div>}
      </div>
      {added>0 && <p className="text-sm vibe-muted">{testCount(added, "new")} · {testCount(cases.length-added, "existing")}. This run includes all {cases.length}; earlier results are kept separately.</p>}
      {!editing && !agentOpen && <TestMeaning />}
      {artifact.validation && <details className="vibe-test-review" open={validationBlocked}><summary>Test review: {validationBlocked ? "Needs attention" : "Ready"}</summary><ValidationDetails validation={artifact.validation} /></details>}
      {rules.length > 0 && !editing && !agentOpen && (
        <details open={validationBlocked || pendingPolicy} className="text-sm vibe-muted">
          <summary className="w-fit cursor-pointer rounded-sm focus-visible:outline-2 focus-visible:outline-offset-4">
            Rules being tested
          </summary>
          <ul className="mt-3 list-disc space-y-2 pl-5">
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
                <span className="vibe-row-title min-w-0 flex-1">
                  {test.input}
                </span>
                <ChevronRight className="vibe-chevron vibe-muted" size={16} />
              </summary>
              <div className="border-t border-[var(--vibe-border)] p-4">
                <p className="mb-1 text-xs vibe-muted">What should happen</p>
                <p>{test.expected}</p>
                {(onAskCase || test.editable) && <div className="mt-3 flex flex-wrap gap-2">
                  {onAskCase && <VibeButton onClick={() => onAskCase(test.key)}>Ask Guide about this test</VibeButton>}
                  {test.editable && <VibeButton disabled={busy} onClick={() => { setFocusedCase(test.key); setEditing(true); onSelectCase?.(test.key); }}>Edit this expectation</VibeButton>}
                </div>}
                {!test.editable && <p className="mt-2 text-sm vibe-muted">To change custom grading rules, edit the original pack and import it again.</p>}
              </div>
            </details>
          ))}
        </div>
      )}
      {editing && draft && (
        <div className="vibe-panel space-y-4 p-4">
          {checkingChanges && <p role="status" className="text-sm vibe-muted">Reviewing these test changes…</p>}
          {draft.map((test, i) => test.editable && (!focusedCase || focusedCase === test.key) && (
            <div key={test.key} className="space-y-3">
              {(focusedCase ? ["expected"] as const : ["input", "expected"] as const).map((field) => (
                <label key={field} className="block vibe-muted">
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
                casesDraft.conflict || !changes.length || draft.some(
                  (t) => !t.input.trim() || !t.expected.trim(),
                )
              }
              onClick={async () => {
                const trigger = document.activeElement;
                if (
                  await onEdit({
                    artifact_id: artifact.id,
                    case_changes: changes,
                  })
                ) {
                  casesDraft.discard(); setEditing(false); onSelectCase?.(); if (document.activeElement === trigger) restoreFocus();
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
                casesDraft.discard();
                setEditing(false); onSelectCase?.(); restoreFocus();
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
          className="vibe-panel space-y-4 p-4"
          onSubmit={async (e) => {
            e.preventDefault();
            if (busy || instructionDraft.conflict || !instructions.trim()) return;
            if (
              await onEdit({
                artifact_id: artifact.id,
                agent_prompt: instructions,
              })
            ) {
              instructionDraft.discard(); setAgentOpen(false); restoreFocus();
              onDirty(false);
            }
          }}
        >
          <div>
            <h3>{prototypeMode ? "Describe an agent to try" : "Add your agent’s instructions"}</h3>
            <p className="mt-1 text-sm vibe-muted">
              We’ll run these with {modelLabel || "the selected model"}. This
              tests text replies; your app’s tools and data aren’t connected.
            </p>
          </div>
          {prototypeMode && <p className="text-sm vibe-muted">Use your own description and rules. The test pack’s expected answers won’t be added to the agent instructions.</p>}
          <label className="block text-sm">
            {prototypeMode ? "Its job and your rules" : "Agent instructions"}
            <textarea
              aria-label={prototypeMode ? "Its job and your rules" : "Agent instructions"}
              className="vibe-textarea mt-2 min-h-40"
              value={instructions}
              onChange={(e) => {
                setInstructions(e.target.value);
                onDirty(true);
              }}
              placeholder={prototypeMode ? "Help customers decide whether their order qualifies for a refund. Our rules are…" : "Paste the instructions your agent actually uses…"}
              autoFocus
            />
          </label>
          <div className="flex flex-wrap gap-2">
            <VibeButton
              variant={primary ? "primary" : "secondary"}
              type="submit"
              disabled={busy || instructionDraft.conflict || !instructions.trim()}
            >
              {prototypeMode ? "Create agent" : "Use these instructions"} <ArrowRight />
            </VibeButton>
            <VibeButton
              variant="quiet"
              disabled={busy}
              onClick={() => {
                setAgentOpen(false);
                instructionDraft.discard(); restoreFocus();
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
          {!ready && <div className="text-sm vibe-muted">
            {preparingOnly ? <><p>Keep these tests for your team. Add an agent when you’re ready to run them.</p><VibeButton variant="quiet" className="!px-0" disabled={busy} onClick={() => setAgentOpen(true)}>Add an agent later</VibeButton></>
              : <><p>{buildJourney && rules.length ? "Use your saved requirements to create a version you can try. We’ll review these saved expectations before running; no answers are copied into its instructions." : "A test pack supplies tests and expectations. Add an agent to get actual replies."}</p><VibeButton variant="quiet" className="!px-0" disabled={busy} onClick={() => {setPrototypeMode(true);setAgentOpen(true)}}>Create an agent</VibeButton><VibeButton variant="quiet" disabled={busy} onClick={() => setPreparingOnly(true)}>Keep for later</VibeButton></>}
          </div>}
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
