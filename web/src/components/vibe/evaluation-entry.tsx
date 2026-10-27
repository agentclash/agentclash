"use client";

import { ArrowRight, FlaskConical, Sparkles } from "lucide-react";

export function EvaluationEntry({ busy, onDoor }: { busy: boolean; onDoor?: (door: "build" | "test") => void }) {
  return <section className="vibe-welcome mb-7">
    <h1 className="vibe-entry-title">What would you like AI to handle?</h1>
    <p className="mt-4 vibe-muted">Try an idea, or find out what your existing agent gets right and wrong.</p>
    <div className="vibe-entry-doors mt-7">
      <button type="button" disabled={busy} onClick={() => onDoor?.("build")}>
        <Sparkles className="vibe-door-icon" size={20} aria-hidden="true" />
        <span>Build an agent <ArrowRight size={18} aria-hidden="true" /></span>
        <small>I have something I want AI to handle.</small>
      </button>
      <button type="button" disabled={busy} onClick={() => onDoor?.("test")}>
        <FlaskConical className="vibe-door-icon" size={20} aria-hidden="true" />
        <span>Improve an existing agent <ArrowRight size={18} aria-hidden="true" /></span>
        <small>I already have an AI agent and want to find what to fix.</small>
      </button>
    </div>
    <details className="mt-6 text-sm"><summary>Read a sample evaluation</summary><article className="mt-3 space-y-2"><h2>Sample: a return policy assistant</h2><p>Instructions: unopened purchases are eligible within 30 days. Ask for missing purchase age or condition.</p><p>Test: “I opened an item bought 10 days ago.”</p><p>Expected: explain that opened items are ineligible. A reply promising a refund would fail this expectation.</p><p>This is a read only example. No model runs when you open it.</p></article></details>
  </section>;
}
