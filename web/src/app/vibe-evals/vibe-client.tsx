"use client";
import { BuildStart } from "@/components/vibe/build-start";
import { ImproveStart } from "@/components/vibe/improve-start";
import { ReimportReview } from "@/components/vibe/reimport-review";
import {ObservedResults} from "@/components/vibe/observed-results";
import {ConnectorSetup} from "@/components/vibe/connector-setup";
import {CanonicalResults} from "@/components/vibe/canonical-results";
import { SDKReportImportForm } from "@/components/vibe/sdk-report-import";
import { SDKReports } from "@/components/vibe/sdk-reports";

import {projectTitle} from "@/lib/vibe-project";
import {suiteCases} from "@/components/vibe/test-suite-cases";
import { ExportDialog } from "@/components/vibe/export-dialog";
import { VibeAppearance } from "@/components/vibe/appearance";
import { VibeConnection } from "@/lib/vibe-connection";


import { TaskInput } from "@/components/vibe/task-input";





import { sessionAccessLost } from "@/lib/use-vibe-session";





import Link from "next/link";
import { useRef, useState } from "react";



import { PrototypeTrial } from "@/components/vibe/prototype-trial";
import { EvaluationWorkspace } from "@/components/vibe/evaluation-workspace";
import { buildVersion } from "@/lib/vibe-build-timeline";
import { EvaluationNavigation } from "@/components/vibe/evaluation-navigation";
import { VibeButton } from "@/components/vibe/vibe-button";
import { CreditsDialog } from "@/components/vibe/credits-dialog";
import { AgentSettings } from "@/components/vibe/agent-settings";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";


import { dollars } from "@/lib/vibe";
import { testCount } from "@/lib/vibe-presentation";

import { keepReturnURL, savedWorkURL, advancedURL } from "@/lib/vibe-keep";

import { useVibeWorkspace } from "@/lib/use-vibe-workspace";

