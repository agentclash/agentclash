"use client";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Tabs } from "@base-ui/react/tabs";
import { Sheet, SheetContent, SheetDescription, SheetTitle } from "@/components/ui/sheet";
import { terminal, type Operation } from "@/lib/vibe";
import { buildVersion } from "@/lib/vibe-build-timeline";
import { useVibeEditorDraft } from "@/lib/use-vibe-editor-draft";
import { editorSource, readEditorDraft } from "@/lib/vibe-build-drafts";
import type { EvaluationWorkspaceProps } from "./evaluation-workspace";
import { AppearanceControl } from "./appearance";
import { evaluationIdentity } from "./evaluation-navigation";
import { TestSuitePanel } from "./test-suite-panel";
import { suiteCases } from "./test-suite-cases";
import { VibeScorecard } from "./scorecard";
import { PromptChange } from "./prompt-change";
import { TaskSources } from "./task-sources";
import { VibeButton } from "./vibe-button";
import { Message } from "./vibe-message";
import { OperationFeedback } from "./operation-feedback";
import { ConversationActions } from "./conversation-actions";
import { ConversationGuidance } from "./conversation-guidance";
import { ActivityStatus, conversationActivity } from "./activity-status";
import { sendOnEnter } from "./composer-keyboard";
import { SavedWorkDialog } from "./saved-work-dialog";
import { BuildNextActions } from "./build-next-actions";
import { EvaluationOutcome } from "./evaluation-outcome";
import { CoverageNote } from "./coverage-note";
import "./project-workspace.css";

type Reference = { project: string; artifact_id?: string; viewed_run_id?: string; viewed_case_key?: string; title: string };
function AgentEditor({ p, onDirty }: { p: EvaluationWorkspaceProps; onDirty: () => void }) {
  const artifact = p.artifact!;
  const draft = useVibeEditorDraft(p.session!.id, artifact.id, "instructions", editorSource({ id: artifact.id, agent_prompt: artifact.agent_prompt }), artifact.agent_prompt);
  const heading = useRef<HTMLHeadingElement>(null);
  const form = useRef<HTMLFormElement>(null);
  const [editing, setEditing] = useState(draft.dirty || !artifact.agent_prompt);
  useEffect(onDirty, [draft.dirty]);
  return <section aria-label="Agent instructions" className="space-y-4"><h2 ref={heading} tabIndex={-1}>Agent instructions</h2>
    {draft.conflict && <div role="alert"><p>The saved source changed. Review your draft before applying it.</p><VibeButton onClick={draft.preserve}>Keep my draft for this source</VibeButton></div>}
    {editing ? <form ref={form} className="space-y-3" onSubmit={async event => {
      event.preventDefault(); if (p.busy || draft.conflict || !draft.value.trim()) return;
      const trigger = document.activeElement; const submitted = draft.value;
      if (await p.onEdit({ artifact_id: artifact.id, agent_prompt: submitted })) {
        draft.discard(submitted); setEditing(false); onDirty();
        const retainedFocus = () => document.activeElement === trigger || trigger instanceof HTMLElement && !trigger.isConnected && document.activeElement === document.body;
        if (retainedFocus()) requestAnimationFrame(() => { if (retainedFocus()) (heading.current || document.querySelector<HTMLElement>('.vibe-project-main [aria-label="Agent instructions"] h2'))?.focus(); });
      }
    }}><label>Agent instructions<textarea aria-label="Agent instructions" className="vibe-textarea" value={draft.value} onChange={event => { draft.setValue(event.target.value); onDirty(); }} /></label>
      <p className="vibe-muted">Uses {p.modelLabel || "the selected model"}. Your live tools and systems are not connected.</p>
      <VibeButton type="submit" disabled={p.busy || draft.conflict || !draft.value.trim()}>Use these instructions</VibeButton>
      <VibeButton onClick={() => { draft.discard(); setEditing(false); onDirty(); requestAnimationFrame(() => heading.current?.focus()); }}>Discard instruction changes</VibeButton></form>
      : <><pre className="vibe-instructions-text">{artifact.agent_prompt || "No agent instructions yet."}</pre><VibeButton onClick={() => { setEditing(true); requestAnimationFrame(() => form.current?.querySelector("textarea")?.focus()); }}>Change agent instructions</VibeButton></>}
  </section>;
}

