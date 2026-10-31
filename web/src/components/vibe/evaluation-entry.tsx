"use client";

import { ArrowRight, Check, X } from "lucide-react";
import { useState, type ReactNode } from "react";
import { ClashMark } from "@/components/marketing/clash-mark";
import { Sheet, SheetContent, SheetDescription, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { VibeButton } from "./vibe-button";

let introduced = false;
export function EvaluationEntry({ busy, onDoor, notice }: { busy: boolean; onDoor?: (door: "build" | "test") => void; notice?: ReactNode }) {
  const [fresh] = useState(() => !introduced);
  return <section className="vibe-welcome vibe-entry-intro" data-fresh={fresh || undefined} ref={node => { if (node) introduced = true; }}>
    <div className="vibe-entry-mark" aria-hidden="true"><ClashMark tone="current" /></div>
    <p className="vibe-entry-eyebrow">VIBE EVALS</p>
    <h1 className="vibe-entry-title">What would you like to evaluate?</h1>
    <p className="vibe-entry-subtitle">Build an agent to try an idea, or improve one you already have.</p>
    {notice}
    <div className="vibe-entry-doors">
      <button id="vibe-door-build" aria-labelledby="vibe-door-build-title" aria-describedby="vibe-door-build-outcome vibe-door-build-next" type="button" disabled={busy} onClick={() => onDoor?.("build")}>
        <small className="vibe-door-eyebrow" aria-hidden="true">START FROM AN IDEA</small><span id="vibe-door-build-title" className="vibe-door-title">Build an agent</span>
        <p id="vibe-door-build-outcome" aria-hidden="true">Turn a task into an agent and three tests.</p>
        <ol className="vibe-door-preview vibe-door-steps" aria-hidden="true"><li><b>01</b>Describe the task</li><li><b>02</b>We draft the agent</li><li><b>03</b>Three tests run</li></ol>
        <span id="vibe-door-build-next" className="vibe-door-next" aria-hidden="true">Next: Describe the task <ArrowRight aria-hidden="true" size={18} /></span>
      </button>
      <button id="vibe-door-test" aria-labelledby="vibe-door-test-title" aria-describedby="vibe-door-test-outcome vibe-door-test-next" type="button" disabled={busy} onClick={() => onDoor?.("test")}>
        <small className="vibe-door-eyebrow" aria-hidden="true">START FROM AN AGENT</small><span id="vibe-door-test-title" className="vibe-door-title">Improve an existing agent</span>
        <p id="vibe-door-test-outcome" aria-hidden="true">Find what fails and what to fix.</p>
        <div className="vibe-door-preview" aria-hidden="true"><div className="vibe-door-issue"><X size={16} aria-hidden="true" />Issue found</div><div className="vibe-door-pass"><Check size={16} aria-hidden="true" />Passed</div></div>
        <span id="vibe-door-test-next" className="vibe-door-next" aria-hidden="true">Next: Add your agent <ArrowRight aria-hidden="true" size={18} /></span>
      </button>
    </div>
    <Sheet><SheetTrigger render={<VibeButton className="vibe-entry-sample" variant="quiet" />}>Read a sample test</SheetTrigger><SheetContent className="vibe-workspace vibe-sample-sheet"><SheetTitle>Sample: a return policy assistant</SheetTitle><SheetDescription>See what one test checks.</SheetDescription><article className="space-y-4"><p>Instructions: unopened purchases are eligible within 30 days. Ask for missing purchase age or condition.</p><p>Test: “I opened an item bought 10 days ago.”</p><p>Expected: explain that opened items are ineligible. A reply promising a refund would fail this expectation.</p><p>No model runs when you open this sample.</p></article></SheetContent></Sheet>
  </section>;
}