export function VibeClient() {
  const { startDoor, startProject, backFromStart, startBuildDraft, startInputs, setStartInputs, startError, arrival, takeStartFocus, evidenceResult,selectedCanonical,selectedSDKReport,selectedResult,selectedResultLoading,selectedResultError,retrySelectedResult,onSelectResult,onSDKPromoted,onSDKRegraded,evidenceAccess,sdkInferenceAvailable,projectReturnPath,saveDraft,saveComplete,saveNotice,saveKept,saveArtifact,saveReturnPath,saveAdvanced,advancedReceipt,archiveMode,setArchiveMode,metadata,exporter, recordedUpdate, startRecordedUpdate, closeRecordedUpdate, addRecordedEvidence, entryAvailable, authUser, authLoading, contextList, savedList, runQuoteState, runQuoteError, funding, inferenceAvailable, runInferenceAvailable, unavailableReason, messageLimit, messageError, buildQuote, buildQuoteState, refreshBuildQuote, runQuoteCurrent, isRunQuoteCurrent, accept, active, activeContext, adoptContext, applyChoice, artifact, artifacts, buildEvidence, buildJourney, buildStart, busy, changeModels, chooseDoor, closeSave, composerEdits, config, configError, connection, content, contexts, currentView, dirtyArtifact, discardedContextIDs, drafts, edit, error, evaluation, exportConversation, file, importError, importNotice, keptArtifact, latestArtifact, loadSavedCase, loadingSession, materialScope, materials, trialMaterials, models, navigate, navigation, newEvaluation, newTrial, openNewEvaluation, openSave, operationAction, pending, pendingAction, pendingEdit, pendingMessage, quoteError, quoteHeading, quoteMatches, quoteTrigger, reloadChoices, rememberSave, requestPreparation, requestRun, requestedRunID, retryOperation, retrySubmission, retryUnsentMessage, runPending, runQuote, save, saveAccess, saveBaseline, saveOpen, saveTarget, saved, savedCheck, savedChecks, savedDraft, savedModelNotice, savedWorkOpen, scenarioCount, select, selectedArtifactID, sendMessage, session, sessionID, sessionUnavailable, setChecksDirtyID, setConnection, setContent, setContexts, setError, setLoadAttempt, setModels, setNewEvaluation, setPending, setPendingMessage, setRunQuote, setSavedWorkOpen, setSelectedArtifactID, setSettingsOpen, setThreadID, setTrialText, setWorkspace, settingsOpen, snapshots, submission, submit, switchContext, testJourney, threadID, token, trialHistory, trialMessages, trialText, twoDoor, uncertain, upload, view, workspace, workspaces } = useVibeWorkspace();
  const startConfig = config as (typeof config & { first_build_ceiling_nano_usd?: number; guest_trial?: { initial_build_used: boolean; build_in_progress: boolean } });
  const buildPrice = !authUser && startConfig?.guest_trial?.initial_build_used === false ? { kind: "trial" as const } : authUser && typeof startConfig?.first_build_ceiling_nano_usd === "number" ? { kind: "ceiling" as const, nanoUSD: startConfig.first_build_ceiling_nano_usd } : { kind: "unknown" as const };
  const buildBlockedReason = !authUser && startConfig?.guest_trial?.build_in_progress ? "Another build is using your free trial. Try again when it finishes." : !entryAvailable ? unavailableReason : undefined;
  const choosingEntry = twoDoor && (!session || newEvaluation);
  const [creditsOpen, setCreditsOpen] = useState(false);
  const [utilitySheet, setUtilitySheet] = useState(false);
  // One dialog at a time: Settings closes, then the dialog it asked for opens.
  const afterSettings = useRef<(() => void) | null>(null);
  const fromSettings = (action: () => void) => { afterSettings.current = action; setSettingsOpen(false); };
  const workspaceSelect = useRef<HTMLSelectElement>(null);
  const choosingWorkspace = !!authUser && (!navigation.session || newEvaluation);
  const workspaceName = workspaces.find(item => item.id === workspace)?.name;
  // The picker shows while workspaces load or when there is one to choose; the notice covers the other states.
  const workspacePicker = choosingWorkspace && (workspaces.length > 1 || saveAccess === "loading");
  const workspaceChoice = choosingWorkspace && (workspaces.length > 1 ? <div className="vibe-entry-workspace">
    <label>Workspace for new projects<select ref={workspaceSelect} aria-label="Workspace for new projects" aria-describedby={workspaceName ? undefined : "vibe-workspace-hint"} value={workspace} disabled={pending} onChange={event => setWorkspace(event.target.value)}><option value="">Choose a workspace</option>{workspaces.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
    {!workspaceName && <p id="vibe-workspace-hint">Choose a workspace to use its credits.</p>}
  </div> : saveAccess === "loading" && !!config && <p className="vibe-connection-status mt-3" role="status">Loading workspaces…</p>);
  const resultSources = session && artifact ? [
          <CanonicalResults key="application" selected={selectedCanonical} onRefreshSelected={retrySelectedResult} sessionId={session.id} workspaceId={session.workspace_id} actorScope={authUser?.id || "guest"} token={token} access={evidenceAccess.access} onSelectResult={onSelectResult} onExport={result=>{const pinned=session.document.artifacts.find(a=>a.id===result.binding.artifact_id);if(pinned)void exporter.startResult(pinned,result.binding.result_ref)}}/>,
          <SDKReports key="sdk" selectedReport={selectedSDKReport} sessionId={session.id} workspaceId={session.workspace_id} artifactId={artifact.id} actorScope={authUser?.id || "guest"} token={token} access={evidenceAccess.access} onSelectResult={onSelectResult} revision={session.revision} models={models} inferenceAvailable={sdkInferenceAvailable} refresh={session.event_cursor} onPromoted={onSDKPromoted} onRegraded={onSDKRegraded} onExport={report=>{const pinned=session.document.artifacts.find(a=>a.id===report.binding.artifact_id);if(pinned)void exporter.startResult(pinned,report.binding.result_ref);}} />,
  ] : [];
  if (selectedResult?.result_ref.kind === "sdk_report") resultSources.reverse();
return (
    <VibeAppearance>
 <ExportDialog controller={exporter}/>
      <Dialog open={!!runQuote && !utilitySheet} onOpenChange={open => { if (!open) setRunQuote(undefined); }}>
        <DialogContent className="vibe-workspace vibe-dialog" initialFocus={quoteHeading} finalFocus={() => {
          const trigger = quoteTrigger.current;
          return trigger?.isConnected && !trigger.matches(":disabled") ? trigger : document.getElementById("vibe-trial-message") || document.getElementById("vibe-message");
        }}><DialogTitle ref={quoteHeading} tabIndex={-1}>{runQuote?.kind === "message" ? "Run harder tests" : runQuote?.baseline ? "Run the same tests again" : "Run these tests"}</DialogTitle>
          <div className="vibe-dialog-body">
          <DialogDescription>{runQuote?.kind === "message" ? `${testCount(runQuote.extra.additional_examples || 0, "new")} + ${testCount((runQuote.quote.cases || 0) - (runQuote.extra.additional_examples || 0), "existing")}` : testCount(runQuote?.quote.cases || 0)} · up to {dollars(runQuote?.quote.max_cost_nano_usd || 0)}. Usually a few minutes; provider queues can take longer. {runQuote?.kind === "message" ? "Includes preparing, reviewing and fixing the new tests, then running all of them. Existing tests and their earlier results stay unchanged." : runQuote?.baseline ? "The same tests and grading stay fixed. Earlier results are kept." : "This run uses the selected instructions or recorded replies."}</DialogDescription>
          {runQuote?.extra.case_keys && <p>Selected tests: {runQuote.extra.case_keys.join(", ")}. A new run will retain these results separately from the original.</p>}
          {runQuote?.kind !== "message" && <p>{runQuote?.extra.purpose === "regrade" ? "Regrade: the evaluator grades the original saved replies again." : artifacts.find(item=>item.id===runQuote?.extra.artifact_id)?.conversation_evaluation ? "Recorded replies: the evaluator grades your exact saved replies. Your app and a target model are not called. The original app model and build identities are unknown." : "Generated replies: a target model writes new replies from the selected agent instructions."}</p>}
          <p>Scope: {artifacts.find(item => item.id === runQuote?.selectionArtifact)?.title || "Selected project"} · up to {runQuote?.quote.calls} model calls · {funding}.</p>
          <p>Running sends the selected instructions, materials and tests to the configured model providers and stores the replies and results.</p>
          {runQuoteState === "preparation" ? <p role="status">Preparing estimate…</p> : runQuoteState === "failure" ? <p role="alert">Estimate failed: {runQuoteError}. Nothing ran.</p> : !runQuoteCurrent && <p role="status">This estimate expired or the project changed. Refresh it before running.</p>}
          </div>
          <div className="vibe-dialog-actions">
            <VibeButton variant="quiet" className="vibe-dialog-aside" disabled={busy} onClick={() => { if (!runQuote) return; const q = runQuote; if (q.kind === "message") void requestPreparation(q.content || "", q.extra.additional_examples || 0, q.extra.artifact_id || ""); else void requestRun(q.baseline, q.extra.evidence_set_id, q.extra.purpose, q.extra.case_keys && q.extra.retry_source_result_ref && q.extra.artifact_id ? {case_keys:q.extra.case_keys,retry_source_result_ref:q.extra.retry_source_result_ref,artifact_id:q.extra.artifact_id} : undefined, q.recordedArtifactID); }}>Refresh estimate</VibeButton>
            <VibeButton onClick={() => setRunQuote(undefined)}>Cancel</VibeButton>
            <VibeButton variant="primary" disabled={busy || !(runQuote?.kind === "message" ? inferenceAvailable : runInferenceAvailable) || !runQuoteCurrent} onClick={() => { if (!runQuote || !isRunQuoteCurrent()) return; const q=runQuote; setRunQuote(undefined); void submit(q.kind,q.content || "",q.baseline,{...q.extra,...(q.extra.cycle_id ? {} : {run_quote_id:q.quote.id})}); }}>Start run</VibeButton>
          </div></DialogContent>
      </Dialog>
      <VibeConnection.Provider value={{ token, contact: config?.contact }}><EvaluationNavigation signInReturnPath={startDoor ? `/vibe-evals?start=${startDoor}` : undefined} actorScope={authUser?.id || "guest"} archiveMode={archiveMode} onArchiveMode={setArchiveMode} onMetadata={metadata} workspace={choosingWorkspace && workspaces.length > 0 ? { name: workspaceName, onChange: () => workspaceSelect.current?.focus() } : undefined} accountLabel={authLoading ? "Loading account…" : authUser ? authUser.email || "Signed in account" : "Guest · private to this browser"} listStatus={contextList.status} onRetryList={contextList.retry} enabled={twoDoor} contexts={contexts.filter(context => !discardedContextIDs.current.has(context.id))} session={session}
        choosing={newEvaluation} disabled={!config || pending || uncertain}
        onDeleted={id => {
          discardedContextIDs.current.add(id);
          setContexts(old => old.filter(item => item.id !== id));
          drafts.discard(id);
          if (sessionID === id) { select(); drafts.activate(); setNewEvaluation(true); setContent(""); setPendingMessage(undefined); setError(""); navigation.writeURL(new URL("/vibe-evals", window.location.origin)); }
        }}
        onNew={openNewEvaluation}
        onSwitch={id => { if (id === sessionID) setNewEvaluation(false); else void switchContext(id); }}
        onSettings={() => setSettingsOpen(true)} onSavedWork={() => setSavedWorkOpen(true)}>
      {navigationToggle => <EvaluationWorkspace onExport={(a,o)=>void exporter.start(a,o)} onOriginal={a=>void exporter.original(a)}
        key={session?.id || "entry"}
        startScreen={startDoor ? startDoor === "build" ? <BuildStart draft={startBuildDraft.value} onDraft={startBuildDraft.setValue} price={buildPrice} trialUsed={!authUser && !!startConfig?.guest_trial?.initial_build_used} busy={pending} error={startError} blockedReason={buildBlockedReason} onStart={text => void startProject("build", { kind: "build", text })} onSwitchDoor={() => chooseDoor("test")} onBack={backFromStart} /> : <ImproveStart mode={startInputs.mode || "instructions"} onMode={mode => setStartInputs({ mode })} drafts={{ ...startInputs, model: startInputs.model || config?.defaults.target }} onDraft={setStartInputs} models={config?.models || []} signedIn={!!authUser} busy={pending} error={startError} blockedReason={!entryAvailable ? unavailableReason : undefined} onWriteTests={input => void startProject("test", { kind: "instructions", ...input })} onSaveReplies={input => void startProject("test", { kind: "replies", ...input })} onImportPack={file => void startProject("test", { kind: "pack", file })} onOther={() => setStartInputs({ mode: "instructions" })} onSwitchDoor={() => chooseDoor("build")} onBack={backFromStart} /> : undefined}
        twoDoor={twoDoor}
        navigationToggle={navigationToggle}
        newEvaluation={newEvaluation}
        contextNavigationBlocked={!config || pending || uncertain}
        entryBlocked={!entryAvailable}
        entryNotice={<>{workspaceChoice}{!entryAvailable && (config || configError ? (configError || !(workspacePicker && !workspaceName)) && <div className="vibe-entry-availability">
          {configError ? <><p role="alert">Couldn’t connect. Try again.</p><VibeButton variant="quiet" onClick={() => setLoadAttempt(attempt => attempt + 1)}>Retry connection</VibeButton></>
            : choosingWorkspace && !workspaceName && saveAccess === "ready" ? <><p role="status">You need a workspace to start a project. Create one from your dashboard, or ask a workspace owner for access.</p><Link className="underline" href="/dashboard">Open your dashboard</Link></>
            : choosingWorkspace && !workspaceName && saveAccess === "error" ? <><p role="alert">Your workspaces couldn’t be loaded.</p><VibeButton variant="quiet" onClick={() => setLoadAttempt(attempt => attempt + 1)}>Try again</VibeButton></>
            : choosingWorkspace && !workspaceName && saveAccess === "signed_out" ? <><p role="alert">Your sign-in expired. Sign in again to choose a workspace.</p><Link className="underline" href={`/auth/login?returnTo=${encodeURIComponent("/vibe-evals")}`}>Sign in again</Link></>
            : <><p role="status">{unavailableReason}</p><VibeButton variant="quiet" onClick={() => setSettingsOpen(true)}>Review settings</VibeButton></>}
        </div> : <p className="vibe-connection-status mt-3" role="status">Connecting…</p>)}</>}
        onCancelNew={() => setNewEvaluation(false)}
        savedWorkOpen={savedWorkOpen}
        onSavedWorkOpenChange={setSavedWorkOpen}
        onDoor={door => void chooseDoor(door)}
        contextControl={activeContext && <p className="vibe-context-label" aria-label="Active project" title={`${activeContext.title} · ${activeContext.scope}`}>
          <span className="vibe-context-dot" aria-hidden="true" /><span className="vibe-context-title">{activeContext.title}</span><span>· {activeContext.scope}</span>
        </p>}
        buildStart={buildStart}
        arrival={arrival}
        takeStartFocus={takeStartFocus}
        onSample={() => sendMessage("I don’t know. Use a clearly labelled sample policy or a narrower sample demonstration.")}
        onDemo={id => sendMessage("Try a sample email assistant", undefined, id)}
        materialInput={config?.material_build && buildJourney && buildStart ? <TaskInput key={`${sessionID}:${materialScope}`} configure inputs={materials} pdfAvailable={!!config?.pdf_uploads} disabled={busy} /> : undefined}
        runInferenceUnavailable={!runInferenceAvailable}
        inferenceUnavailable={!inferenceAvailable || !session && !entryAvailable}
        sendBlocked={!inferenceAvailable || !!messageError || buildStart && (!quoteMatches || materials.blocked)}
        sendError={messageError || (buildStart ? quoteError : undefined)}
        testJourney={testJourney}
        interactionActions={config?.interaction_actions}
        onChoice={applyChoice}
        onReloadChoices={reloadChoices}
        modelLabel={
          config?.models?.find((m) => m.id === models.target)?.name ||
          models.target
        }
        session={session}
        artifact={artifact}
        view={view}
        busy={busy}
        onRetry={(id, model) => void retryOperation(id, model)}
        retryModels={config?.models}
        retryPendingOperationID={pending ? submission.current?.retryOperationID : undefined}
        retryUncertainOperationID={uncertain ? submission.current?.retryOperationID : undefined}
        checkingTestChanges={pendingEdit?.artifactID === artifact?.id && !!pendingEdit}
        pendingMessage={pendingMessage}
        requestedRunID={requestedRunID}
        connectorSettings={session && artifact && <ConnectorSetup sessionId={session.id} workspaceId={session.workspace_id || ""} artifactId={artifact.id} actorScope={authUser?.id || "guest"} token={token} access={evidenceAccess.access} onRunCreated={onSelectResult}/>}
        selectedResult={selectedResult}
        onSelectResult={onSelectResult}
        externalResults={session && artifact && <div className="vibe-sdk-results">
          {selectedResultLoading && <p role="status">Loading selected result…</p>}
          {selectedResultError && <div role="alert"><p>{selectedResultError}</p><VibeButton onClick={retrySelectedResult}>Retry selected result</VibeButton></div>}
          {evidenceAccess.access.status === "unavailable" && <div role="alert"><p>Workspace access could not be confirmed.</p><VibeButton onClick={evidenceAccess.retry}>Retry workspace access</VibeButton></div>}
          {resultSources}
          {evidenceResult&&<ObservedResults binding={evidenceResult} actorScope={authUser?.id||"guest"} workspaceId={session.workspace_id} token={token} access={evidenceAccess.access} onExport={()=>{const pinned=session.document.artifacts.find(a=>a.id===evidenceResult.artifact_id);if(pinned)void exporter.startResult(pinned,evidenceResult.result_ref,"selected",undefined,true)}}/>}
          <details><summary>Import SDK report</summary><SDKReportImportForm sessionId={session.id} workspaceId={session.workspace_id} artifactId={artifact.id} actorScope={authUser?.id || "guest"} token={token} access={evidenceAccess.access} onImported={binding=>{onSelectResult(binding);void snapshots.reload();}} /></details>
        </div>}
        selectedCase={navigation.case}
        onSelectCase={key => navigation.update({ case: key })}
        quickChecking={runPending}
        pendingLabel={
          (loadingSession && !session ? "Loading your project…" : undefined) ||
          pendingAction ||
          (pending && !active
            ? runPending
              ? "Starting your run…"
              : pendingMessage
                ? "Sending your message…"
                : "Saving your changes…"
            : undefined)
        }
        dirty={dirtyArtifact}
        content={content}
        onContent={(value) => {
          composerEdits.current++;
          setContent(value);
        }}
        onSend={context => sendMessage(content, context)}
        onTougher={(text, count, artifactID) => {
          void requestPreparation(text, count, artifactID);
        }}
        onMessage={(text, operation, instructions) => {
          navigate("build");
          void submit("message", instructions && session?.document.format_version === 1 ? `${text}\n\nAgent instructions to test:\n${instructions}` : text, undefined, {
            ...(instructions ? { instructions } : {}),
            ...(operation
              ? {
                  artifact_id:
                    operation.source?.artifact_id ||
                    operation.results[0]?.version,
                  baseline_id: operation.id,
                  purpose: "suggest_change",
                }
              : {}),
          });
        }}
        onNavigate={navigate}
        onSelectRun={run => navigation.update({ view: "results", run })}
        onRun={(baseline, evidenceID, recordedArtifactID) => void requestRun(baseline, evidenceID, undefined, undefined, recordedArtifactID)}
        recordedUpdate={recordedUpdate}
        onNewReplies={startRecordedUpdate}
        onCloseRecordedUpdate={closeRecordedUpdate}
        onReloadEvidence={() => { void snapshots.reload(); }}
        onRetryCases={(source, caseKeys) => { const id=source.source?.artifact_id || source.results[0]?.version; if(id) void requestRun(undefined,source.source?.evidence_set_id,undefined,{case_keys:caseKeys,retry_source_result_ref:{kind:"vibe_operation",id:source.id},artifact_id:id}); }}
        onRecordedEvidence={addRecordedEvidence}
        onEdit={edit}
        onDirty={(dirty) => {
          setChecksDirtyID(dirty ? artifact?.id || null : null);
          if (dirty && artifact) setSelectedArtifactID(artifact.id);
        }}
        onSave={openSave}
        onSettings={() => setSettingsOpen(true)}
        estimateOpen={!!runQuote}
        onUtilitySheet={setUtilitySheet}
        importNotice={importError && importError.sessionID === sessionID && !newEvaluation && <div ref={importNotice} tabIndex={-1} className="vibe-import-error" role="alert">
          <p>{importError.message}</p>
          <VibeButton variant="quiet" disabled={busy} onClick={() => file.current?.click()}>Choose a file</VibeButton>
        </div>}
        onImport={() => file.current?.click()}
        loadEvidence={loadSavedCase}
        onAction={operationAction}
        onRegrade={config?.grading_recheck ? operation => void requestRun(operation, undefined, "regrade") : undefined}
        onDispute={(rule, result, operation) => {
          const source = artifacts.find((a) => a.id === result.version);
          if (source) setSelectedArtifactID(source.id);
          buildEvidence.current = testJourney
            ? undefined
            : {
                operation: operation.id,
                artifact: result.version,
              };
          composerEdits.current++;
          setContent(
            `That’s not our rule: “${rule}”\n\nThe correct expectation is: `,
          );
          navigate("build", result.version);
          requestAnimationFrame(() =>
            document.getElementById("vibe-message")?.focus(),
          );
        }}
        savedChecks={savedChecks} savedListStatus={savedList.status} onRetrySaved={savedList.retry}
        notice={
          <>
            {runQuoteState === "preparation" && <p role="status">Preparing estimate…</p>}
            {runQuoteState === "failure" && <p role="alert">Estimate failed: {runQuoteError}. Nothing ran. Run the tests again for a new estimate.</p>}
            {config && !inferenceAvailable && !choosingEntry && <p role="status">{unavailableReason}</p>}
            {buildStart && !choosingEntry && <section aria-label="Build estimate"><p>Requesting an estimate sends your draft to AgentClash to calculate and store the estimate. It does not run model inference.</p><p role="status">{buildQuoteState === "failure" ? "Estimate failed" : buildQuoteState === "preparation" || !buildQuote ? content.trim() ? "Preparing estimate…" : "Describe the task to see an estimate" : `Estimate: up to ${dollars(buildQuote.max_cost_nano_usd)} · ${buildQuote.cases} tests · up to ${buildQuote.max_calls} model calls${buildQuoteState === "expired" ? " · Out of date" : ""}`} · {funding}.</p><p>Includes preparation, review, automatic fixes and the first run.</p><VibeButton disabled={busy || !content.trim() || !inferenceAvailable} onClick={refreshBuildQuote}>Refresh build estimate</VibeButton></section>}
            {!choosingEntry && !config && !configError && <p className="vibe-connection-status" role="status">Connecting…</p>}
            {!choosingEntry && configError && (
              <div className="mb-3 space-y-2 text-sm">
                <p role="alert">
                  Couldn’t connect. Try again.{content.trim() ? " Your message is still here." : ""}
                </p>
                <VibeButton
                  onClick={() => setLoadAttempt((attempt) => attempt + 1)}
                >
                  Retry connection
                </VibeButton>
              </div>
            )}
            {!drafts.storageAvailable && <p role="status">Browser storage is unavailable. Drafts stay while this page is open, but cannot be restored after a full reload.</p>}
            {savedDraft && (
              <div className="mb-3 space-y-2 text-sm vibe-muted">
                <p>
                  {saved
                    ? testJourney
                      ? "Your tests are saved. Find them in Saved work whenever you update your agent."
                      : "Your agent and tests are saved."
                    : savedModelNotice}
                </p>
                <Link
                  className="underline"
                  href={advancedURL(savedDraft.workspace_id,savedDraft.draft_id,projectReturnPath,session?projectTitle(session):undefined)}
                >
                  {testJourney
                    ? "Open tests in the advanced editor"
                    : "Open in the advanced editor"}
                </Link>
              </div>
            )}
            {workspace && !testJourney && !session?.document.evaluation_first && (
              <Button variant="ghost" size="sm" onClick={() => setCreditsOpen(true)}>Credits</Button>
            )}
            {artifact &&
              artifact.kind !== "test_plan" &&
              artifact.kind !== "conversation_evaluation" &&
              !testJourney && !session?.document.evaluation_first && (
                <VibeButton
                  variant="quiet"
                  disabled={busy || dirtyArtifact}
                  onClick={() => void openSave()}
                >
                  Save agent
                </VibeButton>
              )}
            {sessionUnavailable && !loadingSession && (
              <p role="status" className="mb-3 text-sm vibe-muted">
                This project could not be loaded.
              </p>
            )}
            {session && selectedArtifactID && !artifact && <p role="alert" className="text-sm text-builder-warn">This test version is unavailable. Choose another version in Settings.</p>}
            {authUser && snapshots.loadError?.includes("Sign in again") && <Link href={`/auth/login?returnTo=${encodeURIComponent(`/vibe-evals?${new URLSearchParams({ ...(sessionID ? { session: sessionID } : {}), view: navigation.view })}`)}`}>Sign in again</Link>}
            {(error || snapshots.loadError) && (
              <p role="alert" className="mb-3 text-sm text-builder-warn">
                {error || snapshots.loadError}
              </p>
            )}
            {error && pendingMessage && !busy && (
              <VibeButton onClick={() => retryUnsentMessage.current?.()}>
                Retry unsent message
              </VibeButton>
            )}
            {session?.operations.at(-1)?.error && !session.operations.at(-1)?.completion_receipt && !session.operations.at(-1)?.retry_of_operation_id && !session.document.messages.some(message => message.role === "user" && message.origin !== "playground" && message.operation_id === session.operations.at(-1)?.id) && (
              <p role="alert" className="mb-3 text-sm text-builder-warn">
                {session.operations.at(-1)?.error?.message}
              </p>
            )}
            {connection && (
              <p role="status" className="mb-3 text-sm vibe-muted">
                {connection}
              </p>
            )}
            {(sessionUnavailable || connection === sessionAccessLost) &&
              !loadingSession && (
                <VibeButton
                  onClick={() => {
                    setConnection("");
                    setError("");
                    setLoadAttempt((n) => n + 1);
                  }}
                >
                  Retry connection
                </VibeButton>
              )}
            {connection === sessionAccessLost &&
              (!session ||
                (!session.document.messages.length &&
                  !session.document.artifacts.length &&
                  !session.document.requirements.length &&
                  !session.operations.length)) && (
                <VibeButton
                  disabled={pending || uncertain}
                  onClick={() => {
                    select(); drafts.activate();
                    setContent(content);
                    setError("");
                    setConnection("");
                    navigation.writeURL(new URL(`/vibe-evals${workspace ? `?workspace=${encodeURIComponent(workspace)}` : ""}`, window.location.origin));
                    document.getElementById("vibe-message")?.focus();
                  }}
                >
                  Keep message in a new project
                </VibeButton>
              )}
            {uncertain && !submission.current?.retryOperationID && (
              <div className="space-y-2 text-sm vibe-muted">
                <p>
                  The submission acknowledgement was not confirmed. Retry to
                  recover its saved status. Your next message stays here.
                </p>
                <VibeButton disabled={pending} onClick={retrySubmission}>
                  Retry submission
                </VibeButton>
              </div>
            )}
            {dirtyArtifact && !testJourney && (
              <p className="mb-3 text-sm vibe-muted">
                Save or discard your edits before starting a run or sending a
                message.
              </p>
            )}
            {latestArtifact && artifact?.id !== latestArtifact.id && (
              <VibeButton
                disabled={busy || dirtyArtifact}
                onClick={() => {
                  navigate("build", latestArtifact.id, "");
                }}
              >
                Review latest version
              </VibeButton>
            )}
          </>
        }
        preview={
          <PrototypeTrial workspace={twoDoor} sessionID={session?.id} error={new TextEncoder().encode(trialText).length > messageLimit ? `Message exceeds the ${messageLimit.toLocaleString()} byte UTF-8 limit.` : undefined} inline={buildJourney} dock={buildJourney && !twoDoor} title={artifact?.title} version={session && artifact ? buildVersion(session, artifact) : undefined} busy={busy} inferenceBlocked={!inferenceAvailable || dirtyArtifact || trialMaterials.blocked} text={trialText} hasMaterial={trialMaterials.bindings.length > 0}
            materialInput={config?.material_build && buildJourney ? <TaskInput key={`${sessionID}:${materialScope}`} inputs={trialMaterials} pdfAvailable={!!config?.pdf_uploads} disabled={busy || dirtyArtifact} /> : undefined}
            onText={setTrialText} onSend={() => void submit("playground", trialText)}
            onBack={() => navigate("build")} onNew={newTrial} thread={threadID}
            history={trialHistory} messages={trialMessages}
            model={config?.models?.find(m => m.id === models.target)}
            example={evaluation?.scenarios?.[0]?.input || evaluation?.examples[0]}
            onThread={id => {
              const next = trialHistory.find(t => t.id === id);
              if (next?.model) setModels(old => ({ ...old, target: next.model! }));
              navigate(buildJourney && view !== "try" ? "build" : "try", artifact?.id, id);
            }} />
        }
      />
      }
      </EvaluationNavigation></VibeConnection.Provider>
      <AgentSettings reimport={session && artifact && artifact.kind !== "test_plan" ? <ReimportReview key={`${authUser?.id}:${session.id}:${artifact.id}`} actor={authUser?.id || "guest"} session={session} artifact={artifact} token={token} disabled={busy || dirtyArtifact} onApplied={updated => { if (accept(updated)) { const next = updated.document.artifacts.at(-1); if (next) navigate("tests", next.id, ""); setSettingsOpen(false); } }} /> : undefined} open={settingsOpen} onOpenChange={setSettingsOpen}
        onOpenChangeComplete={open => { const action = afterSettings.current; afterSettings.current = null; if (!open) action?.(); }}
        config={config} session={session} artifact={artifact} artifacts={artifacts}
        models={models} busy={busy || dirtyArtifact} workspace={workspace}
        testJourney={testJourney} projectReturnPath={projectReturnPath} savedDraft={savedDraft} onModels={changeModels}
        onVersion={id => navigate(view, id, "")}
        onSave={() => fromSettings(() => void openSave())} onCredits={() => fromSettings(() => setCreditsOpen(true))} onImport={() => file.current?.click()}
        onExportConversation={() => fromSettings(() => void exportConversation())} />
      {workspace && <CreditsDialog key={workspace} workspace={workspace} open={creditsOpen} onOpenChange={setCreditsOpen} />}
      <input
        ref={file}
        type="file"
        accept=".json,.yaml,.yml"
        className="hidden"
        aria-label="Test pack file"
        onChange={(e) => upload(e.target.files?.[0])}
      />
      <Dialog open={saveOpen} onOpenChange={closeSave}>
        <DialogContent className="vibe-workspace vibe-dialog">
          <DialogTitle>
            {saveArtifact?.kind === "test_plan" ? "Keep this brief" : testJourney
              ? "Keep these tests"
              : saveTarget
                ? "Save this result"
                : "Save agent and tests"}
          </DialogTitle>
          <DialogDescription>
            {saveArtifact?.kind === "test_plan" ? "Keep your plan for when you’re ready. No agent or results are needed." : testJourney
              ? "Save this test set so you can run it again after changing your agent."
              : saveTarget
                ? "Keep the source, expectations and this result together. Only you can reopen this result. Future runs use your workspace’s AI credits."
                : `Save these instructions and ${suiteCases(saveArtifact?.blueprint).length || "the reviewed"} tests to your workspace. Future runs use your workspace’s AI credits.`}
          </DialogDescription>
          <div className="vibe-dialog-body space-y-4">
          <dl className="space-y-2 text-sm"><dt>Project</dt><dd className="break-words">{session?projectTitle(session):"Selected project"}</dd><dt>Version and tests</dt><dd>{saveArtifact?.title} · {saveArtifact?.id.slice(0,8)} · {saveArtifact?.conversation_evaluation?`${saveArtifact.conversation_evaluation.expectations?.length||0} expectations`:saveAdvanced?"Advanced test count is reviewed in the builder":`${suiteCases(saveArtifact?.blueprint).length} tests`}</dd><dt>Baseline</dt><dd>{saveBaseline?`Run ${saveBaseline.slice(0,8)}`:"No baseline selected"}</dd><dt>Workspace and visibility</dt><dd>{workspaces.find(w=>w.id===workspace)?.name||"Choose a workspace"} · {saveTarget||saveArtifact?.kind==="test_plan"?"Private saved result or brief":"Workspace members can access the saved pack/build"}</dd></dl>
          <p>Future runs use this workspace’s AI credits. Saving does not deploy or run anything.</p>
          {advancedReceipt&&<a className="underline" href={advancedURL(advancedReceipt.workspace_id,advancedReceipt.draft_id,saveReturnPath,session?projectTitle(session):undefined)}>Open advanced editor and keep a return link</a>}
          {(error || snapshots.loadError) && (
            <p role="alert" className="text-sm text-builder-warn">
              {error || snapshots.loadError}
            </p>
          )}
          {saveAccess === "loading" ? <p role="status" className="text-sm vibe-muted">Loading your workspaces…</p>
            : saveAccess === "error" ? <Button onClick={() => setLoadAttempt(n => n + 1)}>Try again</Button>
            : saveAccess === "ready" && !workspaces.length ? <p className="text-sm">You need a workspace you can save to. <Link className="underline" href="/dashboard">Open your workspace</Link>, or ask its owner for access. Your work is still here.</p>
            : saveAccess === "ready" ? (
            <>
              <label className="text-sm">
                Workspace
                <select
                  aria-label="Save workspace"
                  disabled={busy}
                  value={workspace}
                  onChange={(e) => setWorkspace(e.target.value)}
                  className="mt-2 w-full rounded-[var(--vibe-radius-sm)] border bg-background p-3"
                >
                  {workspaces.map((w) => (
                    <option key={w.id} value={w.id}>
                      {w.name}
                    </option>
                  ))}
                </select>
              </label>
              <Button
                onClick={save}
                disabled={
                  busy ||
                  dirtyArtifact ||
                  !saveArtifact ||
                  !workspaces.some(w => w.id === workspace) ||
                  (saveAdvanced?!!advancedReceipt:saveArtifact?.kind === "test_plan" || saveTarget ? !!savedCheck : saveComplete)
                }
              >
                {savedCheck||advancedReceipt||saveComplete?"Saved":"Save and iterate"}
              </Button>
            </>
          ) : (
            <Link
              href={`/auth/login?returnTo=${encodeURIComponent(keepReturnURL(sessionID || "", saveArtifact?.id || "", workspace, saveBaseline,saveReturnPath,saveAdvanced))}`}
              onClick={rememberSave}
              className="rounded-[var(--vibe-radius-sm)] bg-primary px-4 py-3 text-center text-sm text-primary-foreground"
            >
              Sign in to save your work
            </Link>
          )}
          {savedCheck && (
            <p role="status" className="text-sm">Saved. <a className="underline" href={savedWorkURL(savedCheck)}>Open saved {savedCheck.kind === "brief" ? "brief" : "result"}</a></p>
          )}
          {!saveTarget && saveDraft && (
            <>
              {!saveComplete && (
                <p className="text-sm text-builder-fg-muted">
                  {saveNotice}
                </p>
              )}
              <a
                href={
                  testJourney
                    ? savedWorkURL(saveKept || { session_id: sessionID!, artifact_id: saveArtifact!.id, workspace_id: saveDraft.workspace_id, baseline_operation_id: saveBaseline || "" })
                    : advancedURL(saveDraft.workspace_id,saveDraft.draft_id,saveReturnPath,session?projectTitle(session):undefined)
                }
                className="text-sm underline"
              >
                {testJourney ? "Open saved tests" : "Open in the advanced editor"}
              </a>
              {testJourney && <Link className="text-sm underline" href={advancedURL(saveDraft.workspace_id,saveDraft.draft_id,saveReturnPath,session?projectTitle(session):undefined)}>Open in full pack builder</Link>}
            </>
          )}
          </div>
        </DialogContent>
      </Dialog>
    </VibeAppearance>
  );
}
