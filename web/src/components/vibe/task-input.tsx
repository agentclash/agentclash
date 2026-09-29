"use client";

import { useId, useState } from "react";
import type { useVibeInputs } from "@/lib/use-vibe-inputs";
import { VibeButton } from "./vibe-button";
import type { TaskMaterial } from "@/lib/vibe-inputs";

export function TaskInput({ inputs, pdfAvailable, disabled, configure = false }: { inputs: ReturnType<typeof useVibeInputs>; pdfAvailable: boolean; disabled: boolean; configure?: boolean }) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");
  const [deleting, setDeleting] = useState<string>();
  const [saved, setSaved] = useState<TaskMaterial[]>();
  const [savedError, setSavedError] = useState("");
  const id = useId();
  return <section className="vibe-task-input" aria-label="Task material">
    <VibeButton variant="quiet" disabled={disabled} onClick={() => setOpen(!open)}>{open ? "Close material controls" : "Add material"}</VibeButton>
    {open && <div className="vibe-panel p-3 space-y-2">
      <p className="text-sm vibe-muted">Add the text you want it to work on. Uploading alone doesn’t run anything or change your rules.</p>
      <label htmlFor={`${id}-paste`}>Paste text</label>
      <textarea id={`${id}-paste`} className="vibe-textarea w-full" rows={3} value={text} onChange={e => setText(e.target.value)} />
      <VibeButton disabled={disabled || inputs.busy || !text.trim()} onClick={() => void inputs.add(text)}>Attach text</VibeButton>
      {pdfAvailable ? <label className="block text-sm" htmlFor={`${id}-pdf`}>Or choose one PDF · 30 pages · 10 MB maximum<input className="block mt-2" id={`${id}-pdf`} type="file" accept="application/pdf,.pdf" disabled={disabled || inputs.busy} onChange={e => { const file = e.target.files?.[0]; if (file) void inputs.add(file); e.target.value = ""; }} /></label> : <p className="text-sm vibe-muted">PDF reading isn’t available right now. Paste the text instead.</p>}
      <p className="text-xs vibe-muted">Text-based PDFs only. Images and scanned pages aren’t read.</p>
    </div>}
    {open && <><VibeButton variant="quiet" disabled={disabled} onClick={() => { void inputs.list().then(setSaved).catch(() => setSavedError("Could not load saved material. Try again.")); }}>Saved material</VibeButton>
      {savedError && <p role="alert">{savedError}</p>}
      {saved && <ul aria-label="Saved material">{saved.filter(item => item.status !== "deleted" && item.status !== "expired").map(item => <li key={item.id}><VibeButton variant="quiet" disabled={disabled || inputs.items.some(x => x.material.id === item.id)} onClick={() => void inputs.attach(item.id)}>{item.name} · {item.status}</VibeButton></li>)}</ul>}
    </>}
    {inputs.items.map(({ material, acknowledged, usage, page, quote }) => <div key={material.id} className="vibe-material-chip">
      <p><strong>{material.name}</strong> · {material.status === "uploaded" || material.status === "extracting" ? "Reading your PDF…" : material.status === "ready" ? "Ready to use" : material.status}</p>
      {material.expires_at && <p className="text-xs vibe-muted">Guest material expires {new Date(material.expires_at).toLocaleDateString()}. Sign up before then to keep it.</p>}
      {material.error && <p role="alert">{material.error}</p>}
      {configure && material.status === "ready" && <details><summary>How to use this material</summary>
        <label htmlFor={`${id}-${material.id}-usage`}>Use as</label><select id={`${id}-${material.id}-usage`} value={usage} disabled={disabled} onChange={e => { const value = e.target.value; if (value === "task_input" || value === "reference" || value === "rules") inputs.configure(material.id, { usage: value }); }}>
          <option value="task_input">Material to work on once</option><option value="reference">Reference for this agent</option><option value="rules">Selected text as rules</option>
        </select>
        {usage === "reference" && <p className="text-sm vibe-muted">This version and its checks will use the same reference. Instructions inside it aren’t rules.</p>}
        {usage === "rules" && <div className="space-y-2"><p className="text-sm vibe-muted">Choose the exact policy text you want to adopt. We’ll check its meaning before using it; the rest of the file stays material.</p>
          <label htmlFor={`${id}-${material.id}-page`}>Page</label><select id={`${id}-${material.id}-page`} value={page} disabled={disabled} onChange={e => inputs.configure(material.id, { page: Number(e.target.value), quote: "" })}>{material.pages?.map(p => <option key={p.number} value={p.number}>{p.number}</option>)}</select>
          <pre className="vibe-material-preview">{material.pages?.find(p => p.number === page)?.text}</pre>
          <label htmlFor={`${id}-${material.id}-quote`}>Exact text to use as rules</label><textarea id={`${id}-${material.id}-quote`} rows={3} disabled={disabled} value={quote} maxLength={8000} onChange={e => inputs.configure(material.id, { quote: e.target.value })} />
          {quote && !material.pages?.find(p => p.number === page)?.text.includes(quote) && <p role="alert">Copy the text exactly from this page.</p>}
        </div>}
      </details>}
      {!!material.warnings.length && <label className="text-sm"><input type="checkbox" checked={acknowledged} onChange={e => inputs.acknowledge(material.id, e.target.checked)} disabled={disabled} /> Use readable text only. {material.warnings.join(" ")}</label>}
      <div className="flex gap-2"><VibeButton variant="quiet" disabled={disabled} onClick={() => void inputs.remove(material.id)}>Detach</VibeButton><VibeButton variant="quiet" disabled={disabled} onClick={() => setDeleting(material.id)}>Delete file</VibeButton></div>
      {deleting === material.id && <div role="alert"><p>This deletes the file. Your conversation and existing results remain and may contain information from it.</p><VibeButton onClick={() => { void inputs.remove(material.id, true); setDeleting(undefined); }}>Delete file permanently</VibeButton><VibeButton variant="quiet" onClick={() => setDeleting(undefined)}>Cancel</VibeButton></div>}
    </div>)}
    {inputs.busy && <p role="status">Saving your material…</p>}
    {inputs.error && <p role="alert">{inputs.error} {inputs.canRetry && <button type="button" onClick={() => void inputs.retry()}>Retry upload</button>}</p>}
  </section>;
}
