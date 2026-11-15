"use client";
import { useEffect, useId, useRef, useState } from "react";
import { friendlyModelName } from "./model-name";
import { ChevronLeft, FileText, FileUp, MessageCircle, Package } from "lucide-react";
import { VibeButton } from "./vibe-button";
import { VibeDataUseNotice } from "./vibe-data-use";
import { exampleAgentDescription, InstructionsHelp, ReplyCount, repliesPlaceholder } from "./improve-inputs";
import { useVibeBrowserReady } from "@/lib/use-vibe-browser-ready";
import "./improve.css";

const sources = [
  { mode: "instructions", title: "Instructions", description: "Paste the prompt your agent uses", Icon: FileText },
  { mode: "replies", title: "Real replies", description: "Paste conversations it already had", Icon: MessageCircle },
  { mode: "pack", title: "Test pack", description: "Import a pack from AgentClash", Icon: Package },
] as const;
// A local report is a rare start: it lives in "Other ways" and opens its own panel, not a fourth radio.
type Mode = typeof sources[number]["mode"] | "report";
const localReportReturn = "/auth/login?returnTo=" + encodeURIComponent("/vibe-evals?start=improve&source=local-report");

export function ImproveStart(p: {
  mode: Mode; onMode(mode: Mode): void;
  drafts: { instructions: string; model?: string; conversations: string; rules: string };
  onDraft(patch: Partial<{ instructions: string; model: string; conversations: string; rules: string }>): void;
  models: { id: string; name: string }[]; signedIn: boolean; busy: boolean; error?: string; blockedReason?: string;
  onWriteTests(input: { text: string; model?: string }): void; onSaveReplies(input: { conversations: string; rules: string }): void;
  onImportPack(file: File): void; onImportReport?(source: string): void; reportBlockedReason?: string; onOther(kind: "chat" | "live_app" | "sdk_report"): void; onSwitchDoor(): void; onBack(): void;
}) {
  const ready = useVibeBrowserReady();
  const id = useId();
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => { if (ready) heading.current?.focus(); }, [ready]);
  const file = useRef<HTMLInputElement>(null), radios = useRef<(HTMLButtonElement | null)[]>([]);
  const blocked = p.busy || !!p.blockedReason;
  const empty = p.mode === "instructions" ? !p.drafts.instructions.trim() : !p.drafts.conversations.trim() || !p.drafts.rules.trim();
  const notices = <>{p.blockedReason && <p id={`${id}-blocked`} role="status">{p.blockedReason}</p>}{p.error && <p id={`${id}-error`} role="alert">{p.error}</p>}</>;
  const describedBy = [p.blockedReason && `${id}-blocked`, p.error && `${id}-error`].filter(Boolean).join(" ") || undefined;
  const importFile = (chosen?: File) => { if (chosen && !blocked) p.onImportPack(chosen); };
  const [reportFile, setReportFile] = useState<{ name: string; source: string }>(), [reportError, setReportError] = useState("");
  const reading = useRef(0);
  // Read the exact bytes as strict UTF-8; nothing leaves the browser until Import.
  const chooseReport = async (chosen?: File) => {
    if (!chosen || p.busy) return;
    const turn = ++reading.current; setReportFile(undefined); setReportError("");
    try {
      if (chosen.size > 1024 * 1024) throw Error("Report files must be at most 1 MiB.");
      const source = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(await chosen.arrayBuffer());
      if (turn === reading.current) setReportFile({ name: chosen.name, source });
    } catch (e) { if (turn === reading.current) setReportError(e instanceof TypeError ? "This file is not UTF-8 text. Choose the results.json that the local runner wrote." : (e as Error).message); }
  };
  const reportBlocked = p.busy || !!p.reportBlockedReason;
  return <section className="vibe-improve-start vibe-start-page vibe-start-layout" onKeyDown={event => { if (event.key === "Escape" && !p.busy) p.onBack(); }}>
    <header><VibeButton variant="quiet" className="vibe-start-back" disabled={p.busy} onClick={p.onBack}><ChevronLeft aria-hidden="true" />All projects</VibeButton></header>
    <div className="vibe-start-intro">
      <p className="vibe-start-eyebrow">Improve an agent</p>
      <h1 className="vibe-start-title" ref={heading} tabIndex={-1}>What can you share about your agent?</h1>
    </div>
    {p.mode !== "report" && <div role="radiogroup" aria-label="Agent source" className="vibe-improve-sources">
      {sources.map(({ mode, title, description, Icon }, index) => <button key={mode} ref={element => { radios.current[index] = element; }} type="button" role="radio" aria-checked={p.mode === mode} aria-labelledby={`${id}-${mode}-title`} aria-describedby={`${id}-${mode}-description`} tabIndex={p.mode === mode ? 0 : -1} disabled={p.busy}
        onClick={() => p.onMode(mode)} onKeyDown={event => {
          let next = index;
          if (p.busy) return;
          if (["ArrowRight", "ArrowDown"].includes(event.key)) next = (index + 1) % sources.length;
          else if (["ArrowLeft", "ArrowUp"].includes(event.key)) next = (index + sources.length - 1) % sources.length;
          else if (event.key === "Home") next = 0; else if (event.key === "End") next = sources.length - 1; else return;
          event.preventDefault(); p.onMode(sources[next].mode); radios.current[next]?.focus();
        }}>
        <span className="vibe-improve-source-icons" aria-hidden="true"><Icon size={20} /><span className="vibe-improve-radio" /></span>
        <span className="vibe-improve-source-copy"><span id={`${id}-${mode}-title`} className="vibe-improve-source-title">{title}</span><span id={`${id}-${mode}-description`} className="vibe-improve-source-description" aria-hidden="true">{description}</span></span>
      </button>)}
    </div>}
    {p.mode === "report" ? <div className="vibe-improve-drop" role="group" aria-label="Local report file" onDragOver={event => event.preventDefault()} onDrop={event => { event.preventDefault(); if (p.signedIn) void chooseReport(event.dataTransfer.files[0]); }}>
      <FileUp aria-hidden="true" /><p>Keep a local report</p>
      <p className="vibe-muted">Choose the results.json that the local runner wrote. We make placeholder tests from its case IDs and keep the report with them. Nothing runs, and no inference credits are used.</p>
      <p className="vibe-muted">The complete original file is kept privately in your workspace, including any prompts, replies and tool arguments. Its results are uploader claims, not verified execution. The placeholder tests cannot run here.</p>
      {!p.signedIn ? <p><a className="underline" href={localReportReturn}>Sign in to keep a local report</a></p> : <>
        <label className="sr-only" htmlFor={`${id}-report`}>Local report file</label><input id={`${id}-report`} hidden ref={file} type="file" accept="application/json,.json" disabled={p.busy} onChange={event => { void chooseReport(event.target.files?.[0]); event.target.value = ""; }} />
        <VibeButton disabled={p.busy} onClick={() => file.current?.click()}>{reportFile ? "Choose a different file" : "Choose results.json"}</VibeButton>
        {reportFile && <p role="status">Selected: {reportFile.name}</p>}
        <VibeButton variant="primary" aria-describedby={[p.reportBlockedReason && `${id}-report-blocked`, p.error && `${id}-error`].filter(Boolean).join(" ") || undefined} disabled={reportBlocked || !reportFile} loading={p.busy} onClick={() => { if (reportFile && !reportBlocked) p.onImportReport?.(reportFile.source); }}>Import</VibeButton>
        {p.reportBlockedReason && <p id={`${id}-report-blocked`} role="status">{p.reportBlockedReason}</p>}
      </>}
      {reportError && <p role="alert">{reportError}</p>}{p.error && <p id={`${id}-error`} role="alert">{p.error}</p>}
      <VibeButton variant="quiet" disabled={p.busy} onClick={() => p.onMode("instructions")}>Bring an agent a different way</VibeButton>
    </div> : p.mode === "pack" ? <div className="vibe-improve-drop" role="group" aria-label="Test pack file" onDragOver={event => event.preventDefault()} onDrop={event => { event.preventDefault(); importFile(event.dataTransfer.files[0]); }}>
      <FileUp aria-hidden="true" /><p>Drop a test pack here</p><p className="vibe-muted">A test pack holds tests. Add your agent’s instructions after import.</p>
      <label className="sr-only" htmlFor={`${id}-file`}>Test pack file</label><input id={`${id}-file`} hidden ref={file} type="file" accept=".json,.yaml,.yml" disabled={blocked} onChange={event => { importFile(event.target.files?.[0]); event.target.value = ""; }} />
      <VibeButton variant="primary" aria-describedby={describedBy} disabled={blocked} onClick={() => file.current?.click()}>Choose a file</VibeButton>{notices}
    </div> : <form className="vibe-improve-form" onSubmit={event => {
      event.preventDefault(); if (blocked || empty) return;
      if (p.mode === "instructions") p.onWriteTests({ text: p.drafts.instructions, model: p.drafts.model });
      else p.onSaveReplies({ conversations: p.drafts.conversations, rules: p.drafts.rules });
    }}>
      {p.mode === "instructions" ? <>
        <div className="vibe-improve-field">
          <div className="vibe-improve-field-heading"><label htmlFor={`${id}-instructions`}>Agent instructions</label><VibeButton variant="quiet" className="vibe-improve-sample" disabled={p.busy} onClick={() => p.onDraft({ instructions: exampleAgentDescription })}>Use sample instructions</VibeButton></div>
          <textarea id={`${id}-instructions`} className="vibe-textarea vibe-improve-instructions" aria-describedby={`${id}-instructions-helper${p.error ? ` ${id}-error` : ""}`} rows={4} value={p.drafts.instructions} disabled={p.busy} onChange={event => p.onDraft({ instructions: event.target.value })} placeholder="Paste the instructions your agent uses…" />
          <p id={`${id}-instructions-helper`} className="vibe-improve-helper">Find these in your agent&apos;s system prompt or settings. Remove keys and customer data.</p>
          <InstructionsHelp />
        </div>
        <label htmlFor={`${id}-model`}>Model your agent uses<select id={`${id}-model`} value={p.drafts.model || ""} disabled={p.busy} onChange={event => p.onDraft({ model: event.target.value })}><option value="">Use the selected model</option>{p.models.map(model => <option key={model.id} value={model.id}>{friendlyModelName(model)} · {model.id}</option>)}</select></label>
      </> : <>
        <p className="vibe-muted">Paste full conversations. We grade the saved replies; your app is not called.</p>
        <label htmlFor={`${id}-conversations`}>Conversations<textarea id={`${id}-conversations`} className="vibe-textarea" aria-describedby={p.error ? `${id}-error` : undefined} rows={6} value={p.drafts.conversations} disabled={p.busy} onChange={event => p.onDraft({ conversations: event.target.value })} placeholder={repliesPlaceholder} /></label>
        <ReplyCount text={p.drafts.conversations} />
        <label htmlFor={`${id}-rules`}>What a good reply must do<textarea id={`${id}-rules`} className="vibe-textarea" aria-describedby={p.error ? `${id}-error` : undefined} rows={3} value={p.drafts.rules} disabled={p.busy} onChange={event => p.onDraft({ rules: event.target.value })} placeholder="One rule per line. For example: Opened items are not eligible for a refund." /></label>
      </>}
      <div className="vibe-improve-submit"><div className="vibe-improve-submit-actions"><VibeButton variant="primary" aria-describedby={describedBy} type="submit" disabled={blocked || empty} loading={p.busy}>{p.mode === "instructions" ? "Write 3 tests" : "Save and check"}</VibeButton></div>
        <p className="vibe-improve-helper">{p.mode === "instructions" ? "You review the tests and see the price before anything runs." : "Save the replies, then review the grading price."}</p>{notices}
      </div>
    </form>}
    <details className="vibe-improve-other"><summary>Other ways to bring your agent</summary><div>
      <VibeButton variant="quiet" disabled={p.busy} onClick={() => p.onOther("chat")}>Describe it in chat</VibeButton>
      {p.signedIn ? <><VibeButton variant="quiet" disabled={p.busy} onClick={() => p.onOther("live_app")}>Connect a live app</VibeButton><VibeButton variant="quiet" disabled={p.busy} onClick={() => p.onOther("sdk_report")}>Import an SDK report</VibeButton>{p.mode !== "report" && <VibeButton variant="quiet" disabled={p.busy} onClick={() => p.onMode("report")}>Keep a local report</VibeButton>}</>
        : <><p><a href="/auth/login?returnTo=%2Fvibe-evals%3Fstart%3Dimprove">Sign in to connect a live app</a></p><p><a href="/auth/login?returnTo=%2Fvibe-evals%3Fstart%3Dimprove">Sign in to import an SDK report</a></p>{p.mode !== "report" && <p><a href={localReportReturn}>Sign in to keep a local report</a></p>}</>}
    </div></details>
    <footer className="vibe-start-footer">
      <VibeDataUseNotice />
      <VibeButton variant="quiet" className="vibe-start-switch" disabled={p.busy} onClick={p.onSwitchDoor}>No agent yet? Build one</VibeButton>
    </footer>
  </section>;
}
