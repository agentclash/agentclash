"use client";

import { useReducedMotion } from "framer-motion";
import { Square } from "lucide-react";
import { useState, useSyncExternalStore } from "react";
import type { Operation } from "@/lib/vibe";
import { testCount } from "@/lib/vibe-presentation";
import { VibeButton } from "./vibe-button";

export function conversationActivity(operation: Operation, testJourney?: boolean, withCount = true) {
  const origin = operation.source?.comparison === "regraded" ? "regraded" : operation.evidence_metadata?.origin ||
    (operation.source?.kind === "provided_conversations" ? "recorded" : operation.source?.kind === "prompt" ? "prototype" : undefined);
  const sourceLabel = origin === "prototype" ? "Generated replies" : origin === "recorded" ? "Recorded app replies" : origin === "regraded" ? "Saved replies · Regrade" : "Source unverified";
  const describe = (label: string) => ["check", "retest", "playground"].includes(operation.kind) ? `${sourceLabel} · ${label}` : label;
  const grading = origin === "prototype" ? "Grading the generated reply…" : origin === "recorded" ? "Grading the recorded replies…" : origin === "regraded" ? "Regrading the saved replies…" : "Grading the reply…";
  if (operation.state === "AWAITING_APPROVAL") return describe("Review the cost before continuing.");
  if (operation.state === "CANCELLING") return describe("Stopping…");
  if (operation.state === "QUEUED") return describe("Waiting to start…");
  if (operation.state === "AWAITING_INPUT") return describe("One detail is needed to continue.");
  if (operation.state === "FINALIZING") return describe("Saving your results…");
  const progress = operation.progress;
  if (progress && operation.state === "RUNNING") {
    const count = withCount && progress.total_cases > 0 ? `${progress.completed_cases} of ${progress.total_cases} results saved · ` : "";
    const label: Record<string, string> = {
      understanding: "Understanding your request…",
      advisory_understanding: "Understanding your request…",
      preparing: "Preparing your next step…",
      reviewing: "Reviewing the tests against your rules…",
      repairing: "Adjusting the tests to match your rules…",
      switching_assistant: "Trying another model…",
      running_agent: origin === "prototype" ? "Generating a text reply from these instructions…" : origin === "recorded" || origin === "regraded" ? grading : "Processing the reply…",
      grading,
    };
    if (label[progress.phase]) return describe(count + label[progress.phase]);
  }
  if (operation.kind === "message" || operation.kind === "build") {
    if (!testJourney) return "Preparing your next step…";
    const intent = operation.conversation_decision?.intent;
    if (intent === "prepare_tests" || intent === "edit_tests") return "Preparing your tests…";
    if (intent === "suggest_fix") return "Preparing a suggested fix…";
    if (intent === "explain_results") return "Reading your results…";
    return "Thinking…";
  }
  if (operation.scorecard && operation.scorecard.evaluated > 0)
    return describe(`${operation.scorecard.evaluated} of ${testCount(operation.scorecard.total)} graded`);
  return describe("Running your tests…");
}

let fallbackPaused = false;
const preferenceEvent = "vibe-status-motion-preference";
function subscribePreference(notify: () => void) {
  window.addEventListener("storage", notify);
  window.addEventListener(preferenceEvent, notify);
  return () => {
    window.removeEventListener("storage", notify);
    window.removeEventListener(preferenceEvent, notify);
  };
}
function readPreference() {
  try { return localStorage.getItem("vibe-status-motion-paused") === "true"; } catch { return fallbackPaused; }
}
const serverPreference = () => false;

