"use client";

import { ArrowRight, BriefcaseBusiness, FlaskConical, Lightbulb } from "lucide-react";
import { useState, type ReactNode } from "react";
import { Sheet, SheetContent, SheetDescription, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { useVibeConnection } from "@/lib/vibe-connection";
import { VIBE_CONTACT_EMAIL } from "@/lib/vibe-contact";
import { TalkToUs } from "./talk-to-us";
import { VibeButton } from "./vibe-button";

export type RecentProject = { title: string; detail: string; tone: "issue" | "pass" | "running" | "idle"; onOpen: () => void };

// Three paths, one for each kind of visitor, in the visitor's own words: a developer with an
// agent, anyone with an idea, and a team that wants AgentClash to set up agents for them.
// Prices, times and steps belong to the screen of each path, not to this first screen.
const paths = [
  { id: "agent", door: "test", Icon: FlaskConical, title: "I have an agent", text: "Find out where it fails, with real tests you can read.", action: "Test my agent" },
  { id: "idea", door: "build", Icon: Lightbulb, title: "I have an idea", text: "Describe a task. We draft an agent and test it for you.", action: "Build from an idea" },
] as const;

let introduced = false;
// "busy" blocks new projects; "navigationBlocked" blocks only a switch to saved work.
export function EvaluationEntry({ busy, navigationBlocked = busy, onDoor, notice, recent }: { busy: boolean; navigationBlocked?: boolean; onDoor?: (door: "build" | "test") => void; notice?: ReactNode; recent?: RecentProject }) {
  const [fresh] = useState(() => !introduced);
  const [talking, setTalking] = useState(false);
  const email = useVibeConnection().contact?.email || VIBE_CONTACT_EMAIL;
  return <section className="vibe-welcome vibe-entry-intro" data-returning={recent ? "" : undefined} data-fresh={fresh || undefined} ref={node => { if (node) introduced = true; }}>
    {recent ? <>
      <h1 className="vibe-entry-title">Welcome back.</h1>
      {notice}
      <div className="vibe-entry-continue">
        <span className="vibe-entry-continue-dot" data-tone={recent.tone} aria-hidden="true" />
        <span className="vibe-entry-continue-copy"><strong id="vibe-entry-recent">{recent.title}</strong><span id="vibe-entry-recent-detail">{recent.detail}</span></span>
        <VibeButton variant="primary" disabled={navigationBlocked} aria-describedby="vibe-entry-recent vibe-entry-recent-detail" onClick={recent.onOpen}>Continue</VibeButton>
      </div>
      <h2 className="vibe-entry-section">Start something new</h2>
    </> : <>
      <h1 className="vibe-entry-title">AI agents you can trust with real work.</h1>
      <p className="vibe-entry-subtitle">We test every agent on real cases before it reaches your customers.</p>
      {notice}
    </>}
    <div className="vibe-entry-paths" role="group" aria-label="Where to start">
      {/* The ids stay "vibe-door-*": focus returns to them after a deletion and after Back. */}
      {paths.map(({ id, door, Icon, title, text, action }) => <button key={id} id={`vibe-door-${door}`} type="button" className="vibe-entry-path" aria-labelledby={`vibe-door-${door}-title`} aria-describedby={`vibe-door-${door}-outcome`} disabled={busy} onClick={() => onDoor?.(door)}>
        <span className="vibe-entry-path-icon" aria-hidden="true"><Icon size={20} /></span>
        <span id={`vibe-door-${door}-title`} className="vibe-entry-path-title">{title}</span>
        <span id={`vibe-door-${door}-outcome`} className="vibe-entry-path-text" aria-hidden="true">{text}</span>
        <span className="vibe-entry-path-action" aria-hidden="true">{action}<ArrowRight size={16} /></span>
      </button>)}
      {/* Contact stays available regardless of the model connection state. */}
      <button id="vibe-path-work" type="button" className="vibe-entry-path" data-tone="brass" aria-labelledby="vibe-path-work-title" aria-describedby="vibe-path-work-text vibe-path-work-email" onClick={() => setTalking(true)}>
        <span className="vibe-entry-path-icon" aria-hidden="true"><BriefcaseBusiness size={20} /></span>
        <span id="vibe-path-work-title" className="vibe-entry-path-title">I have work to automate</span>
        <span id="vibe-path-work-text" className="vibe-entry-path-text" aria-hidden="true">We set up agents for your team and keep them reliable.</span>
        <span className="vibe-entry-path-action" aria-hidden="true">Talk to us<ArrowRight size={16} /></span>
        <span id="vibe-path-work-email" className="vibe-entry-path-email" aria-hidden="true">or email {email}</span>
      </button>
    </div>
    <div className="vibe-entry-footer">
      <Sheet><SheetTrigger render={<VibeButton className="vibe-entry-sample" variant="quiet" />}>See a sample result</SheetTrigger><SheetContent className="vibe-workspace vibe-sample-sheet"><SheetTitle>Sample: a return policy assistant</SheetTitle><SheetDescription>See what one test checks.</SheetDescription><article className="space-y-4"><p>Instructions: unopened purchases are eligible within 30 days. Ask for missing purchase age or condition.</p><p>Test: “I opened an item bought 10 days ago.”</p><p>Expected: explain that opened items are ineligible. A reply promising a refund would fail this expectation.</p><p>No model runs when you open this sample.</p></article></SheetContent></Sheet>
      <p className="vibe-entry-cli">Run evals from your terminal <code>npx agentclash</code></p>
    </div>
    <TalkToUs open={talking} onOpenChange={setTalking} email={email} />
  </section>;
}
