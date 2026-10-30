"use client";
import { useEffect, useRef, useState } from "react";
import { VibeButton } from "./vibe-button";

export function ImproveStart(p: { mode: "instructions" | "replies" | "pack"; onMode(mode: "instructions" | "replies" | "pack"): void; drafts: { instructions: string; model?: string; conversations: string; rules: string }; onDraft(patch: Partial<{ instructions: string; model: string; conversations: string; rules: string }>): void; models: { id: string; name: string }[]; signedIn: boolean; busy: boolean; error?: string; blockedReason?: string; onWriteTests(input: { text: string; model?: string }): void; onSaveReplies(input: { conversations: string; rules: string }): void; onImportPack(file: File): void; onOther(kind: "chat" | "live_app" | "sdk_report"): void; onSwitchDoor(): void; onBack(): void }) {
  const heading = useRef<HTMLHeadingElement>(null);
  const file = useRef<HTMLInputElement>(null);
  const [chosenFile, setChosenFile] = useState<File>();
  useEffect(() => { heading.current?.focus(); }, []);
  return <section className="vibe-column vibe-start-page" onKeyDown={e => { if (e.key === "Escape" && !p.busy) p.onBack(); }}>
    <VibeButton variant="quiet" disabled={p.busy} onClick={p.onBack}>Back</VibeButton>
    <h1 ref={heading} tabIndex={-1}>Improve an agent you already have.</h1>
    <div className="vibe-start-modes" role="group" aria-label="Agent source">{([['instructions', 'Try my saved instructions'], ['replies', 'Test replies from my app'], ['pack', 'Import a test pack']] as const).map(([mode, label]) => <VibeButton key={mode} aria-pressed={p.mode === mode} disabled={p.busy} onClick={() => { p.onMode(mode); if (mode === "pack") file.current?.click(); }}>{label}</VibeButton>)}</div>
    {p.mode === "instructions" ? <form className="vibe-start-form" onSubmit={e => { e.preventDefault(); if (!p.busy && !p.blockedReason && p.drafts.instructions.trim()) p.onWriteTests({ text: p.drafts.instructions, model: p.drafts.model }); }}>
      <label htmlFor="vibe-instructions-source">Agent instructions to test</label><textarea id="vibe-instructions-source" aria-label="Agent instructions to test" rows={6} value={p.drafts.instructions} onChange={e => p.onDraft({ instructions: e.target.value })} />
      <label>Agent model<select value={p.drafts.model || p.models[0]?.id || ""} onChange={e => p.onDraft({ model: e.target.value })}>{p.models.map(model => <option value={model.id} key={model.id}>{model.name}</option>)}</select></label>
      <VibeButton type="submit" variant="primary" disabled={p.busy || !!p.blockedReason || !p.drafts.instructions.trim()}>{p.busy ? "Starting…" : "Prepare tests"}</VibeButton>
      {p.error && <p role="alert">{p.error}</p>}
    </form> : p.mode === "replies" ? <form className="vibe-start-form" onSubmit={e => { e.preventDefault(); if (!p.busy && p.drafts.conversations.trim() && p.drafts.rules.trim()) p.onSaveReplies(p.drafts); }}>
      <p>Paste full conversations. Your app is not called.</p><label>Recorded replies<textarea aria-label="Recorded replies" rows={6} value={p.drafts.conversations} onChange={e => p.onDraft({ conversations: e.target.value })} placeholder={'Customer: Can I return this?\nAgent: Was it opened?'} /></label>
      <label>Expected behavior<textarea aria-label="Expected behavior" rows={3} value={p.drafts.rules} onChange={e => p.onDraft({ rules: e.target.value })} /></label>
      <VibeButton type="submit" variant="primary" disabled={p.busy || !p.drafts.conversations.trim() || !p.drafts.rules.trim()}>{p.busy ? "Starting…" : "Review recorded replies"}</VibeButton>
      {p.error && <p role="alert">{p.error}</p>}
    </form> : <div><p>A test pack is a saved set of tests. Add your agent after import.</p><VibeButton variant="primary" disabled={p.busy} onClick={() => chosenFile ? p.onImportPack(chosenFile) : file.current?.click()}>{chosenFile ? "Import test pack" : "Choose a file"}</VibeButton>{chosenFile && <><p>{chosenFile.name}</p><VibeButton disabled={p.busy} onClick={() => file.current?.click()}>Choose another file</VibeButton></>}{p.error && <p role="alert">{p.error}</p>}</div>}
    <input ref={file} hidden type="file" accept=".json,.yaml,.yml" onChange={e => { const chosen = e.target.files?.[0]; if (chosen) { setChosenFile(chosen); p.onImportPack(chosen); } e.target.value = ""; }} />
    {p.blockedReason && p.mode === "instructions" && <p role="status">{p.blockedReason}</p>}
    <VibeButton variant="quiet" disabled={p.busy} onClick={p.onSwitchDoor}>Actually, I want to build an agent.</VibeButton>
  </section>;
}
