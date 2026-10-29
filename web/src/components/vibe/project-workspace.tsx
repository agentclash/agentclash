"use client";
import { hasConversationChoice, runStateLabel } from "@/lib/vibe-presentation";
import { parseResultSelection } from "@/lib/vibe-result-selection";
import { VibeDataUseNotice } from "./vibe-data-use";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { Tabs } from "@base-ui/react/tabs";
import { Info, MessageSquare } from "lucide-react";
import { Sheet, SheetContent, SheetDescription, SheetTitle } from "@/components/ui/sheet";
import { terminal, type Operation } from "@/lib/vibe";
import { buildVersion, instructionChangeBaseline } from "@/lib/vibe-build-timeline";
import { useVibeEditorDraft } from "@/lib/use-vibe-editor-draft";
import { editorSource, readEditorDraft } from "@/lib/vibe-build-drafts";
import type { EvaluationWorkspaceProps } from "./evaluation-workspace";
import { AppearanceControl } from "./appearance";
import { evaluationIdentity } from "./evaluation-navigation";
import { GradeReviews } from "./grade-reviews";
import { ValidationHistory } from "./validation-review";
import { ResultComparison } from "./result-comparison";
import { ResultRecoveryControls, offersFullRun } from "./result-recovery";
import { RecordedIntake, RecordedIntakeRecovery } from "./recorded-intake";
import { RecordedTests } from "./recorded-tests";
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
import { ProgressStrip, useRunTotal } from "./activity-status";
import { sendOnEnter } from "./composer-keyboard";
import { SavedWorkDialog } from "./saved-work-dialog";
import { BuildNextActions, ResultContact } from "./build-next-actions";
import { EvaluationOutcome } from "./evaluation-outcome";
import { CoverageNote } from "./coverage-note";
import { guidePreference, setGuidePreference } from "@/lib/vibe-project";
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
    }}><VibeDataUseNotice /><label>Agent instructions<textarea aria-label="Agent instructions" className="vibe-textarea" value={draft.value} onChange={event => { draft.setValue(event.target.value); onDirty(); }} /></label>
      <p className="text-sm vibe-muted">Uses {p.modelLabel || "the selected model"}. Your live tools and systems are not connected.</p>
      <VibeButton type="submit" disabled={p.busy || draft.conflict || !draft.value.trim()}>Use these instructions</VibeButton>
      <VibeButton onClick={() => { draft.discard(); setEditing(false); onDirty(); requestAnimationFrame(() => heading.current?.focus()); }}>Discard instruction changes</VibeButton></form>
      : <><pre className="vibe-instructions-text">{artifact.agent_prompt || "No agent instructions yet."}</pre><VibeButton onClick={() => { setEditing(true); requestAnimationFrame(() => form.current?.querySelector("textarea")?.focus()); }}>Change agent instructions</VibeButton></>}
  </section>;
}

