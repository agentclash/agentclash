"use client";
import { useEffect, useRef } from "react";
import Link from "next/link";
import { VibeButton } from "./vibe-button";
import { dollars } from "@/lib/vibe";
import { sendOnEnter } from "./composer-keyboard";

export function BuildStart(p: { draft: string; onDraft(text: string): void; price: { kind: "trial" } | { kind: "ceiling"; nanoUSD: number } | { kind: "unknown" }; trialUsed: boolean; busy: boolean; error?: string; blockedReason?: string; onStart(text: string): void; onSwitchDoor(): void; onBack(): void }) {
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => { heading.current?.focus(); }, []);
  const send = () => { if (!p.busy && !p.trialUsed && !p.blockedReason && p.draft.trim()) p.onStart(p.draft); };
  return <section className="vibe-column vibe-start-page" onKeyDown={e => { if (e.key === "Escape" && !p.busy) p.onBack(); }}>
    <VibeButton variant="quiet" disabled={p.busy} onClick={p.onBack}>Back</VibeButton>
    <h1 ref={heading} tabIndex={-1}>What would you like help with?</h1>
    <p>Describe the task and what a good reply should do.</p>
    {p.trialUsed ? <p>Your free build is used. <Link className="underline" href={`/auth/login?returnTo=${encodeURIComponent("/vibe-evals?start=build")}`}>Sign in to continue.</Link></p> : <form className="vibe-start-form" onSubmit={e => { e.preventDefault(); send(); }}>
      <label htmlFor="vibe-start-build">Describe the task</label><textarea id="vibe-start-build" rows={5} value={p.draft} onChange={e => p.onDraft(e.target.value)} onKeyDown={e => sendOnEnter(e, send)} />
      <p>{p.price.kind === "trial" ? "One free build" : p.price.kind === "ceiling" ? `Up to ${dollars(p.price.nanoUSD)}` : "Cost is checked when you start."}</p>
      <VibeButton type="submit" variant="primary" disabled={p.busy || !!p.blockedReason || !p.draft.trim()}>{p.busy ? "Starting…" : "Build and run 3 tests"}</VibeButton>
      {p.blockedReason && <p role="status">{p.blockedReason}</p>}{p.error && <p role="alert">{p.error}</p>}
    </form>}
    <VibeButton variant="quiet" disabled={p.busy} onClick={p.onSwitchDoor}>Already have an agent? Improve it instead.</VibeButton>
  </section>;
}
