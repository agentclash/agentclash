"use client";
import { useRef, useState } from "react";
import { vibeFetch } from "@/lib/vibe";
import { inputPath, type InputBinding, type TaskMaterial } from "@/lib/vibe-inputs";
import { useVibeConnection } from "@/lib/vibe-connection";
import { VibeButton } from "./vibe-button";
type Source = { material?: TaskMaterial; error?: string; busy?: boolean };
// Saved execution bindings are authoritative; one unavailable source cannot hide its siblings.
export function TaskSources({ sessionID, bindings }: { sessionID: string; bindings: InputBinding[] }) {
  const { token } = useVibeConnection();
  const identity = JSON.stringify([sessionID, bindings]);
  const owner = useRef(identity); owner.current = identity;
  const [state, setState] = useState<{ identity: string; rows: Record<string, Source> }>({ identity, rows: {} });
  const rows = state.identity === identity ? state.rows : {};
  const inFlight = useRef(new Set<string>());
  function update(id: string, value: Source) { if (owner.current === identity) setState(old => ({ identity, rows: { ...(old.identity === identity ? old.rows : {}), [id]: value } })); }
  async function load(binding: InputBinding) {
    const key = identity + binding.input_id; if (inFlight.current.has(key)) return;
    inFlight.current.add(key); update(binding.input_id, { busy: true });
    try {
      const material = await vibeFetch<TaskMaterial>(inputPath(sessionID, binding.input_id), await token());
      if (material.status !== "ready" || material.content_hash !== binding.content_hash) throw Error("This source was deleted, expired or changed. The saved output remains available.");
      update(binding.input_id, { material });
    } catch (error) { update(binding.input_id, { error: (error as Error).message }); }
    finally { inFlight.current.delete(key); }
  }
  return <details className="vibe-task-sources mt-3" onToggle={event => { if (event.currentTarget.open) for (const binding of bindings) if (!rows[binding.input_id]) void load(binding); }}>
    <summary>Material used</summary><p className="text-sm vibe-muted">This is the text supplied to the agent, not proof that every statement in its reply is correct. PDF images and visual layout were not checked.</p>
    {bindings.map(binding => { const row = rows[binding.input_id]; return <section key={binding.input_id} aria-label={`Source ${row?.material?.name || binding.input_id}`}>
      {row?.busy && <p role="status">Loading source {binding.input_id}…</p>}
      {row?.error && <p role="alert">{row.error} <VibeButton onClick={() => void load(binding)}>Retry source {binding.input_id}</VibeButton></p>}
      {row?.material && <><h4>{row.material.name}</h4>{row.material.warnings.map(warning => <p key={warning}>{warning}</p>)}{row.material.pages?.map(page => <details key={page.number}><summary>Page {page.number}</summary><pre className="whitespace-pre-wrap break-words text-sm">{page.text}</pre></details>)}</>}
    </section>; })}
  </details>;
}
