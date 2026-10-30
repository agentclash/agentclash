"use client";

import { useEffect, useRef, useState } from "react";
import { dollars } from "@/lib/vibe";
import { VibeButton } from "./vibe-button";
import { sendOnEnter } from "./composer-keyboard";
import { VibeDataUseNotice } from "./vibe-data-use";
import "./build-output.css";

const examples = [
  { label: "Answer return questions", text: "Answer customer questions about returns. Ask for the purchase date and allow returns within 30 days." },
  { label: "Sort support emails", text: "Sort support emails by topic and urgency. Flag billing issues and requests that need a person." },
];

export function BuildStart(p: { draft: string; onDraft(text: string): void; price: { kind: "trial" } | { kind: "ceiling"; nanoUSD: number } | { kind: "unknown" }; trialUsed: boolean; busy: boolean; error?: string; blockedReason?: string; onStart(text: string): void; onSwitchDoor(): void; onBack(): void }) {
  const input = useRef<HTMLTextAreaElement>(null);
  const file = useRef<HTMLInputElement>(null);
  const [fileError, setFileError] = useState("");
  const [reading, setReading] = useState(false);
  const reason = p.trialUsed ? "Your free build is used. Sign in to continue." : p.blockedReason || (p.draft.trim().length < 20 ? "Describe the task in a sentence or two." : "");
  const unavailable = p.busy || reading;
  const send = () => { if (!reason && !unavailable) p.onStart(p.draft.trim()); };
  useEffect(() => { input.current?.focus(); }, []);
  return <section className="vibe-build-start vibe-start-page" onKeyDown={event => { if (event.key === "Escape" && !unavailable) p.onBack(); }}>
    <header><VibeButton variant="quiet" disabled={unavailable} onClick={p.onBack}>Back</VibeButton></header>
    <h1 id="vibe-build-question" className="vibe-empty-title">What task should the agent handle?</h1>
    <p id="vibe-build-helper">A few sentences: what comes in, what a good reply does, any rules.</p>
    {p.trialUsed && <p className="vibe-build-trial-used">Your free build is used. <a href="/auth/login?returnTo=%2Fvibe-evals%3Fstart%3Dbuild">Sign in to continue.</a></p>}
    <form className="vibe-build-start-card" onSubmit={event => { event.preventDefault(); send(); }}>
      <textarea id="vibe-start-build" ref={input} disabled={reading} aria-labelledby="vibe-build-question" aria-describedby="vibe-build-helper" className="vibe-textarea" rows={4} value={p.draft} onChange={event => p.onDraft(event.target.value)} onKeyDown={event => sendOnEnter(event, send)} placeholder="Help customers check whether they can return an item." />
      <div className="vibe-build-examples" aria-label="Example tasks">{examples.map(example => <VibeButton key={example.label} disabled={unavailable} onClick={() => { p.onDraft(example.text); input.current?.focus(); }}>{example.label}</VibeButton>)}</div>
      <div className="vibe-build-start-actions">
        <VibeButton variant="quiet" disabled={unavailable} onClick={() => file.current?.click()}>Attach a file</VibeButton>
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
        <div className="vibe-build-start-submit">
          <VibeButton type="submit" variant="primary" loading={p.busy || reading} disabled={!!reason || unavailable} aria-describedby={reason ? "vibe-build-reason" : undefined}>Build and run 3 tests</VibeButton>
          <span className="vibe-build-price">{p.price.kind === "trial" ? "Free trial" : p.price.kind === "ceiling" ? `Up to ${dollars(p.price.nanoUSD)}` : "Price available when you start"}</span>
        </div>
      </div>
      {reason && !p.trialUsed && <p id="vibe-build-reason" className="vibe-build-note">{reason}</p>}
      {p.trialUsed && <span id="vibe-build-reason" className="sr-only">{reason}</span>}
      {(p.error || fileError) && <p role="alert">{p.error || fileError}</p>}
    </form>
    <VibeDataUseNotice />
    <VibeButton variant="quiet" disabled={unavailable} onClick={p.onSwitchDoor}>Already have an agent? Improve it instead.</VibeButton>
  </section>;
}
