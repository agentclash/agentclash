"use client";

import { VibeConnection } from "@/lib/vibe-connection";


import { TaskInput } from "@/components/vibe/task-input";





import { sessionAccessLost } from "@/lib/use-vibe-session";





import Link from "next/link";



import { ArchivedConversation } from "@/components/vibe/archived-conversation";
import { PrototypeTrial } from "@/components/vibe/prototype-trial";
import { EvaluationWorkspace } from "@/components/vibe/evaluation-workspace";
import { buildVersion } from "@/lib/vibe-build-timeline";
import { EvaluationNavigation } from "@/components/vibe/evaluation-navigation";
import { VibeButton } from "@/components/vibe/vibe-button";
import { CreditsDialog } from "@/components/vibe/credits-dialog";
import { AgentSettings } from "@/components/vibe/agent-settings";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";


import { vibeFetch, type CaseResult, dollars, type Session } from "@/lib/vibe";

import { keepReturnURL, savedWorkURL } from "@/lib/vibe-keep";

import { useVibeWorkspace } from "@/lib/use-vibe-workspace";

export function VibeClient() {
  const { accept, active, activeContext, adoptContext, applyChoice, artifact, artifacts, buildEvidence, buildJourney, buildStart, busy, canChangeDoor, changeDoor, changeModels, chooseDoor, closeSave, composerEdits, config, configError, connection, content, contexts, currentView, dirtyArtifact, discardedContextIDs, drafts, edit, error, evaluation, exportConversation, file, importError, importNotice, keptArtifact, latestArtifact, loadSavedCase, loadingSession, materialScope, materials, models, navigate, navigation, newEvaluation, newTrial, openNewEvaluation, openSave, operationAction, pending, pendingAction, pendingEdit, pendingMessage, quoteError, quoteHeading, quoteMatches, quoteTrigger, reloadChoices, rememberSave, requestPreparation, requestRun, requestedRunID, retryOperation, retrySubmission, retryUnsentMessage, runPending, runQuote, save, saveAccess, saveBaseline, saveOpen, saveTarget, saved, savedCheck, savedChecks, savedDraft, savedModelNotice, savedWorkOpen, scenarioCount, select, selectedArtifactID, sendMessage, session, sessionID, sessionUnavailable, setChecksDirtyID, setConnection, setContent, setContexts, setError, setLoadAttempt, setModels, setNewEvaluation, setPending, setPendingMessage, setRunQuote, setSavedWorkOpen, setSelectedArtifactID, setSettingsOpen, setThreadID, setTrialText, setWorkspace, settingsOpen, snapshots, submission, submit, switchContext, testJourney, threadID, token, trialHistory, trialMessages, trialText, twoDoor, uncertain, upload, view, workspace, workspaces } = useVibeWorkspace();
return (
    <main className="vibe-workspace dark flex h-dvh flex-col overflow-hidden font-sans">
      <Dialog open={!!runQuote} onOpenChange={open => { if (!open) setRunQuote(undefined); }}>
        <DialogContent className="vibe-workspace vibe-dialog" initialFocus={quoteHeading} finalFocus={() => {
          const trigger = quoteTrigger.current;
          return trigger?.isConnected && !trigger.matches(":disabled") ? trigger : document.getElementById("vibe-trial-message") || document.getElementById("vibe-message");
        }}><DialogTitle ref={quoteHeading} tabIndex={-1}>{runQuote?.kind === "message" ? "Try tougher situations" : runQuote?.baseline ? "Rerun the same examples" : "Try these examples"}</DialogTitle>
          <div className="vibe-dialog-body">
          <DialogDescription>{runQuote?.kind === "message" ? `${runQuote.extra.additional_examples} new situations + ${(runQuote.quote.cases || 0) - (runQuote.extra.additional_examples || 0)} existing examples` : `${runQuote?.quote.cases} examples`} · up to {dollars(runQuote?.quote.max_cost_nano_usd || 0)}. Usually a few minutes; provider queues can take longer. {runQuote?.kind === "message" ? "Includes preparation, review, bounded repairs and running this batch. Existing cases and their earlier results stay unchanged." : runQuote?.baseline ? "The same examples and grading stay fixed. Earlier results are kept." : "This run uses the selected instructions or recorded replies."}</DialogDescription>
          <VibeButton variant="primary" disabled={busy || runQuote?.sessionID !== sessionID || runQuote?.revision !== session?.revision} onClick={() => { if (!runQuote) return; const q=runQuote; setRunQuote(undefined); void submit(q.kind,q.content || "",q.baseline,{...q.extra,...(q.extra.cycle_id ? {} : {run_quote_id:q.quote.id})}); }}>{runQuote?.kind === "message" ? "Prepare and run this batch" : `Run ${runQuote?.quote.cases} examples`}</VibeButton>
          <VibeButton variant="quiet" onClick={() => setRunQuote(undefined)}>Cancel</VibeButton>
        </div></DialogContent>
      </Dialog>
      <VibeConnection.Provider value={{ token, contact: config?.contact }}><EvaluationNavigation enabled={twoDoor} contexts={contexts.filter(context => !discardedContextIDs.current.has(context.id))} session={session}
        choosing={newEvaluation} disabled={!config || pending || uncertain || dirtyArtifact}
        onDeleted={id => {
          discardedContextIDs.current.add(id);
          setContexts(old => old.filter(item => item.id !== id));
          drafts.discard(id);
          if (sessionID === id) { select(); drafts.activate(); setNewEvaluation(true); setContent(""); setPendingMessage(undefined); setError(""); navigation.writeURL(new URL("/vibe-evals", window.location.origin)); }
        }}
        onNew={openNewEvaluation}
        onSwitch={id => { if (id === sessionID) setNewEvaluation(false); else void switchContext(id); }}
        onSettings={() => setSettingsOpen(true)} onSavedWork={() => setSavedWorkOpen(true)}>
      {navigationToggle => session && session.document.format_version !== 1 && !newEvaluation && config?.two_door ? <ArchivedConversation key={session.id} session={session} navigation={navigationToggle} busy={pending} error={error || snapshots.loadError}
        onExport={exportConversation} loadEvidence={(id, key) => token().then(auth => vibeFetch<CaseResult>(`/operations/${id}/case?key=${encodeURIComponent(key)}`, auth))} onContinue={async (artifactID, clientID) => {
          setPending(true); setError("");
          try {
            const copy = await vibeFetch<Session>(`/sessions/${session.id}/continue`, await token(), { method: "POST", body: JSON.stringify({ client_id: clientID, artifact_id: artifactID }) });
            adoptContext(copy);
          } catch (e) { setError((e as Error).message); } finally { setPending(false); }
        }} /> : <EvaluationWorkspace
        key={session?.id || "entry"}
        twoDoor={twoDoor}
        navigationToggle={navigationToggle}
        newEvaluation={newEvaluation}
        contextNavigationBlocked={!config || pending || uncertain || dirtyArtifact}
        onCancelNew={() => setNewEvaluation(false)}
        canChangeDoor={canChangeDoor}
        onChangeDoor={() => void changeDoor()}
        savedWorkOpen={savedWorkOpen}
        onSavedWorkOpenChange={setSavedWorkOpen}
        onDoor={door => void chooseDoor(door)}
        contextControl={activeContext && <p className="vibe-context-label" aria-label="Active evaluation" title={`${activeContext.title} · ${activeContext.scope}`}>
          <span className="vibe-context-dot" aria-hidden="true" /><span className="vibe-context-title">{activeContext.title}</span><span>· {activeContext.scope}</span>
        </p>}
        buildStart={buildStart}
        onSample={() => sendMessage("I don’t know. Use a clearly labelled sample policy or a narrower sample demonstration.")}
        onDemo={id => sendMessage("Try a sample email assistant", undefined, id)}
        materialInput={buildJourney && buildStart ? <TaskInput key={`${sessionID}:${materialScope}`} configure inputs={materials} pdfAvailable={!!config?.pdf_uploads} disabled={busy} /> : undefined}
        sendBlocked={buildStart && (!quoteMatches || materials.blocked)}
        sendError={buildStart ? quoteError : undefined}
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
        quickChecking={runPending}
        pendingLabel={
          (loadingSession && !session ? "Loading your conversation…" : undefined) ||
          pendingAction ||
          (pending && !active
            ? runPending
              ? "Starting your check…"
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
          navigate("build");
          setSelectedArtifactID(null);
          void requestPreparation(text, count, artifactID);
        }}
        onMessage={(text, operation, instructions) => {
          navigate("build");
          void submit("message", instructions && session?.document.format_version === 1 ? `${text}\n\nAgent instructions to check:\n${instructions}` : text, undefined, {
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
        onSelectRun={run => { navigation.update({ view: "checks", run }); currentView.current = "checks"; }}
        onRun={(baseline, evidenceID) => void requestRun(baseline, evidenceID)}
        onEdit={edit}
        onDirty={(dirty) => {
          setChecksDirtyID(dirty ? artifact?.id || null : null);
          if (dirty && artifact) setSelectedArtifactID(artifact.id);
        }}
        onSave={openSave}
        onSettings={() => setSettingsOpen(true)}
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
        savedChecks={savedChecks}
        notice={
          <>
            {!config && !configError && <p className="vibe-connection-status" role="status">Connecting…</p>}
            {configError && (
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
            {savedDraft && (
              <div className="mb-3 space-y-2 text-sm vibe-muted">
                <p>
                  {saved
                    ? testJourney
                      ? "Your tests are saved. Find them in Saved work whenever you update your agent."
                      : "Your agent and checks are saved."
                    : savedModelNotice}
                </p>
                <Link
                  className="underline"
                  href={`/workspaces/${savedDraft.workspace_id}/challenge-packs/builder/${savedDraft.draft_id}`}
                >
                  {testJourney
                    ? "Open tests in the advanced editor"
                    : "Open your evaluation"}
                </Link>
              </div>
            )}
            {workspace && !testJourney && !session?.document.evaluation_first && (
              <CreditsDialog workspace={workspace} />
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
                This conversation could not be loaded.
              </p>
            )}
            {session && selectedArtifactID && !artifact && <p role="alert" className="text-sm text-builder-warn">This test version is unavailable. Open History to choose saved work.</p>}
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
                  Keep message in a new conversation
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
                Save or discard your edits before running a check or sending a
                message.
              </p>
            )}
            {latestArtifact && artifact?.id !== latestArtifact.id && (
              <VibeButton
                disabled={busy || dirtyArtifact}
                onClick={() => {
                  setSelectedArtifactID(latestArtifact.id);
                  navigate("build", latestArtifact.id, "");
                }}
              >
                Review latest version
              </VibeButton>
            )}
          </>
        }
        preview={
          <PrototypeTrial inline={buildJourney} dock={buildJourney} title={artifact?.title} version={session && artifact ? buildVersion(session, artifact) : undefined} busy={busy || dirtyArtifact || materials.blocked} text={trialText} hasMaterial={materials.bindings.length > 0}
            materialInput={buildJourney ? <TaskInput key={`${sessionID}:${materialScope}`} inputs={materials} pdfAvailable={!!config?.pdf_uploads} disabled={busy || dirtyArtifact} /> : undefined}
            onText={setTrialText} onSend={() => void submit("playground", trialText)}
            onBack={() => navigate("build")} onNew={newTrial} thread={threadID}
            history={trialHistory} messages={trialMessages}
            model={config?.models?.find(m => m.id === models.target)}
            example={evaluation?.scenarios?.[0]?.input || evaluation?.examples[0]}
            onThread={id => {
              const next = trialHistory.find(t => t.id === id);
              setThreadID(id);
              if (next?.model) setModels(old => ({ ...old, target: next.model! }));
              navigate(buildJourney && view !== "try" ? "build" : "try", artifact?.id, id);
            }} />
        }
      />
      }
      </EvaluationNavigation></VibeConnection.Provider>
      <AgentSettings open={settingsOpen} onOpenChange={setSettingsOpen}
        config={config} session={session} artifact={artifact} artifacts={artifacts}
        models={models} busy={busy || dirtyArtifact} workspace={workspace}
        testJourney={testJourney} savedDraft={savedDraft} onModels={changeModels}
        onVersion={id => {setSelectedArtifactID(id); setThreadID(""); navigate(view, id, "");}}
        onSave={() => void openSave()} onImport={() => file.current?.click()}
        onExportConversation={() => void exportConversation()} />
      <input
        ref={file}
        type="file"
        accept=".json,.yaml,.yml"
        className="hidden"
        aria-label="Evaluation file"
        onChange={(e) => upload(e.target.files?.[0])}
      />
      <Dialog open={saveOpen} onOpenChange={closeSave}>
        <DialogContent className="vibe-workspace">
          <DialogTitle>
            {artifact?.kind === "test_plan" ? "Keep this brief" : testJourney
              ? "Keep these tests"
              : saveTarget
                ? "Save this check"
                : "Save agent and checks"}
          </DialogTitle>
          <DialogDescription>
            {artifact?.kind === "test_plan" ? "Keep your plan for when you’re ready. No agent or results are needed." : testJourney
              ? "Save this test set so you can run it again after changing your agent."
              : saveTarget
                ? "Keep the source, expectations and this result together. Only you can reopen this check. Future runs use your workspace’s AI credits."
                : `Save these instructions and ${scenarioCount || "the reviewed"} checks to your workspace. Future checks use your workspace’s AI credits.`}
          </DialogDescription>
          {(error || snapshots.loadError) && (
            <p role="alert" className="text-xs text-builder-warn">
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
                  value={workspace}
                  onChange={(e) => setWorkspace(e.target.value)}
                  className="mt-2 w-full rounded-lg border bg-background p-3"
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
                  !artifact ||
                  !workspaces.some(w => w.id === workspace) ||
                  (artifact?.kind === "test_plan" || saveTarget ? !!savedCheck : saved)
                }
              >
                {artifact?.kind === "test_plan" ? (savedCheck ? "Saved" : "Keep this brief") : saveTarget
                  ? savedCheck
                    ? "Saved"
                    : "Save this check"
                  : saved
                    ? "Saved"
                    : testJourney
                      ? "Keep these tests"
                      : "Save agent and checks"}
              </Button>
            </>
          ) : (
            <Link
              href={`/auth/login?returnTo=${encodeURIComponent(keepReturnURL(sessionID || "", artifact?.id || "", workspace, saveBaseline))}`}
              onClick={rememberSave}
              className="rounded-lg bg-primary px-4 py-3 text-center text-sm text-primary-foreground"
            >
              Sign in to save your work
            </Link>
          )}
          {savedCheck && (
            <p role="status" className="text-sm">Saved. <a className="underline" href={savedWorkURL(savedCheck)}>Open saved {savedCheck.kind === "brief" ? "brief" : "check"}</a></p>
          )}
          {!saveTarget && savedDraft && (
            <>
              {!saved && (
                <p className="text-xs text-builder-fg-muted">
                  {savedModelNotice}
                </p>
              )}
              <a
                href={
                  testJourney
                    ? savedWorkURL(keptArtifact || { session_id: sessionID!, artifact_id: artifact!.id, workspace_id: savedDraft.workspace_id, baseline_operation_id: saveBaseline || "" })
                    : `/workspaces/${savedDraft.workspace_id}/challenge-packs/builder/${savedDraft.draft_id}`
                }
                className="text-sm underline"
              >
                {testJourney ? "Open saved tests" : "Open your evaluation"}
              </a>
              {testJourney && <Link className="text-sm underline" href={`/workspaces/${savedDraft.workspace_id}/challenge-packs/builder/${savedDraft.draft_id}`}>Open in full pack builder</Link>}
            </>
          )}
        </DialogContent>
      </Dialog>
    </main>
  );
}