// Mounted before a response starts. Only the status text enters the live region;
// the animated duplicate is decorative and never announces animation frames.
export function ActivityStatus({ label, working }: { label?: string; working: boolean }) {
  const reduced = useReducedMotion();
  const motionPaused = useSyncExternalStore(subscribePreference, readPreference, serverPreference);
  return (
    <div className="vibe-status" data-visible={!!label}>
      <p role="status" aria-live="polite" aria-atomic="true" className="vibe-activity-label">
        <span key={label || "idle"} className="vibe-status-text">
          {label || ""}
          {!!label && working && !reduced && !motionPaused && (
            <span aria-hidden="true" className="vibe-status-dots">{label}</span>
          )}
        </span>
      </p>
      {!!label && working && !reduced && (
        <button type="button" className="vibe-motion-toggle" aria-label={motionPaused ? "Resume status animation" : "Pause status animation"} onClick={() => {
          const next = !motionPaused;
          fallbackPaused = next;
          try { localStorage.setItem("vibe-status-motion-paused", String(next)); } catch { /* animation remains usable */ }
          window.dispatchEvent(new Event(preferenceEvent));
        }}>
          {motionPaused ? "Resume animation" : "Pause animation"}
        </button>
      )}
    </div>
  );
}

const runKinds = new Set(["check", "retest"]);
// Before approval or start nothing has run yet, so the note about Stop does not apply.
const runningStates = new Set(["RUNNING", "FINALIZING", "CANCELLING"]);
// The first snapshot of a run fixes its mode: a run with a known total keeps a
// count and a filling bar, a run without one keeps the indeterminate bar.
export function useRunTotal(operation?: Operation) {
  const current = operation && runKinds.has(operation.kind) ? operation.progress?.total_cases || 0 : 0;
  const [run, setRun] = useState<{ id?: string; total: number }>({ total: 0 });
  if (run.id !== operation?.id) setRun({ id: operation?.id, total: current });
  else if (run.total > 0 && current > 0 && current !== run.total) setRun({ id: run.id, total: current });
  return run.id === operation?.id ? run.total : current;
}

// The only progress, Stop and Approve controls for a project. Stay mounted while
// idle so the live region exists before the first status arrives.
export function ProgressStrip({ operation, total, pendingLabel, testJourney, onAction }: {
  operation?: Operation; total: number; pendingLabel?: string; testJourney?: boolean;
  onAction: (id: string, action: "stop" | "approve") => void;
}) {
  const completed = Math.min(operation?.progress?.completed_cases || 0, total);
  const counting = !pendingLabel && operation?.state === "RUNNING" && total > 0;
  // completed_cases leaves out errored cases, so name a count, never the test that is running.
  const label = pendingLabel || (operation ? counting ? `${completed} of ${testCount(total)} finished` : conversationActivity(operation, testJourney) : undefined);
  const detail = counting ? conversationActivity(operation!, testJourney, false) : undefined;
  const motionPaused = useSyncExternalStore(subscribePreference, readPreference, serverPreference);
  const value = total > 0 ? { "aria-valuemin": 0, "aria-valuemax": total, "aria-valuenow": completed, "aria-valuetext": `${completed} of ${testCount(total)} done` } : {};
  return <div className="vibe-progress-strip" data-active={!!label || undefined}>
    <div className="vibe-progress-row">
      <div className="vibe-progress-copy">
        <ActivityStatus label={label} working={operation?.state === "RUNNING" && !pendingLabel} />
        {detail && <p className="vibe-progress-detail">{detail}</p>}
      </div>
      {operation && <div className="vibe-progress-actions">
        {operation.state === "AWAITING_APPROVAL" && <VibeButton variant="primary" disabled={!!pendingLabel} onClick={() => onAction(operation.id, "approve")}>Approve run</VibeButton>}
        <VibeButton disabled={!!pendingLabel || operation.state === "CANCELLING"} onClick={() => onAction(operation.id, "stop")}><Square aria-hidden="true" className="vibe-stop-icon" fill="currentColor" />Stop</VibeButton>
      </div>}
    </div>
    {label && <div className="vibe-progress-bar" role="progressbar" aria-label="Progress" data-indeterminate={total > 0 ? undefined : true} data-paused={motionPaused || undefined} {...value}>
      <span style={total > 0 ? { inlineSize: `${(completed / total) * 100}%` } : undefined} />
    </div>}
    {operation && runKinds.has(operation.kind) && runningStates.has(operation.state) && <p className="vibe-progress-note">Stop keeps finished results. Provider costs can stay unknown until billing is settled.</p>}
  </div>;
}
