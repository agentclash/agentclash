"use client";

import { useEffect, useState, type ReactNode } from "react";
import { motion, useReducedMotion } from "framer-motion";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent } from "@/lib/vibe-build-analytics";
import { buildTimeline, buildVersion, workingBuildArtifact, type BuildMessage } from "@/lib/vibe-build-timeline";
import { terminal, type Artifact, type CaseResult, type Operation, type Session } from "@/lib/vibe";
import { ProjectEnquiry } from "./project-enquiry";
import { TaskOutput } from "./task-output";
import { AgentReply } from "./safe-markdown";
import { VibeButton } from "./vibe-button";
import { BuildRunResult } from "./build-run-result";
import { suiteCases } from "./test-suite-cases";
import { PromptChange } from "./prompt-change";

export function BuildConversation({ session, artifact, busy, primary, onPreviewOpen, onReviewArtifact, onGuide, proposal, pending, renderMessage, onDetails, onImprove, onSave, onTougher, loadEvidence }: {
  session: Session;
  artifact?: Artifact;
  busy: boolean;
  primary: boolean;
  onPreviewOpen: (open: boolean, artifactID?: string) => void;
  onReviewArtifact: (artifactID: string) => void;
  onGuide: () => void;
  proposal: ReactNode;
  pending: ReactNode;
  renderMessage: (message: BuildMessage) => ReactNode;
  onDetails: (operation: Operation) => void;
  onImprove: (operation: Operation) => void;
  onSave: (operation: Operation) => void;
  onTougher?: (text: string, count: number, artifactID: string) => void;
  loadEvidence: (id: string, key: string) => Promise<CaseResult>;
}) {
  const entries = buildTimeline(session);
  const reduced = useReducedMotion();
  const [initialEntries] = useState(() => new Set(entries.map(e => e.id)));
  const runs = entries.filter(e => e.kind === "run");
  const working = workingBuildArtifact(session);
  const latestRun = runs.filter(e => e.artifact?.id === working?.id).at(-1)?.operation;
  const firstEvidence = runs.find(e => e.operation.results.length > 0)?.id;
  const latestTrialMessage = session.document.messages.filter(message => message.origin === "playground" && message.role === "assistant").at(-1);
  const latestTrialReply = latestTrialMessage ? `message:${latestTrialMessage.id}` : undefined;
  useEffect(() => {
    if (!latestTrialReply || !latestTrialMessage) return;
    const node = document.querySelector(`[data-build-entry="${latestTrialReply}"]`);
    if (!node) return;
    const message = latestTrialMessage;
    const report = () => captureBuildEvent(WEB_EVENTS.VIBE_BUILD_TRIAL_REPLY_VIEWED, { session_id: session.id, operation_id: message.operation_id, artifact_id: message.artifact_id, thread_id: message.preview_thread_id }, `${session.id}:${message.id}`);
    if (typeof IntersectionObserver === "undefined") { report(); return; }
    const observer = new IntersectionObserver(items => { if (items.some(item => item.isIntersecting)) report(); }, { threshold: 0.1 });
    observer.observe(node);
    return () => observer.disconnect();
  }, [latestTrialReply, latestTrialMessage, session.id]);

  function openTrial(id?: string) {
    onPreviewOpen(true, id || working?.id);
    requestAnimationFrame(() => document.getElementById("vibe-trial-message")?.focus({ preventScroll: false }));
  }
  return <div className="vibe-build-timeline" role="log" aria-label="Conversation with Vibe Evals" aria-live="polite" aria-relevant="additions">
    {entries.map(entry => {
      const entrance = { initial: reduced || initialEntries.has(entry.id) ? false as const : { opacity: 0, y: 4 }, animate: { opacity: 1, y: 0 }, transition: { duration: reduced ? 0 : .18 } };
      if (entry.kind === "message") {
        const trial = entry.message.origin === "playground";
        const target = trial ? session.document.artifacts.find(a => a.id === entry.message.artifact_id) : undefined;
        return <motion.div {...entrance} key={entry.id} data-message-id={entry.message.id} data-build-entry={entry.id}>
          {trial ? <div className={`vibe-message ${entry.message.role === "user" ? "vibe-message-user" : ""}`}>
            <p className="vibe-build-eyebrow">{entry.message.role === "user" ? "You → " : ""}{target?.title || "Saved prototype"}{target ? ` · trial v${buildVersion(session, target)}` : ""}</p>
            {entry.message.role === "assistant" ? <><TaskOutput text={entry.message.content} sessionID={session.id} materials={entry.message.materials} />{entry.id === latestTrialReply && target && <ProjectEnquiry primary key={target.id} session={session} artifact={target} operation={latestRun} />}</> : <AgentReply>{entry.message.content}</AgentReply>}
            {entry.id === latestTrialReply && working && target && working.id === target.id && <div className="vibe-build-trial-next">
              <VibeButton variant="quiet" onClick={() => { captureBuildEvent(WEB_EVENTS.VIBE_BUILD_ACTION_CLICKED, { session_id: session.id, artifact_id: working.id, action: "change" }); onGuide(); }}>Change how it works</VibeButton>

            </div>}
          </div> : renderMessage(entry.message)}
        </motion.div>;
      }
      if (entry.kind === "prototype") {
        if (entry.artifact.kind === "task_brief") return <section key={entry.id} aria-label="Unexecuted project brief" className="vibe-prototype-card"><h2>{entry.artifact.title}</h2><TaskOutput text={entry.artifact.summary || ""} /><p className="vibe-build-note">{entry.artifact.scope_note}</p><ProjectEnquiry session={session} artifact={entry.artifact} /><VibeButton variant="quiet" onClick={onGuide}>Add missing information</VibeButton></section>;
        const current = entry.artifact.id === working?.id;
        const selected = entry.artifact.id === artifact?.id;
        const pendingChange = !entry.artifact.accepted && !entry.artifact.dismissed;
        const parent = session.document.artifacts.find(a => a.id === entry.artifact.parent_id);
        const associated = runs.find(r => r.artifact?.id === entry.artifact.id);
        if (associated && terminal(associated.operation.state)) return null;
        return <motion.div {...entrance} key={entry.id} data-build-entry={entry.id}><section className="vibe-prototype-card" aria-label={entry.artifact.agent_prompt ? `Interactive prototype version ${entry.version}` : "Saved test draft"}>
          <p className="vibe-build-eyebrow">{!entry.artifact.agent_prompt ? "Saved checks · needs a prototype" : pendingChange && working ? "Proposed version" : `Interactive prototype · v${entry.version}`}{entry.artifact.sample ? " · Sample demonstration" : ""}</p>
          <h2>{entry.artifact.title}</h2>
          <p className="vibe-build-note">A first version you can try here. Your business systems aren’t connected.</p>
          {entry.artifact.sample && <p className="vibe-build-note">Sample rules are for this demonstration. Your real policy is still unspecified.</p>}
          {entry.artifact.scope_note && <details className="vibe-prototype-details"><summary>What it can do here</summary><p>{entry.artifact.scope_note}</p></details>}
          {current && <VibeButton variant="quiet" disabled={busy} onClick={() => openTrial()}>{busy ? "Try it after this check" : "Try this prototype"}</VibeButton>}

          {pendingChange && !selected && <>
            {parent?.agent_prompt && entry.artifact.agent_prompt && parent.agent_prompt !== entry.artifact.agent_prompt && <PromptChange before={parent.agent_prompt} after={entry.artifact.agent_prompt} defaultOpen />}
            <VibeButton disabled={busy} onClick={() => onReviewArtifact(entry.artifact.id)}>Review update</VibeButton>
          </>}
          {selected && !associated && <div className="vibe-build-proposal">{proposal}</div>}
        </section></motion.div>;
      }
      const current = entry.operation.id === latestRun?.id && entry.artifact?.id === working?.id;
      const firstCase = entry.artifact && suiteCases(entry.artifact.blueprint)[0];
      return <motion.div {...entrance} key={entry.id} data-build-entry={entry.id}><div>
        {entry.historical && <p className="vibe-build-eyebrow">Earlier saved check</p>}
        {terminal(entry.operation.state) ? <BuildRunResult session={session} operation={entry.operation} artifact={entry.artifact}
          current={current} first={entry.id === firstEvidence} primary={primary} busy={busy} loadEvidence={loadEvidence}
          onTry={() => openTrial(entry.artifact?.id)} onImprove={() => onImprove(entry.operation)}
          onDetails={() => onDetails(entry.operation)}
          onGuide={onGuide}
          onSave={() => onSave(entry.operation)} onTougher={onTougher} />
          : <div><p className="vibe-build-note">Checking {entry.operation.progress?.total_cases || 3} sample situations against {entry.artifact?.sample ? "the sample rules" : "your instructions"}…</p>
            {firstCase && <details className="vibe-prototype-details"><summary>See one situation we’re checking</summary>
              <p className="vibe-evidence-label">We’ll ask</p><AgentReply>{firstCase.input}</AgentReply>
              <p className="vibe-evidence-label">It should</p><AgentReply>{firstCase.expected}</AgentReply>
              <p className="vibe-build-note">Waiting for the completed check; this is the expectation, not a result.</p>
            </details>}</div>}

      </div></motion.div>;
    })}
    {pending}
  </div>;
}
