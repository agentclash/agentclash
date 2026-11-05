"use client";

import { useEffect, useId, useRef, useState } from "react";
import { ChevronLeft, Paperclip } from "lucide-react";
import { dollars } from "@/lib/vibe";
import { VibeButton } from "./vibe-button";
import { sendOnEnter } from "./composer-keyboard";
import { VibeDataUseNotice } from "./vibe-data-use";
import { useVibeBrowserReady } from "@/lib/use-vibe-browser-ready";
import "./build-output.css";

const examples = [
  { label: "Answer return questions", text: "Answer customer questions about returns. Ask for the purchase date and allow returns within 30 days." },
  { label: "Sort support emails", text: "Sort support emails by topic and urgency. Flag billing issues and requests that need a person." },
];

export function BuildStart(p: { draft: string; onDraft(text: string): void; price: { kind: "trial" } | { kind: "ceiling"; nanoUSD: number } | { kind: "unknown" }; trialUsed: boolean; busy: boolean; error?: string; blockedReason?: string; onStart(text: string): void; onSwitchDoor(): void; onBack(): void }) {
  const ready = useVibeBrowserReady();
  const id = useId();
  const input = useRef<HTMLTextAreaElement>(null);
  const file = useRef<HTMLInputElement>(null);
  const [fileError, setFileError] = useState("");
  const [reading, setReading] = useState(false);
  const reason = p.trialUsed ? "Your free build is used. Sign in to continue." : p.blockedReason || (p.draft.trim().length < 20 ? "Describe the task in a sentence or two." : "");
  const unavailable = p.busy || reading;
  const send = () => { if (!reason && !unavailable) p.onStart(p.draft.trim()); };
  useEffect(() => { if (ready) input.current?.focus(); }, [ready]);
  return <section className="vibe-build-start vibe-start-page vibe-start-layout" onKeyDown={event => { if (event.key === "Escape" && !unavailable) p.onBack(); }}>
    <header><VibeButton variant="quiet" className="vibe-start-back" disabled={unavailable} onClick={p.onBack}><ChevronLeft aria-hidden="true" />All projects</VibeButton></header>
    <div className="vibe-build-intro vibe-start-intro">
      <p className="vibe-start-eyebrow">Build an agent</p>
      <h1 className="vibe-start-title"><label htmlFor={`${id}-task`}>What task should the agent handle?</label></h1>
      <p id={`${id}-helper`}>A few sentences: what comes in, what a good reply does, and any rules.</p>
    </div>
    {p.trialUsed && <p className="vibe-build-trial-used">Your free build is used. <a href="/auth/login?returnTo=%2Fvibe-evals%3Fstart%3Dbuild">Sign in to continue.</a></p>}
    <form className="vibe-build-start-form" onSubmit={event => { event.preventDefault(); send(); }}>
      <div className="vibe-build-start-card">
        <textarea id={`${id}-task`} ref={input} disabled={reading} aria-describedby={`${id}-helper${p.error ? ` ${id}-error` : ""}`} className="vibe-textarea" rows={4} value={p.draft} onChange={event => p.onDraft(event.target.value)} onKeyDown={event => sendOnEnter(event, send)} placeholder="Help customers check whether they can return an item." />
        <div className="vibe-build-example-chips" role="group" aria-label="Example tasks"><span>Try</span>{examples.map(example => <VibeButton variant="quiet" className="vibe-example-chip" key={example.label} disabled={unavailable} onClick={() => { p.onDraft(example.text); input.current?.focus(); }}>{example.label}</VibeButton>)}<VibeButton variant="quiet" className="vibe-build-attach" disabled={unavailable} aria-describedby={fileError ? `${id}-file-error` : undefined} onClick={() => file.current?.click()}><Paperclip aria-hidden="true" />Attach a file</VibeButton></div>
        {fileError && <p id={`${id}-file-error`} role="alert">{fileError}</p>}
      </div>
      <div className="vibe-build-start-actions">
        <div className="vibe-build-start-submit">
          <VibeButton type="submit" variant="primary" loading={p.busy || reading} disabled={!!reason || unavailable} aria-describedby={reason ? `${id}-reason` : undefined}>Build and run 3 tests</VibeButton>
          <div className="vibe-build-price-block"><span className="vibe-build-price" data-kind={p.price.kind}>{p.price.kind === "trial" ? "Free trial" : p.price.kind === "ceiling" ? `Up to ${dollars(p.price.nanoUSD)}` : "Price available when you start"}</span>
          {p.price.kind !== "unknown" && <span className="vibe-build-price-detail">{p.price.kind === "trial" ? "Your first build is free" : "Maximum for this build"}</span>}</div>
        </div>
        <span className="vibe-build-key-hint">Enter to start · Shift+Enter for a new line</span>
      </div>
      {reason && !p.trialUsed && <p id={`${id}-reason`} className="vibe-build-note" role={p.blockedReason ? "status" : undefined}>{reason}</p>}
      {p.trialUsed && <span id={`${id}-reason`} className="sr-only">{reason}</span>}
      {p.error && <p id={`${id}-error`} role="alert">{p.error}</p>}
    </form>
    <input ref={file} type="file" hidden accept=".txt,.md,.csv,.json,text/plain,text/markdown,text/csv,application/json" aria-label="Attach a task file" onChange={async event => {
      const selected = event.target.files?.[0]; event.target.value = ""; if (!selected) return;
      if (selected.size > 100_000 || !/\.(txt|md|csv|json)$/i.test(selected.name)) { setFileError("Choose a text file under 100 KB (.txt, .md, .csv or .json)."); return; }
      setReading(true); setFileError("");
      try {
        const bytes = await selected.arrayBuffer();
        let text: string;
        try { text = new TextDecoder("utf-8", { fatal: true }).decode(bytes); }
        catch { setFileError("This file is not UTF-8 text. Save it as UTF-8 and try again."); return; }
        p.onDraft([p.draft.trim(), text.trim()].filter(Boolean).join("\n\n")); input.current?.focus(); }
      catch { setFileError("Couldn’t read this file. Try another file."); }
      finally { setReading(false); }
    }} />
    <section className="vibe-build-next" aria-labelledby={`${id}-next`}>
      <h2 id={`${id}-next`}>What happens next</h2>
      <ol className="vibe-build-next-steps" role="list">
        <li><span className="vibe-build-step-number" aria-hidden="true">01</span><h3>We draft the agent</h3><p>Instructions written from your task. You can edit them later.</p></li>
        <li><span className="vibe-build-step-number" aria-hidden="true">02</span><h3>We write three tests</h3><p>Real messages a customer could send, each with a rule to meet.</p></li>
        <li><span className="vibe-build-step-number" aria-hidden="true">03</span><h3>The tests run</h3><p>You see each reply, what passed and what to fix.</p></li>
      </ol>
    </section>
    <footer className="vibe-start-footer">
      <VibeDataUseNotice />
      <VibeButton variant="quiet" className="vibe-start-switch" disabled={unavailable} onClick={p.onSwitchDoor}>Have an agent already? Improve it</VibeButton>
    </footer>
  </section>;
}
