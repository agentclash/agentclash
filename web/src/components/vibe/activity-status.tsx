"use client";

import { useReducedMotion } from "framer-motion";
import { useSyncExternalStore } from "react";
import type { Operation } from "@/lib/vibe";

export function conversationActivity(operation: Operation, testJourney?: boolean) {
  if (operation.state === "AWAITING_APPROVAL") return "Review the cost before continuing.";
  if (operation.state === "CANCELLING") return "Stopping…";
  if (operation.state === "QUEUED") return "Waiting to start…";
  if (operation.kind === "message" || operation.kind === "build") {
    if (!testJourney) return "Preparing your next step…";
    const intent = operation.conversation_decision?.intent;
    if (intent === "prepare_tests" || intent === "edit_tests") return "Preparing your tests…";
    if (intent === "suggest_fix") return "Preparing a suggested fix…";
    if (intent === "explain_results") return "Reading your results…";
    return "Thinking…";
  }
  if (operation.scorecard && operation.scorecard.evaluated > 0)
    return `${operation.scorecard.evaluated} of ${operation.scorecard.total} tests checked`;
  return "Running your tests…";
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
            <span aria-hidden="true" className="vibe-status-shine">{label}</span>
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
