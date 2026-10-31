"use client";

import { useEffect, useId, useRef, useState } from "react";
import { useVibeEditorDraft } from "@/lib/use-vibe-editor-draft";
import { Tabs } from "@base-ui/react/tabs";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import type { MaterialBlocker, useVibeInputs } from "@/lib/use-vibe-inputs";
import type { TaskMaterial } from "@/lib/vibe-inputs";
import { VibeButton } from "./vibe-button";
import { VibeDataUseNotice, VibeRedactionHint } from "./vibe-data-use";
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
  const [tab, setTab] = useState<"upload" | "paste">(pdfAvailable ? "upload" : "paste");
  const pastedDraft = useVibeEditorDraft(inputs.projectID || "material", "material:" + (inputs.recipient || "default"), "paste", "paste-v1", "");
  const text = pastedDraft.value;
  const setText = pastedDraft.setValue;
  const latestPaste = useRef(text); latestPaste.current = text;
  useEffect(() => { if (!pdfAvailable) setTab("paste"); }, [pdfAvailable]);
  const [announcement, setAnnouncement] = useState("");
  const [localError, setLocalError] = useState("");
  const [deleting, setDeleting] = useState<string>();
  const [saved, setSaved] = useState<TaskMaterial[]>();
  const [savedError, setSavedError] = useState("");
  const [loadingSaved, setLoadingSaved] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const dialogRef = useRef<HTMLDivElement>(null);
  const isOpen = useRef(open); isOpen.current = open;
  const id = useId();
  const owner = useRef(inputs.draftKey); owner.current = inputs.draftKey;
  const openedOwner = useRef(inputs.draftKey);
  useEffect(() => {
    if (openedOwner.current !== inputs.draftKey) setOpen(false);
    setSaved(undefined); setSavedError(""); setLoadingSaved(false); setDeleting(undefined);
  }, [inputs.draftKey]);
  const target = useRef<{ owner: string; source: string; field: MaterialBlocker["field"] | "source" } | null>(null);
  function manage(source: string, field: MaterialBlocker["field"] | "source" = "source") {
    openedOwner.current = inputs.draftKey;
    target.current = { owner: inputs.draftKey, source, field };
    setOpen(true);
  }
  function initialFocus() {
    const intent = target.current;
    if (!intent) return titleRef.current;
    if (intent.owner !== owner.current) return false;
    const control = document.getElementById(`${id}-${intent.source}-${intent.field}`);
    const details = control?.closest("details");
    if (details) details.open = true;
    control?.scrollIntoView?.({ block: "nearest" });
    return control || titleRef.current;
  }
  function finalFocus() {
    const intent = target.current;
    if (openedOwner.current !== owner.current || isOpen.current) return false;
    const focused = document.activeElement;
    if (focused && focused !== document.body && !dialogRef.current?.contains(focused)) return false;
    return intent && document.getElementById(`${id}-${intent.source}-manage`) || triggerRef.current;
  }
  async function detach(material: TaskMaterial, paste = false) {
    const draftKey = inputs.draftKey;
    const focused = document.activeElement;
    const removed = await inputs.remove(material.id);
    if (removed === false || owner.current !== draftKey) return;
    setAnnouncement(`Detached ${material.name}`);
    // Only this explicit action may move focus, and only while it still owns it.
    if (document.activeElement !== focused && document.activeElement !== document.body) return;
    if (paste) setTab("paste");
    requestAnimationFrame(() => {
      if (owner.current !== draftKey || (document.activeElement !== focused && document.activeElement !== document.body)) return;
      (paste ? document.getElementById(`${id}-paste`) : open ? titleRef.current : triggerRef.current)?.focus();
    });
  }
  function addFile(file: File | undefined) {
    if (!file) return;
    if (!pdfAvailable) { setLocalError("PDF reading isn’t available. Paste the text instead."); return; }
    if (file.size > 10_000_000) { setLocalError("Choose a PDF no larger than 10 MB."); return; }
    if (!file.name.toLowerCase().endsWith(".pdf") && file.type !== "application/pdf") { setLocalError("Choose a PDF file."); return; }
    setLocalError("");
    void inputs.add(file).then(material => { if (material) setAnnouncement(`Attached ${material.name}`); });
  }
  async function loadSaved() {
    if (loadingSaved) return;
    const draftKey = inputs.draftKey;
    setLoadingSaved(true); setSavedError("");
    try { const materials = await inputs.list(); if (owner.current === draftKey) setSaved(materials); }
    catch { if (owner.current === draftKey) setSavedError("Could not load saved material. Try again."); }
    finally { if (owner.current === draftKey) setLoadingSaved(false); }
  }
  const inputFeedback = <>{inputs.busy && <p role="status">Saving your material… You can close this dialog while it continues.</p>}{inputs.error && <p id={`${id}-input-error`} role="alert" className="vibe-material-error">{inputs.error} {inputs.canRetry && <VibeButton variant="quiet" disabled={inputs.busy} onClick={() => void inputs.retry()}>Retry upload</VibeButton>}</p>}</>;
  return <section className="vibe-task-input" aria-label="Task material">
    <span role="status" className="sr-only">{announcement}</span>
    {!inputs.recipient?.startsWith("trial:") && <p className="vibe-material-hint">Recipient: Agent brief</p>}
    <VibeButton ref={triggerRef} variant="quiet" disabled={disabled} onClick={() => { openedOwner.current = inputs.draftKey; target.current = null; setOpen(true); }}>Add a file or text</VibeButton>
    <VibeDataUseNotice />
    {!!inputs.items.length && <ul className="vibe-material-chips" aria-label="Attached material">{inputs.items.map(({ material, usage, acknowledged }) => <li key={material.id} className="vibe-material-chip">
      <span className="vibe-material-chip-name" title={material.name}>{material.name}</span><span className="vibe-material-chip-status">{status(material)} · {usage === "rules" ? "Selected rule text" : usage === "reference" ? "Reference" : "Task input"}{material.warnings.length > 0 && !acknowledged ? " · Acknowledge partial text" : ""}</span>
      <VibeButton id={`${id}-${material.id}-manage`} variant="quiet" onClick={() => manage(material.id)} aria-label={`Manage attachment ${material.name}`}>Manage attachment</VibeButton>
      {!open && (inputs.blockers || []).some(blocker => blocker.inputID === material.id) && <div className="vibe-material-blockers">
        {(inputs.blockers || []).filter(blocker => blocker.inputID === material.id).map(blocker => <div key={blocker.code}>
          <p>{material.name}: {blocker.reason}</p>
          <VibeButton variant="quiet" disabled={disabled} aria-label={`${blocker.action} for ${material.name}`} onClick={() => manage(material.id, blocker.field)}>{blocker.action}</VibeButton>
        </div>)}
        <VibeButton variant="quiet" disabled={disabled} aria-label={`Detach ${material.name} and continue`} onClick={() => void detach(material)}>Detach and continue</VibeButton>
      </div>}
    </li>)}</ul>}
    {!open && inputs.busy && <p className="vibe-material-inline-status" role="status">Saving your material…</p>}
    {!open && inputs.error && <p className="vibe-material-inline-error" role="alert">{inputs.error} {inputs.canRetry && <VibeButton variant="quiet" disabled={inputs.busy} onClick={() => void inputs.retry()}>Retry upload</VibeButton>}</p>}
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent ref={dialogRef} className="vibe-workspace vibe-dialog vibe-material-dialog" showCloseButton={false} initialFocus={initialFocus} finalFocus={finalFocus}>
        <div className="vibe-material-dialog-head"><div className="vibe-material-dialog-heading">
          <DialogTitle ref={titleRef} tabIndex={-1}>Add material</DialogTitle>
          <DialogDescription>Choose a PDF or paste text for this task. Uploading stores material and extracts text before execution. Running sends selected material to configured model providers. Deleting prevents future source use; existing conversation/results may retain information.</DialogDescription>
        </div><VibeButton variant="quiet" onClick={() => setOpen(false)} aria-label="Close material dialog">Close</VibeButton></div>
        <div className="vibe-dialog-body vibe-material-dialog-body">
          <VibeDataUseNotice />
          <Tabs.Root value={tab} onValueChange={value => setTab(value as typeof tab)}>
            <Tabs.List className="vibe-material-tabs" aria-label="Material type"><Tabs.Tab value="upload" className="vibe-material-tab">Upload PDF</Tabs.Tab><Tabs.Tab value="paste" className="vibe-material-tab">Paste text</Tabs.Tab></Tabs.List>
            <Tabs.Panel value="upload" className="vibe-material-tab-panel">{pdfAvailable ? <>
              <label htmlFor={`${id}-pdf`}>Choose one PDF</label><input id={`${id}-pdf`} type="file" aria-describedby={localError ? `${id}-file-error` : inputs.error ? `${id}-input-error` : undefined} accept="application/pdf,.pdf" disabled={disabled || inputs.busy} onChange={event => { addFile(event.currentTarget.files?.[0]); event.currentTarget.value = ""; }} />
              <p className="vibe-material-hint">Maximum 30 pages and 10 MB. Text-based PDFs only; scans and images can’t be read.</p>
              {localError && <p id={`${id}-file-error`} role="alert" className="vibe-material-error">{localError}</p>}
            </> : <p role="status">PDF reading isn’t available right now. Use Paste text instead.</p>}{tab === "upload" && inputFeedback}</Tabs.Panel>
            <Tabs.Panel value="paste" className="vibe-material-tab-panel">{!pdfAvailable && tab === "paste" && <p role="status">PDF reading isn’t available right now. Paste text instead.</p>}<label htmlFor={`${id}-paste`}>Text to work on</label>
              <VibeRedactionHint /><textarea id={`${id}-paste`} aria-describedby={inputs.error ? `${id}-input-error` : undefined} rows={5} value={text} onChange={event => setText(event.target.value)} maxLength={60000} />
              <div className="vibe-material-actions"><VibeButton variant="primary" disabled={disabled || inputs.busy || !text.trim()} onClick={() => { const submitted = text; void inputs.add(submitted).then(material => { if (material) { if (latestPaste.current === submitted) pastedDraft.discard(submitted); setAnnouncement(`Attached ${material.name}`); } }); }}>Attach text</VibeButton></div>
              <p className="vibe-material-hint">Up to 60,000 characters. You can edit this draft after closing the dialog.</p>
              {tab === "paste" && inputFeedback}
            </Tabs.Panel>
          </Tabs.Root>
          {!!inputs.items.length && <section className="vibe-material-section" aria-label="Current attachments"><h3>Attached material</h3>
            {inputs.items.map(({ material, acknowledged, usage, page, quote, readError, waitingSince }) => <article key={material.id} id={`${id}-${material.id}-source`} tabIndex={-1} aria-label={material.name} className="vibe-material-detail">
              <div className="vibe-material-detail-head"><strong title={material.name}>{material.name}</strong><span>{status(material)}</span></div>
              {material.expires_at && <p className="vibe-material-hint">Guest material expires {new Date(material.expires_at).toLocaleDateString()}. Sign up before then to keep it.</p>}
              {material.error && <p role="alert" className="vibe-material-error">{material.error}</p>}
              {!!material.warnings.length && <label className="vibe-material-check"><input id={`${id}-${material.id}-partial`} type="checkbox" checked={acknowledged} onChange={event => inputs.acknowledge(material.id, event.target.checked)} disabled={disabled} /><span>Use readable text only. {material.warnings.join(" ")}</span></label>}
              {configure && material.status === "ready" && <details className="vibe-material-config"><summary>How to use this material</summary>
                <label htmlFor={`${id}-${material.id}-usage`}>Use as</label><select id={`${id}-${material.id}-usage`} value={usage} disabled={disabled} onChange={event => { const value = event.target.value; if (value === "task_input" || value === "reference" || value === "rules") inputs.configure(material.id, { usage: value }); }}>
                  <option value="task_input">Material to work on once</option><option value="reference">Reference for this agent</option><option value="rules">Selected text as rules</option>
                </select>
                {usage === "reference" && <p className="vibe-material-hint">This version and its tests will use the same reference. Instructions inside it aren’t rules.</p>}
                {usage === "rules" && <div className="vibe-material-rule-fields"><p className="vibe-material-hint">Choose the exact policy text to adopt. The rest of the file stays material.</p>
                  <label htmlFor={`${id}-${material.id}-page`}>Page</label><select id={`${id}-${material.id}-page`} value={page} disabled={disabled} onChange={event => inputs.configure(material.id, { page: Number(event.target.value), quote: "" })}>{material.pages?.map(p => <option key={p.number} value={p.number}>{p.number}</option>)}</select>
                  <pre className="vibe-material-preview" tabIndex={0} onMouseUp={event => { const selection = window.getSelection(); if (selection?.anchorNode && event.currentTarget.contains(selection.anchorNode) && event.currentTarget.contains(selection.focusNode) && selection.toString()) inputs.configure(material.id, { quote: selection.toString() }); }}>{material.pages?.find(p => p.number === page)?.text}</pre>
                  <label htmlFor={`${id}-${material.id}-quote`}>Exact text to use as rules</label><textarea id={`${id}-${material.id}-quote`} aria-describedby={quote && !material.pages?.find(p => p.number === page)?.text.includes(quote) ? `${id}-${material.id}-quote-error` : undefined} rows={3} disabled={disabled} value={quote} maxLength={8000} onChange={event => inputs.configure(material.id, { quote: event.target.value })} />
                  {quote && !material.pages?.find(p => p.number === page)?.text.includes(quote) && <p id={`${id}-${material.id}-quote-error`} role="alert" className="vibe-material-error">Copy the text exactly from this page.</p>}
                </div>}
              </details>}
              {readError && <p role="alert">{readError}</p>}
              {(material.status === "uploaded" || material.status === "extracting") && <div><p>{waitingSince && Date.now() - waitingSince > 60000 ? "Reading is taking longer than expected." : "Reading this source…"} You can check again, detach it, or paste the text instead.</p><VibeButton id={`${id}-${material.id}-refresh`} onClick={() => void inputs.refresh?.(material.id)}>Check again</VibeButton><VibeButton disabled={disabled} onClick={() => void detach(material, true)}>Detach and paste text instead</VibeButton></div>}
              {["failed", "unreadable", "expired", "deleted"].includes(material.status) && <div><p role="alert">This source cannot be used. Detach it and attach readable text or another file.</p><VibeButton id={`${id}-${material.id}-replacement`} disabled={disabled} onClick={() => void detach(material, true)}>Detach and paste text instead</VibeButton></div>}
              <div className="vibe-material-actions"><VibeButton variant="quiet" disabled={disabled} onClick={() => void detach(material)}>Detach</VibeButton><VibeButton variant="quiet" disabled={disabled} onClick={() => setDeleting(material.id)}>Delete file</VibeButton></div>
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