// Projection only. The client retains journal, drafts, admission and recovery ownership.
export function ProjectWorkspace({ p, initial }: { p: EvaluationWorkspaceProps; initial: ReactNode }) {
  const session = p.session!;
  const [utility, setUtility] = useState<"guide" | "details" | null>(() => !!p.artifact && typeof matchMedia !== "undefined" && matchMedia("(min-width: 1280px)").matches ? "guide" : null);
  const [wide, setWide] = useState(() => typeof matchMedia !== "undefined" && matchMedia("(min-width: 1280px)").matches);
  const [reference, setReference] = useState<Reference>();
  const utilityTrigger = useRef<HTMLElement | null>(null);
  const utilityHeading = useRef<HTMLHeadingElement>(null);
  const composer = useRef<HTMLTextAreaElement>(null);
  const guideBody = useRef<HTMLDivElement>(null);
  const [read, setRead] = useState<string>();
  const view = p.view === "checks" ? "results" : p.view === "tests" ? "tests" : "agent";
  const artifact = p.artifact;
  const runs = session.operations.filter(op => op.kind === "check" || op.kind === "retest");
  const result = p.requestedRunID ? runs.find(op => op.id === p.requestedRunID) : runs.at(-1);
  const parent = session.document.artifacts.find(item => item.id === artifact?.parent_id);
  const active = session.operations.find(op => !terminal(op.state));
  const messages = session.document.messages.filter(message => message.origin !== "playground");
  const latestResponse = messages.findLast(message => message.role === "assistant");
  const initialBrief = !artifact || artifact.kind === "task_brief";
  const selectedReference = reference?.project === session.id && (!reference.artifact_id || session.document.artifacts.some(item => item.id === reference.artifact_id)) && (!reference.viewed_run_id || runs.some(run => run.id === reference.viewed_run_id)) ? reference : undefined;
  useEffect(() => {
    if (typeof matchMedia !== "function") return;
    const media = matchMedia("(min-width: 1280px)"); const update = () => setWide(media.matches);
    media.addEventListener("change", update); return () => media.removeEventListener("change", update);
  }, []);
  const open = (next: "guide" | "details") => { const trigger = document.activeElement as HTMLElement; utilityTrigger.current = trigger; setUtility(next); if (wide) requestAnimationFrame(() => { if (document.activeElement === trigger) utilityHeading.current?.focus(); }); };
  const close = () => { setUtility(null); if (wide) requestAnimationFrame(() => utilityTrigger.current?.focus()); };
  const ask = (next: Reference) => { setReference(next); open("guide"); requestAnimationFrame(() => composer.current?.focus()); };
  const askRun = (run: Operation, caseKey?: string) => ask({ project: session.id, viewed_run_id: run.id, ...(caseKey ? { viewed_case_key: caseKey } : {}), title: caseKey ? run.results.find(item => item.case_key === caseKey)?.title || caseKey : `Run ${runs.indexOf(run) + 1}` });
  const dirty = () => {
    if (!artifact) return;
    const instructions = readEditorDraft(session.id, artifact.id, "instructions"); const cases = readEditorDraft(session.id, artifact.id, "cases");
    p.onDirty(!!instructions && instructions.value !== artifact.agent_prompt || !!cases && cases.value !== JSON.stringify(suiteCases(artifact.blueprint)));
  };
  const send = () => {
    if (p.busy || p.dirty || p.sendBlocked || !p.content.trim()) return;
    const { project: _project, title: _title, ...context } = selectedReference || {};
    p.onSend(selectedReference ? context : undefined);
  };
  const latest = () => { const node = guideBody.current?.querySelector<HTMLElement>(`[data-message-id="${latestResponse?.id}"]`); node?.scrollIntoView?.({ block: "nearest", behavior: "auto" }); node?.focus({ preventScroll: true }); setRead(latestResponse?.id); };
  const guide = <>
    <div className="vibe-utility-scroll" ref={guideBody}>
      {initialBrief ? <p>Write your brief in Agent. Your estimate and materials stay with that brief.</p> : <>
        {messages.map(message => <div key={message.id} tabIndex={-1} data-message-id={message.id} aria-label={message.role === "assistant" ? "Guide response" : "Your message"}>
          <Message message={message} animate={false} /><ConversationGuidance cards={message.cards} scope={session.document.conversation_state?.brief.scope_id} message={message.id} />
          {message.role === "user" && message.operation_id && <OperationFeedback serverTime={session.server_time} original={session.operations.find(op => op.id === message.operation_id)} operation={session.operations.findLast(op => op.retry_of_operation_id === message.operation_id) || session.operations.find(op => op.id === message.operation_id)} busy={p.busy} pendingID={p.retryPendingOperationID} uncertainID={p.retryUncertainOperationID} onRetry={p.onRetry} retryModels={p.retryModels} />}
        </div>)}
        {p.pendingMessage && !messages.some(message => message.id === p.pendingMessage?.id) && <Message pending message={{ ...p.pendingMessage, role: "user" }} />}
        <ActivityStatus label={p.pendingLabel || (active ? conversationActivity(active, p.testJourney) : undefined)} working={active?.state === "RUNNING"} />
        {active && <VibeButton disabled={!!p.pendingLabel} onClick={() => p.onAction(active.id, active.state === "AWAITING_APPROVAL" ? "approve" : "stop")}>{active.state === "AWAITING_APPROVAL" ? "Approve run" : "Stop"}</VibeButton>}
        {p.interactionActions && p.onChoice && p.onReloadChoices && <ConversationActions session={session} busy={p.busy || p.dirty} primary onAction={p.onChoice} onReload={p.onReloadChoices} />}
      </>}
    </div>
    {!initialBrief && <div className="vibe-guide-composer">
      {latestResponse && <VibeButton variant="quiet" onClick={latest}>{read === latestResponse.id ? "Latest" : "New response"}</VibeButton>}
      {selectedReference && <div aria-label="Guide reference" className="vibe-run-reference"><span>{selectedReference.title} · Read-only context</span><VibeButton aria-label="Remove Guide reference" onClick={() => setReference(undefined)}>Remove reference</VibeButton></div>}
      <form className="vibe-composer" onSubmit={event => { event.preventDefault(); send(); }}><label htmlFor="vibe-message">Message Guide</label><textarea ref={composer} id="vibe-message" aria-label="Message Vibe Evals" rows={3} value={p.content} onChange={event => p.onContent(event.target.value)} onKeyDown={event => sendOnEnter(event, send)} /><VibeButton type="submit" aria-label="Send message" disabled={p.busy || p.dirty || p.sendBlocked || !p.content.trim()}>Send</VibeButton></form>
      {p.sendError && <p role="alert">{p.sendError}</p>}
    </div>}
  </>;
  const details = <div className="vibe-utility-scroll" aria-label="Project details"><h3>{evaluationIdentity(session).title}</h3><p>{evaluationIdentity(session).scope}</p><p>Version: {artifact ? `v${buildVersion(session, artifact)} · ${artifact.title}` : "Initial brief"}</p><p>Evidence: {result ? `Run ${runs.indexOf(result) + 1} · ${result.state}` : "No saved runs"}</p><AppearanceControl /><VibeButton onClick={p.onSettings}>Settings</VibeButton></div>;
  const utilityContent = <><div className="vibe-utility-heading"><h2 ref={utilityHeading} tabIndex={-1}>{utility === "guide" ? "Guide" : "Details"}</h2><VibeButton aria-label={`Close ${utility === "guide" ? "Guide" : "Details"}`} onClick={close}>Close</VibeButton></div>{utility === "guide" ? guide : details}</>;
  return <>
    <header className="vibe-project-header"><div className="vibe-project-identity">{p.navigationToggle}<div><h1>{evaluationIdentity(session).title}</h1><p>{artifact ? `v${buildVersion(session, artifact)} · ${artifact.title}` : "Initial brief"}{result ? ` · Evidence: Run ${runs.indexOf(result) + 1} (${result.state})` : " · No saved runs"}</p></div></div>
      <div className="vibe-project-tools"><VibeButton aria-pressed={utility === "guide"} onClick={() => open("guide")}>Guide</VibeButton><VibeButton aria-pressed={utility === "details"} onClick={() => open("details")}>Details</VibeButton></div>
      <Tabs.Root value={view} onValueChange={next => p.onNavigate(next === "results" ? "checks" : next === "tests" ? "tests" : "build")}><Tabs.List aria-label="Workspace views" className="vibe-nav">{["agent", "tests", "results"].map(value => <Tabs.Tab className="vibe-button vibe-button-quiet" id={`vibe-tab-${value}`} aria-controls="vibe-workspace-panel" key={value} value={value}>{value[0].toUpperCase() + value.slice(1)}</Tabs.Tab>)}</Tabs.List></Tabs.Root>
    </header>
    <div className="vibe-project-body" data-utility-open={!!utility && wide}>
      <div className="vibe-project-main" data-initial={view === "agent" && initialBrief || undefined} id="vibe-workspace-panel" role="tabpanel" aria-labelledby={`vibe-tab-${view}`}>
        {view === "agent" ? initialBrief ? initial : <>
          <section aria-label="Version scope"><p>{artifact!.accepted ? "Saved version" : artifact!.dismissed ? "Earlier proposal" : "Proposed version"}{artifact!.sample ? " · Sample demonstration" : ""}</p><p className="vibe-muted">{artifact!.scope_note || "Runs here using what you supplied; your business systems are not connected."}</p>{artifact!.sample && <p>Sample rules are for this demonstration. Your real policy is still unspecified.</p>}</section>
          <AgentEditor key={artifact!.id} p={p} onDirty={dirty} />
          {parent && parent.agent_prompt !== artifact!.agent_prompt && <PromptChange key={artifact!.id} before={parent.agent_prompt} after={artifact!.agent_prompt} />}
          <section aria-label="Agent references"><h2>References</h2><p>{session.document.policies?.find(policy => policy.id === artifact!.policy_id)?.rules.map(rule => rule.statement).join("\n") || "No adopted rules in this version."}</p>{!!artifact!.reference_inputs?.length && <TaskSources key={`${session.id}:${artifact!.id}`} sessionID={session.id} bindings={artifact!.reference_inputs} />}</section>
          {artifact!.agent_prompt && <VibeButton onClick={() => { p.onNavigate("try", artifact!.id); requestAnimationFrame(() => document.getElementById("vibe-trial-message")?.focus()); }}>Try it yourself</VibeButton>}
          {artifact!.agent_prompt ? p.preview : <p>Add instructions to try this agent.</p>}
        </> : view === "tests" ? artifact?.kind === "test_suite" ? <TestSuitePanel key={artifact.id} mode="tests" onAgent={() => p.onNavigate("build")} onAskCase={key => ask({ project: session.id, artifact_id: artifact.id, viewed_case_key: key, title: suiteCases(artifact.blueprint).find(test => test.key === key)?.input || key })} projectID={session.id} selectedCase={p.selectedCase} onSelectCase={p.onSelectCase} artifact={artifact} parent={parent} busy={p.busy} blocked={p.dirty} comparison={false} buildJourney={session.document.evaluation?.door === "build"} inferenceUnavailable={p.inferenceUnavailable} checkingChanges={p.checkingTestChanges} onEdit={p.onEdit} onDirty={dirty} onRun={() => p.onRun()} onSave={() => p.onSave()} onSettings={p.onSettings} modelLabel={p.modelLabel} rules={session.document.policies?.find(policy => policy.id === artifact.policy_id)?.rules} /> : <p>No tests in this version yet. Open Guide to discuss examples.</p>
        : result ? <>
          <OperationFeedback serverTime={session.server_time} original={session.operations.find(op => op.id === result.retry_of_operation_id) || result} operation={result} busy={p.busy} pendingID={p.retryPendingOperationID} uncertainID={p.retryUncertainOperationID} onRetry={p.onRetry} retryModels={p.retryModels} primary />
          {!terminal(result.state) && <VibeButton disabled={!!p.pendingLabel} onClick={() => p.onAction(result.id, result.state === "AWAITING_APPROVAL" ? "approve" : "stop")}>{result.state === "AWAITING_APPROVAL" ? "Approve run" : "Stop"}</VibeButton>}
          <label>Result history<select aria-label="Result history" value={result.id} onChange={event => p.onSelectRun(event.target.value)}>{runs.map((run, i) => <option key={run.id} value={run.id}>Run {i + 1} · {run.state}</option>)}</select></label>
          <VibeScorecard operation={result} findingFirst testJourney={p.testJourney} primary baseline={runs.find(run => run.id === result.baseline_id)} eventCursor={session.event_cursor} busy={p.busy || p.dirty} loadEvidence={key => p.loadEvidence(result.id, key)} onImprove={() => p.onMessage("Suggest a focused improvement using the failed examples. Preserve expectations.", result)} onRetest={() => p.onRun(result)} onDispute={(rule, evidence) => { p.onDispute(rule, evidence, result); open("guide"); }} onNewCheck={() => p.onRun(undefined, result.source?.evidence_set_id)} onSave={() => p.onSave(result)} onRegrade={p.onRegrade ? () => p.onRegrade!(result) : undefined} />
          {terminal(result.state) && <><VibeButton onClick={() => askRun(result)}>Ask about this result</VibeButton>{result.results.map(test => <VibeButton key={test.case_key} onClick={() => askRun(result, test.case_key)}>Ask Guide about {test.title || test.case_key}</VibeButton>)}</>}
          {artifact && terminal(result.state) && <BuildNextActions session={session} artifact={artifact} operation={result} hasOutput={false} busy={p.busy || p.dirty} primary={false} onTry={() => p.onNavigate("try", artifact.id)} onDetails={run => p.onSelectRun(run.id)} onGuide={() => open("guide")} onSave={p.onSave} onTougher={p.onTougher} loadEvidence={p.loadEvidence} />}
          {artifact && terminal(result.state) && <EvaluationOutcome session={session} artifact={artifact} operation={result} busy={p.busy || p.dirty} loadEvidence={p.loadEvidence} onTougher={p.onTougher} />}
          {terminal(result.state) && <CoverageNote rows={session.rule_coverage?.[result.source?.artifact_id || ""] || []} busy={p.busy || p.dirty} onSuggest={rule => { p.onContent(`Prepare an extra test for review for this rule: ${rule}`); open("guide"); }} />}
        </> : <p role="status">{p.requestedRunID || p.quickChecking ? "Waiting for your check…" : "No saved results yet. Run your tests to collect evidence."}</p>}
        {!(view === "agent" && initialBrief) && <div className="vibe-notice">{p.notice}</div>}
      </div>
      {wide && utility && <aside className="vibe-utility" aria-label={`${utility === "guide" ? "Guide" : "Details"} utility`}>{utilityContent}</aside>}
    </div>
    {!wide && <Sheet open={!!utility} onOpenChange={value => { if (!value) setUtility(null); }}><SheetContent className="vibe-utility-sheet" showCloseButton={false} initialFocus={utilityHeading} finalFocus={() => utilityTrigger.current || false}><SheetTitle className="sr-only">{utility === "guide" ? "Guide" : "Details"}</SheetTitle><SheetDescription className="sr-only">Supporting project context. Closing keeps your draft.</SheetDescription>{utilityContent}</SheetContent></Sheet>}
    <span className="sr-only" role="status">{latestResponse && read !== latestResponse.id ? "New Guide response available." : ""}</span>
    <SavedWorkDialog status={p.savedListStatus} onRetry={p.onRetrySaved} open={!!p.savedWorkOpen} onOpenChange={p.onSavedWorkOpenChange || (() => {})} testJourney={p.testJourney} savedChecks={p.savedChecks} hasResults={runs.length > 0} onResults={() => p.onNavigate("checks")} />
  </>;
}
