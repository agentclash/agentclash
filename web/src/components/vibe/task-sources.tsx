"use client";

import { useState } from "react";
import { vibeFetch } from "@/lib/vibe";
import { inputPath, type InputBinding, type TaskMaterial } from "@/lib/vibe-inputs";
import { useVibeConnection } from "@/lib/vibe-connection";
import { VibeButton } from "./vibe-button";

// These links come from the server's execution bindings, never model prose.
export function TaskSources({ sessionID, bindings }: { sessionID: string; bindings: InputBinding[] }) {
  const { token } = useVibeConnection();
  const [records, setRecords] = useState<TaskMaterial[]>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function load() {
    if (busy) return;
    setBusy(true); setError("");
    try {
      const auth = await token();
      const result = await Promise.all(bindings.map(b => vibeFetch<TaskMaterial>(inputPath(sessionID,b.input_id),auth)));
      if (result.some((r,i) => r.status !== "ready" || r.content_hash !== bindings[i].content_hash)) throw new Error("Some source material was deleted or expired. The saved output remains available.");
      setRecords(result);
    } catch (e) { setError((e as Error).message); }
    finally { setBusy(false); }
  }
  return <details className="vibe-task-sources mt-3" onToggle={e => { if(e.currentTarget.open && !records) void load(); }}>
    <summary>Material used</summary>
    <p className="text-sm vibe-muted">This is the text supplied to the agent, not proof that every statement in its reply is correct. PDF images and visual layout were not checked.</p>
    {busy && <p role="status">Loading source text…</p>}
    {error && <p role="alert">{error} <VibeButton variant="quiet" onClick={() => void load()}>Retry source read</VibeButton></p>}
    {records?.map(r => <div key={r.id}><p>{r.name}</p>{r.warnings.map(w => <p key={w} className="text-sm vibe-muted">{w}</p>)}{r.pages?.map(p => <details key={p.number}><summary>Page {p.number}</summary><pre className="whitespace-pre-wrap break-words max-h-64 overflow-auto text-sm">{p.text}</pre></details>)}</div>)}
  </details>;
}
