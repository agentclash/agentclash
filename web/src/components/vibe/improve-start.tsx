"use client";
import { useEffect, useId, useRef } from "react";
import { FileUp } from "lucide-react";
import { VibeButton } from "./vibe-button";
import { VibeDataUseNotice } from "./vibe-data-use";
import { exampleAgentDescription, InstructionsHelp, ReplyCount, repliesPlaceholder } from "./improve-inputs";
import "./improve.css";

export function ImproveStart(p: {
  mode: "instructions" | "replies" | "pack"; onMode(mode: "instructions" | "replies" | "pack"): void;
  drafts: { instructions: string; model?: string; conversations: string; rules: string };
  onDraft(patch: Partial<{ instructions: string; model: string; conversations: string; rules: string }>): void;
  models: { id: string; name: string }[]; signedIn: boolean; busy: boolean; error?: string; blockedReason?: string;
  onWriteTests(input: { text: string; model?: string }): void; onSaveReplies(input: { conversations: string; rules: string }): void;
  onImportPack(file: File): void; onOther(kind: "chat" | "live_app" | "sdk_report"): void; onSwitchDoor(): void; onBack(): void;
}) {
  const id = useId();
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => { heading.current?.focus(); }, []);
  const file = useRef<HTMLInputElement>(null), radios = useRef<(HTMLButtonElement | null)[]>([]);
  const modes = ["instructions", "replies", "pack"] as const;
  const blocked = p.busy || !!p.blockedReason;
  const empty = p.mode === "instructions" ? !p.drafts.instructions.trim() : !p.drafts.conversations.trim() || !p.drafts.rules.trim();
  const notices = <>{p.blockedReason && <p role="status">{p.blockedReason}</p>}{p.error && <p role="alert">{p.error}</p>}</>;
  const importFile = (chosen?: File) => { if (chosen && !blocked) p.onImportPack(chosen); };
  return <section className="vibe-improve-start vibe-start-page" onKeyDown={event => { if (event.key === "Escape" && !p.busy) p.onBack(); }}>
    <header><VibeButton variant="quiet" className="vibe-improve-link" disabled={p.busy} onClick={p.onBack}>Back</VibeButton></header>
    <h1 className="vibe-empty-title" ref={heading} tabIndex={-1}>How do you want to test your agent?</h1>
    <div role="radiogroup" aria-label="Agent source" className="vibe-improve-segments">
      {modes.map((mode, index) => <button key={mode} ref={element => { radios.current[index] = element; }} type="button" role="radio" aria-checked={p.mode === mode} tabIndex={p.mode === mode ? 0 : -1} disabled={p.busy}
        onClick={() => p.onMode(mode)} onKeyDown={event => {
          let next = index;
          if (["ArrowRight", "ArrowDown"].includes(event.key)) next = (index + 1) % modes.length;
          else if (["ArrowLeft", "ArrowUp"].includes(event.key)) next = (index + modes.length - 1) % modes.length;
          else if (event.key === "Home") next = 0; else if (event.key === "End") next = modes.length - 1; else return;
          event.preventDefault(); p.onMode(modes[next]); radios.current[next]?.focus();
        }}>{["Instructions", "Real replies", "Test pack"][index]}</button>)}
    </div>
    {p.mode === "pack" ? <div className="vibe-improve-drop" role="group" aria-label="Test pack file" onDragOver={event => event.preventDefault()} onDrop={event => { event.preventDefault(); importFile(event.dataTransfer.files[0]); }}>
      <FileUp aria-hidden="true" /><p>Drop a test pack here</p><p className="vibe-muted">A test pack holds tests. Add your agent’s instructions after import.</p>
      <label className="sr-only" htmlFor={`${id}-file`}>Test pack file</label><input id={`${id}-file`} hidden ref={file} type="file" accept=".json,.yaml,.yml" disabled={blocked} onChange={event => { importFile(event.target.files?.[0]); event.target.value = ""; }} />
      <VibeButton variant="primary" disabled={blocked} onClick={() => file.current?.click()}>Choose a file</VibeButton>{notices}
    </div> : <form className="vibe-improve-form" onSubmit={event => {
      event.preventDefault(); if (blocked || empty) return;
      if (p.mode === "instructions") p.onWriteTests({ text: p.drafts.instructions, model: p.drafts.model });
      else p.onSaveReplies({ conversations: p.drafts.conversations, rules: p.drafts.rules });
    }}>
      {p.mode === "instructions" ? <>
        <div><div className="vibe-improve-field-heading"><label htmlFor={`${id}-instructions`}>Agent instructions</label><InstructionsHelp><VibeButton variant="quiet" className="vibe-improve-link" disabled={p.busy} onClick={() => p.onDraft({ instructions: exampleAgentDescription })}>Use sample instructions</VibeButton></InstructionsHelp></div><textarea id={`${id}-instructions`} className="vibe-textarea" rows={4} value={p.drafts.instructions} disabled={p.busy} onChange={event => p.onDraft({ instructions: event.target.value })} placeholder="Paste the instructions your agent uses…" /></div>
      </> : <>
        <p className="vibe-muted">Paste full conversations. We grade the saved replies; your app is not called.</p>
        <label htmlFor={`${id}-conversations`}>Conversations<textarea id={`${id}-conversations`} className="vibe-textarea" rows={6} value={p.drafts.conversations} disabled={p.busy} onChange={event => p.onDraft({ conversations: event.target.value })} placeholder={repliesPlaceholder} /></label>
        <ReplyCount text={p.drafts.conversations} />
        <label htmlFor={`${id}-rules`}>What a good reply must do<textarea id={`${id}-rules`} className="vibe-textarea" rows={3} value={p.drafts.rules} disabled={p.busy} onChange={event => p.onDraft({ rules: event.target.value })} placeholder="One rule per line. For example: Opened items are not eligible for a refund." /></label>
      </>}
      <div className="vibe-improve-submit"><div className="vibe-improve-submit-actions"><VibeButton variant="primary" type="submit" disabled={blocked || empty} loading={p.busy}>{p.mode === "instructions" ? "Write 3 tests" : "Save and check"}</VibeButton></div>
        <p className="text-sm vibe-muted">{p.mode === "instructions" ? "Review the tests and run price before you run." : "Save the replies, then review the grading price."}</p>{notices}
      </div>
    </form>}
    <details className="vibe-improve-other"><summary>Other ways to bring your agent</summary><div>
      <VibeButton variant="quiet" disabled={p.busy} onClick={() => p.onOther("chat")}>Describe it in chat</VibeButton>
      {p.signedIn ? <><VibeButton variant="quiet" disabled={p.busy} onClick={() => p.onOther("live_app")}>Connect a live app</VibeButton><VibeButton variant="quiet" disabled={p.busy} onClick={() => p.onOther("sdk_report")}>Import an SDK report</VibeButton></>
        : <><p><a href="/auth/login?returnTo=%2Fvibe-evals%3Fstart%3Dimprove">Sign in to connect a live app</a></p><p><a href="/auth/login?returnTo=%2Fvibe-evals%3Fstart%3Dimprove">Sign in to import an SDK report</a></p></>}
      {p.mode === "instructions" && <label htmlFor={`${id}-model`}>Which model does your agent use?<select id={`${id}-model`} value={p.drafts.model || ""} disabled={p.busy} onChange={event => p.onDraft({ model: event.target.value })}><option value="">Use the selected model</option>{p.models.map(model => <option key={model.id} value={model.id}>{model.name}</option>)}</select></label>}
      <VibeButton variant="quiet" className="vibe-improve-link" disabled={p.busy} onClick={p.onSwitchDoor}>Build an agent instead</VibeButton>
    </div></details>
    <VibeDataUseNotice />
  </section>;
}