// Projection only. The client retains journal, drafts, admission and recovery ownership.
export function ProjectWorkspace({ p, initial }: { p: EvaluationWorkspaceProps; initial: ReactNode }) {
  const session = p.session!;
  const [utility, setUtility] = useState<"guide" | "details" | null>(null);
  const [wide, setWide] = useState(() => typeof matchMedia !== "undefined" && matchMedia("(min-width: 1280px)").matches);
  const [guideClosed, setGuideClosed] = useState(guidePreference);
  const [reference, setReference] = useState<Reference>();
  const utilityTrigger = useRef<HTMLElement | null>(null);
  const afterUtilityClose = useRef<(() => void) | null>(null);
  const utilityHeading = useRef<HTMLHeadingElement>(null);
  const guideButton = useRef<HTMLButtonElement>(null);
  const composer = useRef<HTMLTextAreaElement>(null);
  const guideBody = useRef<HTMLDivElement>(null);
  const refocusGuide = useRef(false);
  const [read, setRead] = useState<string>();
  const view = p.view === "checks" ? "results" : p.view === "tests" ? "tests" : "agent";
  const artifact = p.artifact;
  const runs = session.operations.filter(op => op.kind === "check" || op.kind === "retest");
  const testedArtifactID = (run: Operation) => run.source?.artifact_id || run.results[0]?.version;
  const candidate = p.requestedRunID ? runs.find(op => op.id === p.requestedRunID) : runs.findLast(op => testedArtifactID(op) === artifact?.id);
  // Canonical explicit history pins the selected artifact in the controller.
  // Until that agrees, never combine one version with another version's evidence.
  const result = !p.selectedResult && candidate && artifact && testedArtifactID(candidate) === artifact.id ? candidate : undefined;
  const resultArtifact = result ? session.document.artifacts.find(item => item.id === testedArtifactID(result)) : undefined;
  const parent = session.document.artifacts.find(item => item.id === artifact?.parent_id);
  const changeBaseline = instructionChangeBaseline(session, artifact);
  const active = session.operations.find(op => !terminal(op.state));
  // Retry is the one primary action only while the shown run is unfinished or failed.
  const retryLeads = !!result && (!terminal(result.state) || result.state === "FAILED");
  const messages = session.document.messages.filter(message => message.origin !== "playground");
  const latestResponse = messages.findLast(message => message.role === "assistant");
  const latestOperation = session.operations.at(-1);
  const guideRecovery = !!p.sendError || !!p.retryUncertainOperationID ||
    !!latestOperation?.retryable && !latestOperation.completion_receipt && terminal(latestOperation.state);
  const initialBrief = !artifact || artifact.kind === "task_brief";
  // Progress, Stop and Approve live in the progress strip, so Guide flags only what needs Guide.
  // A brief has no Guide (its chat is the Agent view), so it has no Guide flag either.
  const guideAttention = initialBrief ? undefined
    : active?.state === "AWAITING_INPUT" ? "Input needed"
    : guideRecovery ? "Needs attention"
    : p.interactionActions && hasConversationChoice(session) ? "Review needed"
    : latestResponse && read !== latestResponse.id ? "New response" : undefined;
  // From 1280px the Guide is a side panel that stays open after the first version until the user closes it.
  const shown = utility ?? (wide && !initialBrief && !guideClosed ? "guide" : null);
  const runTotal = useRunTotal(active);
  // Below 1280 the utility is a modal sheet. It counts as open until its close completes,
  // so the run estimate opens only after it, never on top of it.
  const [sheetActive, setSheetActive] = useState(false);
  if (!wide && utility && !sheetActive) setSheetActive(true);
  if (wide && sheetActive) setSheetActive(false);
  if (p.estimateOpen && !wide && utility) setUtility(null);
  const reportSheet = p.onUtilitySheet;
  useEffect(() => { reportSheet?.(sheetActive); }, [reportSheet, sheetActive]);
  useEffect(() => () => reportSheet?.(false), [reportSheet]);
  const intakeOpen = !!p.recordedUpdate && !p.recordedUpdate.artifactID;
  const newRecordedBaseline = p.recordedUpdate?.artifactID === artifact?.id ? p.recordedUpdate?.baseline : undefined;
  const selectedReference = reference?.project === session.id && (!reference.artifact_id || session.document.artifacts.some(item => item.id === reference.artifact_id)) && (!reference.viewed_run_id || runs.some(run => run.id === reference.viewed_run_id)) ? reference : undefined;
  useEffect(() => {
    if (typeof matchMedia !== "function") return;
    const media = matchMedia("(min-width: 1280px)");
    const update = () => {
      // A desktop utility must not become a new modal above an active dialog.
      // Its draft stays in the controller and can be reopened explicitly.
      if (!media.matches && document.querySelector(".vibe-dialog[data-open]")) setUtility(null);
      // The side panel unmounts below 1280px. Focus inside it must not fall to the page.
      if (!media.matches && document.querySelector(".vibe-utility")?.contains(document.activeElement)) refocusGuide.current = true;
      setWide(media.matches);
    };
    media.addEventListener("change", update); return () => media.removeEventListener("change", update);
  }, []);
  useEffect(() => {
    if (!refocusGuide.current) return; refocusGuide.current = false;
    // A Details sheet that opens in its place takes focus itself.
    if (!document.activeElement || document.activeElement === document.body) guideButton.current?.focus();
  }, [wide]);
  // Only the Guide button changes the saved choice; other actions open the Guide for now.
  const open = (next: "guide" | "details", remember = false) => {
    const trigger = document.activeElement as HTMLElement; utilityTrigger.current = trigger;
    if (remember && wide && guideClosed) { setGuideClosed(false); setGuidePreference(false); }
    setUtility(next); if (wide) requestAnimationFrame(() => { if (document.activeElement === trigger) utilityHeading.current?.focus(); });
  };
  const close = () => {
    // The Guide that opened by default has no opener; its own button brings it back.
    const target = shown === "guide" && !utility ? guideButton : utilityTrigger;
    if (shown === "guide") setRead(latestResponse?.id);
    if (wide && shown === "guide") { setGuideClosed(true); setGuidePreference(true); }
    setUtility(null); if (wide) requestAnimationFrame(() => target.current?.focus());
  };
  // Below 1280 the utility is a modal sheet: close it before Settings opens.
  const settings = () => { if (wide) { p.onSettings(); return; } afterUtilityClose.current = p.onSettings; setUtility(null); };
  const ask = (next: Reference) => { setReference(next); open("guide"); requestAnimationFrame(() => composer.current?.focus()); };
  const askForTests = () => { open("guide"); requestAnimationFrame(() => composer.current?.focus()); };
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
  // Below 1280px the Guide sheet is modal and covers the main pane, so the strip moves into it.
  const stripInSheet = !wide && utility === "guide";
  const progress = !initialBrief && <ProgressStrip operation={active} total={runTotal} pendingLabel={p.pendingLabel} testJourney={p.testJourney} onAction={p.onAction} />;
  const guide = <>
    {stripInSheet && progress}
    <div className="vibe-utility-scroll" ref={guideBody}>
      {initialBrief ? <p className="text-sm">Write your brief in Agent. Your estimate and materials stay with that brief.</p> : <>
        {messages.map(message => <div key={message.id} tabIndex={-1} data-message-id={message.id} aria-label={message.role === "assistant" ? "Guide response" : "Your message"}>
          <Message message={message} animate={false} /><ConversationGuidance cards={message.cards} scope={session.document.conversation_state?.brief.scope_id} message={message.id} />
          {message.role === "user" && message.operation_id && <OperationFeedback serverTime={session.server_time} original={session.operations.find(op => op.id === message.operation_id)} operation={session.operations.findLast(op => op.retry_of_operation_id === message.operation_id) || session.operations.find(op => op.id === message.operation_id)} busy={p.busy} pendingID={p.retryPendingOperationID} uncertainID={p.retryUncertainOperationID} onRetry={p.onRetry} retryModels={p.retryModels} />}
        </div>)}
        {p.pendingMessage && !messages.some(message => message.id === p.pendingMessage?.id) && <Message pending message={{ ...p.pendingMessage, role: "user" }} />}
        {p.interactionActions && p.onChoice && p.onReloadChoices && <ConversationActions session={session} busy={p.busy || p.dirty} primary onAction={p.onChoice} onReload={p.onReloadChoices} />}
      </>}
    </div>
    {!initialBrief && <div className="vibe-guide-composer">
      <div className="vibe-guide-context">
      <div className="vibe-guide-tools">
        <VibeDataUseNotice className="vibe-guide-data-use" />
        {latestResponse && <VibeButton variant="quiet" onClick={latest}>{read === latestResponse.id ? "Latest" : "New response"}</VibeButton>}
      </div>
      {selectedReference && <div aria-label="Guide reference" className="vibe-run-reference"><span>{selectedReference.title} · Read-only context</span><VibeButton aria-label="Remove Guide reference" onClick={() => setReference(undefined)}>Remove reference</VibeButton></div>}
      </div>
      <form className="vibe-composer vibe-composer-compact" onSubmit={event => { event.preventDefault(); send(); }}><label htmlFor="vibe-message" className="sr-only">Message Guide</label><textarea ref={composer} id="vibe-message" aria-label="Message Vibe Evals" placeholder="Message Guide…" rows={2} value={p.content} onChange={event => p.onContent(event.target.value)} onKeyDown={event => sendOnEnter(event, send)} /><VibeButton type="submit" aria-label="Send message" disabled={p.busy || p.dirty || p.sendBlocked || !p.content.trim()}>Send</VibeButton></form>
      {p.sendError && <p role="alert">{p.sendError}</p>}
    </div>}
  </>;
  const sourceLabel = p.selectedResult?.result_ref.kind === "sdk_report" ? "Imported SDK report" : p.selectedResult?.result_ref.kind === "eval_session" ? "Application session" : "Application run";
  const identity = evaluationIdentity(session);
  const versionLabel = artifact ? `v${buildVersion(session, artifact)}${artifact.title && artifact.title !== identity.title ? ` · ${artifact.title}` : ""}` : "Initial brief";
  const details = <div className="vibe-utility-scroll text-sm" aria-label="Project details"><h3>{identity.title}</h3><p>{identity.scope}</p><p>Version: {versionLabel}</p><p>{result ? `Results: Run ${runs.indexOf(result) + 1} · ${runStateLabel(result.state)}` : p.selectedResult ? `Results: ${sourceLabel}` : "No results for this version"}</p>{p.connectorSettings && <section aria-label="Agent connection settings"><h3>Connect your application</h3>{p.connectorSettings}</section>}<AppearanceControl /><VibeButton onClick={settings}>Settings</VibeButton></div>;
  const utilityContent = <><div className="vibe-utility-heading"><h2 ref={utilityHeading} tabIndex={-1}>{shown === "guide" ? "Guide" : "Details"}</h2><VibeButton aria-label={`Close ${shown === "guide" ? "Guide" : "Details"}`} onClick={close}>Close</VibeButton></div>{shown === "guide" ? guide : details}</>;
  return <>
    <header className="vibe-project-header" data-utility-open={!!shown && wide} data-initial={view === "agent" && initialBrief || undefined}><div className="vibe-project-identity">{p.navigationToggle}<div><h1>{identity.title}</h1><p>{versionLabel}{result ? ` · Run ${runs.indexOf(result) + 1}: ${runStateLabel(result.state)}` : p.selectedResult ? ` · ${sourceLabel}` : artifact ? " · No results for this version" : " · No saved runs"}</p></div></div>
      <Tabs.Root className="vibe-project-tabs" value={view} onValueChange={next => p.onNavigate(next === "results" ? "checks" : next === "tests" ? "tests" : "build")}><Tabs.List aria-label="Workspace views" className="vibe-nav">{["agent", "tests", "results"].map(value => <Tabs.Tab className="vibe-button vibe-button-quiet" id={`vibe-tab-${value}`} aria-controls="vibe-workspace-panel" key={value} value={value}>{value[0].toUpperCase() + value.slice(1)}</Tabs.Tab>)}</Tabs.List></Tabs.Root>
      <div className="vibe-project-tools">{!initialBrief && <VibeButton ref={guideButton} aria-label="Guide" aria-describedby={shown !== "guide" && guideAttention ? "vibe-guide-attention" : undefined} aria-pressed={shown === "guide"} onClick={() => open("guide", true)}><MessageSquare className="vibe-project-tool-icon" aria-hidden="true" /><span className="vibe-project-tool-label">Guide</span>{shown !== "guide" && guideAttention && <small id="vibe-guide-attention" className="vibe-guide-attention">{guideAttention}</small>}</VibeButton>}<VibeButton aria-label="Details" aria-pressed={shown === "details"} onClick={() => open("details")}><Info className="vibe-project-tool-icon" aria-hidden="true" /><span className="vibe-project-tool-label">Details</span></VibeButton></div>
    </header>
    <div className="vibe-project-body" data-utility-open={!!shown && wide}>
      <div className="vibe-project-main" data-initial={view === "agent" && initialBrief || undefined} id="vibe-workspace-panel" role="tabpanel" aria-labelledby={`vibe-tab-${view}`}>
        <div className="vibe-project-content">
        {!stripInSheet && progress}
        {!initialBrief && !intakeOpen && p.onRecordedEvidence && <RecordedIntakeRecovery project={session.id} busy={p.busy} onSave={p.onRecordedEvidence} />}
        {view === "results" && runs.length > 0 && <label>Result history<select aria-label="Result history" value={result?.id || ""} onChange={event => { if (event.target.value) p.onSelectRun(event.target.value); }}><option value="" disabled>Choose a saved run</option>{runs.map((run, i) => <option key={run.id} value={run.id}>Run {i + 1} · {runStateLabel(run.state)}</option>)}</select></label>}
        {intakeOpen && p.onRecordedEvidence ? <section aria-label="New recorded replies"><p>Compare with the selected saved run. Keep the customer messages and expectations the same; supply the actual new app replies. Review and enter the expectations explicitly before saving.</p><RecordedIntake reviewedExpectations={session.document.artifacts.find(item=>item.id===p.recordedUpdate?.baseline.source?.artifact_id)?.conversation_evaluation?.expectations} project={session.id} revision={session.revision} busy={p.busy} onSave={p.onRecordedEvidence} onClose={p.onCloseRecordedUpdate || (()=>{})}/></section> : view === "agent" ? initialBrief ? initial : <>
          <section aria-label="Version scope"><h2>{artifact!.accepted ? "Saved version" : artifact!.dismissed ? "Earlier proposal" : "Proposed version"}{artifact!.sample ? " · Sample demonstration" : ""}</h2><p className="vibe-muted">{artifact!.conversation_evaluation ? "Recorded app replies with your explicit expectations. Your app is not called." : artifact!.scope_note || "Runs here using what you supplied; your business systems are not connected."}</p>{artifact!.sample && <p>Sample rules are for this demonstration. Your real policy is still unspecified.</p>}</section>
          {artifact!.conversation_evaluation ? <section><h2>Recorded replies</h2><p>These are replies you supplied from your app. Your app model, build and live tools are unverified. Review the exact messages and expectations in Tests.</p><VibeButton onClick={() => p.onNavigate("tests",artifact!.id)}>Review recorded replies</VibeButton></section> : <AgentEditor key={`agent:${artifact!.id}`} p={p} onDirty={dirty} />}
          {parent && parent.agent_prompt !== artifact!.agent_prompt && <PromptChange key={`change:${artifact!.id}`} before={parent.agent_prompt} after={artifact!.agent_prompt} />}
          <section aria-label="Agent references"><h2>References</h2><p>{session.document.policies?.find(policy => policy.id === artifact!.policy_id)?.rules.map(rule => rule.statement).join("\n") || "No adopted rules in this version."}</p>{!!artifact!.reference_inputs?.length && <TaskSources key={`${session.id}:${artifact!.id}`} sessionID={session.id} bindings={artifact!.reference_inputs} />}</section>
          {artifact!.agent_prompt ? p.preview : !artifact!.conversation_evaluation && <p>Add instructions to try this agent.</p>}
        </> : view === "tests" ? artifact?.conversation_evaluation ? <RecordedTests key={artifact.id} project={session.id} artifact={artifact} blocked={p.dirty||p.runInferenceUnavailable} evidence={session.document.evidence_sets?.find(item => item.id === artifact.conversation_evaluation!.evidence_set_id)} busy={p.busy} onDirty={p.onDirty} onEdit={p.onEdit} onRun={() => p.onRun(newRecordedBaseline,artifact.conversation_evaluation!.evidence_set_id,newRecordedBaseline ? artifact.id : undefined)} /> : artifact?.kind === "test_suite" ? <TestSuitePanel key={artifact.id} mode="tests" onAgent={() => p.onNavigate("build")} onAskCase={key => ask({ project: session.id, artifact_id: artifact.id, viewed_case_key: key, title: suiteCases(artifact.blueprint).find(test => test.key === key)?.input || key })} projectID={session.id} selectedCase={p.selectedCase} onSelectCase={p.onSelectCase} artifact={artifact} parent={parent} busy={p.busy} blocked={p.dirty} comparison={!!changeBaseline} buildJourney={session.document.evaluation?.door === "build"} inferenceUnavailable={p.inferenceUnavailable} checkingChanges={p.checkingTestChanges} onEdit={p.onEdit} onDirty={dirty} onRun={() => p.onRun(changeBaseline)} onSave={() => p.onSave()} onOriginal={()=>p.onOriginal?.(artifact)} onSettings={p.onSettings} modelLabel={p.modelLabel} rules={session.document.policies?.find(policy => policy.id === artifact.policy_id)?.rules} />
          : initialBrief ? <div className="vibe-empty-state"><p>No tests yet. Write your brief in Agent first.</p><VibeButton onClick={() => p.onNavigate("build")}>Go to Agent</VibeButton></div>
          : <div className="vibe-empty-state"><p>No tests in this version yet. Guide can prepare tests for you to review.</p><VibeButton onClick={askForTests}>Ask Guide for tests</VibeButton></div>
        : result && resultArtifact ? <>
          <OperationFeedback serverTime={session.server_time} original={session.operations.find(op => op.id === result.retry_of_operation_id) || result} operation={result} busy={p.busy} pendingID={p.retryPendingOperationID} uncertainID={p.retryUncertainOperationID} onRetry={p.onRetry} retryModels={p.retryModels} primary={retryLeads} />
          {result.baseline_id && runs.find(run=>run.id===result.baseline_id) && <ResultComparison key={`comparison:${result.id}`} current={result} baseline={runs.find(run=>run.id===result.baseline_id)!} load={p.loadEvidence}/>}
          {p.testJourney && terminal(result.state) && result.source?.kind === "provided_conversations" && p.onNewReplies && <VibeButton disabled={p.busy || p.dirty} onClick={()=>p.onNewReplies!(result)}>Add new recorded replies</VibeButton>}
          <VibeScorecard onAskCase={key => askRun(result, key)} operation={result} findingFirst testJourney={p.testJourney} primary={!(retryLeads && !!result.retryable)} baseline={runs.find(run => run.id === result.baseline_id)} eventCursor={session.event_cursor} busy={p.busy || p.dirty} loadEvidence={key => p.loadEvidence(result.id, key)} onImprove={() => p.onMessage(result.source?.kind === "provided_conversations" ? "Suggest a concrete instruction change to address the observed failures. Keep the expectations fixed. Explain what to copy into my agent; do not claim to apply it." : "Suggest a focused improvement using the failed tests. Keep the expectations.", result)} onRetest={() => p.onRun(result)} onNewReplies={result.source?.kind === "provided_conversations" && p.onNewReplies ? () => p.onNewReplies!(result) : undefined} onDispute={(rule, evidence) => { p.onDispute(rule, evidence, result); open("guide"); }} onNewCheck={offersFullRun(result) ? undefined : () => p.onRun(undefined, result.source?.evidence_set_id)} onSave={() => p.onSave(result)} onRegrade={p.onRegrade ? () => p.onRegrade!(result) : undefined} />
          <ResultRecoveryControls key={`recovery:${result.id}`} operation={result} busy={p.busy||p.dirty} onReload={p.onReloadEvidence||(()=>{})} onRetry={p.onRetryCases?keys=>p.onRetryCases!(result,keys):undefined} onFullRun={()=>p.onRun(undefined,result.source?.evidence_set_id)} />
          {terminal(result.state) && <details className="vibe-results-more"><summary>More actions</summary><div className="vibe-results-more-body">
            <div><VibeButton onClick={() => askRun(result)}>Ask about this result</VibeButton></div>
            <GradeReviews key={`reviews:${result.id}`} sessionID={session.id} operation={result} eventCursor={session.event_cursor} load={key=>p.loadEvidence(result.id,key)} />
            {!resultArtifact.conversation_evaluation && <BuildNextActions showResultLink={false} showContact={false} session={session} artifact={resultArtifact} operation={result} hasOutput={false} busy={p.busy || p.dirty} primary={false} onTry={() => p.onNavigate("try", resultArtifact.id)} onDetails={run => p.onSelectRun(run.id)} onGuide={() => open("guide")} onSave={p.onSave} onTougher={p.onTougher} onExport={p.onExport} loadEvidence={p.loadEvidence} />}
            {resultArtifact.conversation_evaluation && <EvaluationOutcome onExport={p.onExport} hideContact session={session} artifact={resultArtifact} operation={result} busy={p.busy || p.dirty} loadEvidence={p.loadEvidence} onTougher={p.onTougher} />}
            <CoverageNote rows={session.rule_coverage?.[result.source?.artifact_id || ""] || []} busy={p.busy || p.dirty} onSuggest={rule => { p.onContent(`Prepare an extra test for review for this rule: ${rule}`); open("guide"); }} />
          </div></details>}
          <ResultContact session={session} artifact={resultArtifact} operation={result} busy={p.busy || p.dirty} />
        </> : p.selectedResult || (parseResultSelection(p.requestedRunID)?.kind && parseResultSelection(p.requestedRunID)?.kind !== "vibe_operation") ? null
          : p.requestedRunID || p.quickChecking ? <p role="status">Waiting for your run…</p>
          : <div className="vibe-empty-state"><p role="status">No results for this version. Run its tests to collect evidence, or choose a saved run to view an earlier tested version.</p><VibeButton onClick={() => p.onNavigate("tests")}>Go to Tests</VibeButton></div>}
        {view === "tests" && !!session.validation_reviews?.length && <details className="vibe-test-review-history" open={session.validation_reviews.some(review => review.validation.status !== "supported" || !!review.error)}><summary>Previous test reviews</summary><ValidationHistory reviews={session.validation_reviews} onFix={text => { p.onContent(text); open("guide"); }} /></details>}
        {view === "results" && p.externalResults}
        {!(view === "agent" && initialBrief) && <div className="vibe-notice">{p.notice}</div>}
        </div>
      </div>
      {wide && shown && <aside className="vibe-utility" aria-label={`${shown === "guide" ? "Guide" : "Details"} utility`}>{utilityContent}</aside>}
    </div>
    {!wide && <Sheet open={!!utility} onOpenChange={value => { if (!value) setUtility(null); }} onOpenChangeComplete={value => { const action = afterUtilityClose.current; afterUtilityClose.current = null; if (!value) { setSheetActive(false); action?.(); } }}><SheetContent className="vibe-utility-sheet" showCloseButton={false} initialFocus={utilityHeading} finalFocus={() => utilityTrigger.current || false}><SheetTitle className="sr-only">{utility === "guide" ? "Guide" : "Details"}</SheetTitle><SheetDescription className="sr-only">Supporting project context. Closing keeps your draft.</SheetDescription>{utilityContent}</SheetContent></Sheet>}
    <span className="sr-only" role="status">{shown !== "guide" && guideAttention ? `${guideAttention}. Open Guide for details.` : ""}</span>
    <SavedWorkDialog status={p.savedListStatus} onRetry={p.onRetrySaved} open={!!p.savedWorkOpen} onOpenChange={p.onSavedWorkOpenChange || (() => {})} testJourney={p.testJourney} savedChecks={p.savedChecks} hasResults={runs.length > 0} onResults={() => p.onNavigate("checks")} />
  </>;
}
