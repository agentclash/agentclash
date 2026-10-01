"use client";

import { useId, useRef, useState } from "react";
import { Tabs } from "@base-ui/react/tabs";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import type { useVibeInputs } from "@/lib/use-vibe-inputs";
import type { TaskMaterial } from "@/lib/vibe-inputs";
import { VibeButton } from "./vibe-button";
import "./material-input.css";

type Props = { inputs: ReturnType<typeof useVibeInputs>; pdfAvailable: boolean; disabled: boolean; configure?: boolean };
function status(material: TaskMaterial) {
  switch (material.status) {
    case "uploaded": case "extracting": return "Reading PDF";
    case "ready": return material.warnings.length ? "Partial text" : "Ready";
    case "unreadable": return "No readable text";
    case "failed": return "Reading failed";
    case "expired": return "Expired";
    case "deleted": return "Deleted";
  }
}

export function TaskInput({ inputs, pdfAvailable, disabled, configure = false }: Props) {
  const [open, setOpen] = useState(false);
  const [tab, setTab] = useState<"upload" | "paste">("upload");
  const [text, setText] = useState("");
  const [localError, setLocalError] = useState("");
  const [deleting, setDeleting] = useState<string>();
  const [saved, setSaved] = useState<TaskMaterial[]>();
  const [savedError, setSavedError] = useState("");
  const [loadingSaved, setLoadingSaved] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const id = useId();
  function addFile(file: File | undefined) {
    if (!file) return;
    if (!pdfAvailable) { setLocalError("PDF reading isn’t available. Paste the text instead."); return; }
    if (file.size > 10_000_000) { setLocalError("Choose a PDF no larger than 10 MB."); return; }
    if (!file.name.toLowerCase().endsWith(".pdf") && file.type !== "application/pdf") { setLocalError("Choose a PDF file."); return; }
    setLocalError("");
    void inputs.add(file);
  }
  async function loadSaved() {
    if (loadingSaved) return;
    setLoadingSaved(true); setSavedError("");
    try { setSaved(await inputs.list()); }
    catch { setSavedError("Could not load saved material. Try again."); }
    finally { setLoadingSaved(false); }
  }
  return <section className="vibe-task-input" aria-label="Task material">
    <VibeButton ref={triggerRef} variant="quiet" disabled={disabled} onClick={() => setOpen(true)}>Add a file or text</VibeButton>
    {!!inputs.items.length && <ul className="vibe-material-chips" aria-label="Attached material">{inputs.items.map(({ material }) => <li key={material.id} className="vibe-material-chip">
      <span className="vibe-material-chip-name" title={material.name}>{material.name}</span><span className="vibe-material-chip-status">{status(material)}</span>
      <VibeButton variant="quiet" onClick={() => setOpen(true)} aria-label={`Manage attachment ${material.name}`}>Manage attachment</VibeButton>
    </li>)}</ul>}
    {!open && inputs.busy && <p className="vibe-material-inline-status" role="status">Saving your material…</p>}
    {!open && inputs.error && <p className="vibe-material-inline-error" role="alert">{inputs.error} {inputs.canRetry && <VibeButton variant="quiet" disabled={inputs.busy} onClick={() => void inputs.retry()}>Retry upload</VibeButton>}</p>}
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="vibe-workspace vibe-dialog vibe-material-dialog" showCloseButton={false} initialFocus={titleRef} finalFocus={triggerRef}>
        <div className="vibe-material-dialog-head"><div className="vibe-material-dialog-heading">
          <DialogTitle ref={titleRef} tabIndex={-1}>Add material</DialogTitle>
          <DialogDescription>Choose a PDF or paste text for this task. Adding material does not run the agent or change its rules.</DialogDescription>
        </div><VibeButton variant="quiet" onClick={() => setOpen(false)} aria-label="Close material dialog">Close</VibeButton></div>
        <div className="vibe-dialog-body vibe-material-dialog-body">
          <Tabs.Root value={tab} onValueChange={value => setTab(value as typeof tab)}>
            <Tabs.List className="vibe-material-tabs" aria-label="Material type"><Tabs.Tab value="upload" className="vibe-material-tab">Upload PDF</Tabs.Tab><Tabs.Tab value="paste" className="vibe-material-tab">Paste text</Tabs.Tab></Tabs.List>
            <Tabs.Panel value="upload" className="vibe-material-tab-panel">{pdfAvailable ? <>
              <label htmlFor={`${id}-pdf`}>Choose one PDF</label><input id={`${id}-pdf`} type="file" accept="application/pdf,.pdf" disabled={disabled || inputs.busy} onChange={event => { addFile(event.currentTarget.files?.[0]); event.currentTarget.value = ""; }} />
              <p className="vibe-material-hint">Maximum 30 pages and 10 MB. Text-based PDFs only; scans and images can’t be read.</p>
            </> : <p role="status">PDF reading isn’t available right now. Use Paste text instead.</p>}</Tabs.Panel>
            <Tabs.Panel value="paste" className="vibe-material-tab-panel"><label htmlFor={`${id}-paste`}>Text to work on</label>
              <textarea id={`${id}-paste`} rows={5} value={text} onChange={event => setText(event.target.value)} maxLength={60000} />
              <div className="vibe-material-actions"><VibeButton variant="primary" disabled={disabled || inputs.busy || !text.trim()} onClick={() => void inputs.add(text)}>Attach text</VibeButton></div>
              <p className="vibe-material-hint">Up to 60,000 characters. You can edit this draft after closing the dialog.</p>
            </Tabs.Panel>
          </Tabs.Root>
          {localError && <p role="alert" className="vibe-material-error">{localError}</p>}
          {inputs.busy && <p role="status">Saving your material… You can close this dialog while it continues.</p>}
          {inputs.error && <p role="alert" className="vibe-material-error">{inputs.error} {inputs.canRetry && <VibeButton variant="quiet" disabled={inputs.busy} onClick={() => void inputs.retry()}>Retry upload</VibeButton>}</p>}
          {!!inputs.items.length && <section className="vibe-material-section" aria-label="Current attachments"><h3>Attached material</h3>
            {inputs.items.map(({ material, acknowledged, usage, page, quote }) => <article key={material.id} className="vibe-material-detail">
              <div className="vibe-material-detail-head"><strong title={material.name}>{material.name}</strong><span>{status(material)}</span></div>
              {material.expires_at && <p className="vibe-material-hint">Guest material expires {new Date(material.expires_at).toLocaleDateString()}. Sign up before then to keep it.</p>}
              {material.error && <p role="alert" className="vibe-material-error">{material.error}</p>}
              {!!material.warnings.length && <label className="vibe-material-check"><input type="checkbox" checked={acknowledged} onChange={event => inputs.acknowledge(material.id, event.target.checked)} disabled={disabled} /><span>Use readable text only. {material.warnings.join(" ")}</span></label>}
              {configure && material.status === "ready" && <details className="vibe-material-config"><summary>How to use this material</summary>
                <label htmlFor={`${id}-${material.id}-usage`}>Use as</label><select id={`${id}-${material.id}-usage`} value={usage} disabled={disabled} onChange={event => { const value = event.target.value; if (value === "task_input" || value === "reference" || value === "rules") inputs.configure(material.id, { usage: value }); }}>
                  <option value="task_input">Material to work on once</option><option value="reference">Reference for this agent</option><option value="rules">Selected text as rules</option>
                </select>
                {usage === "reference" && <p className="vibe-material-hint">This version and its checks will use the same reference. Instructions inside it aren’t rules.</p>}
                {usage === "rules" && <div className="vibe-material-rule-fields"><p className="vibe-material-hint">Choose the exact policy text to adopt. The rest of the file stays material.</p>
                  <label htmlFor={`${id}-${material.id}-page`}>Page</label><select id={`${id}-${material.id}-page`} value={page} disabled={disabled} onChange={event => inputs.configure(material.id, { page: Number(event.target.value), quote: "" })}>{material.pages?.map(p => <option key={p.number} value={p.number}>{p.number}</option>)}</select>
                  <pre className="vibe-material-preview">{material.pages?.find(p => p.number === page)?.text}</pre>
                  <label htmlFor={`${id}-${material.id}-quote`}>Exact text to use as rules</label><textarea id={`${id}-${material.id}-quote`} rows={3} disabled={disabled} value={quote} maxLength={8000} onChange={event => inputs.configure(material.id, { quote: event.target.value })} />
                  {quote && !material.pages?.find(p => p.number === page)?.text.includes(quote) && <p role="alert" className="vibe-material-error">Copy the text exactly from this page.</p>}
                </div>}
              </details>}
              <div className="vibe-material-actions"><VibeButton variant="quiet" disabled={disabled} onClick={() => void inputs.remove(material.id)}>Detach</VibeButton><VibeButton variant="quiet" disabled={disabled} onClick={() => setDeleting(material.id)}>Delete file</VibeButton></div>
              {deleting === material.id && <div className="vibe-material-delete" role="alert"><p>This deletes the file. Your conversation and existing results remain and may contain information from it.</p><div className="vibe-material-actions"><VibeButton disabled={disabled} onClick={() => { void inputs.remove(material.id, true); setDeleting(undefined); }}>Delete file permanently</VibeButton><VibeButton variant="quiet" onClick={() => setDeleting(undefined)}>Cancel</VibeButton></div></div>}
            </article>)}
          </section>}
          <details className="vibe-material-section vibe-material-saved"><summary onClick={event => { if (!(event.currentTarget.parentElement as HTMLDetailsElement | null)?.open && !saved) void loadSaved(); }}>Saved material</summary>
            {loadingSaved && <p role="status">Loading saved material…</p>}
            {savedError && <p role="alert">{savedError} <VibeButton variant="quiet" onClick={() => void loadSaved()}>Try again</VibeButton></p>}
            {saved && <ul aria-label="Saved material">{saved.filter(item => item.status !== "deleted" && item.status !== "expired").map(item => <li key={item.id}><span title={item.name}>{item.name} · {status(item)}</span><VibeButton variant="quiet" disabled={disabled || inputs.items.some(x => x.material.id === item.id)} onClick={() => void inputs.attach(item.id)}>Attach</VibeButton></li>)}{!saved.some(item => item.status !== "deleted" && item.status !== "expired") && <li>No saved material yet.</li>}</ul>}
          </details>
        </div>
      </DialogContent>
    </Dialog>
  </section>;
}
