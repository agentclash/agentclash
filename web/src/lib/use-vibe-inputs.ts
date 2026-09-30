"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { inputPath, uploadMaterial, type DocumentSource, type InputBinding, type TaskMaterial } from "./vibe-inputs";
import { selectMaterials } from "./vibe-material-selection";
import { WEB_EVENTS } from "./analytics/events";
import { captureBuildEvent } from "./vibe-build-analytics";
import { vibeFetch } from "./vibe";

type Selection = { material: TaskMaterial; acknowledged: boolean; usage: "task_input" | "reference" | "rules"; page: number; quote: string };
type Draft = { items: Selection[]; busy: boolean; error: string };
const empty: Draft = { items: [], busy: false, error: "" };

// This hook owns attachment UI only. Paid submissions still belong to the
// existing Vibe client. Every asynchronous write captures its original scope.
export function useVibeInputs(session: string, scope: string, token: () => Promise<string | null | undefined>) {
  const [drafts, setDrafts] = useState<Record<string, Draft>>({});
  const key = `${session}:${scope}`;
  const current = drafts[key] || empty;
  const pending = useRef(new Map<string, { id: string; value: File | string }>());
  const inFlight = useRef(new Set<string>());
  const edited = useRef(new Set<string>());
  const restored = useRef(new Set<string>());
  const restoring = useRef(new Set<string>());
  function update(k: string, change: (old: Draft) => Draft) { setDrafts(old => ({ ...old, [k]: change(old[k] || empty) })); }
  const bindings = useMemo<InputBinding[]>(() => current.items.flatMap(x => x.material.status === "ready" && x.usage !== "rules" ? [{ input_id: x.material.id, content_hash: x.material.content_hash, usage: x.usage, ...(x.acknowledged ? { accept_partial: true } : {}) }] : []), [current.items]);
  const adoptions = useMemo<DocumentSource[]>(() => current.items.filter(x => x.usage === "rules" && x.material.status === "ready").map(x => ({ input_id: x.material.id, hash: x.material.content_hash, page: x.page, exact_quote: x.quote })), [current.items]);
  useEffect(() => {
    if (!session || restored.current.has(key)) return;
    let selected: { id: string; usage: Selection["usage"]; page: number; quote: string; acknowledged: boolean }[] = [];
    try { const value = JSON.parse(sessionStorage.getItem(`vibe-materials:${key}`) || "[]"); if (Array.isArray(value)) selected = value.slice(0, 2).filter(x => typeof x.id === "string" && ["task_input", "reference", "rules"].includes(x.usage) && typeof x.quote === "string" && x.quote.length <= 8000 && Number.isInteger(x.page)); } catch { /* Drafts are optional. */ }
    restored.current.add(key);
    if (!selected.length) return;
    restoring.current.add(key);
    update(key, old => ({ ...old, busy: true }));
    void token().then(auth => Promise.all(selected.map(async x => ({ ...x, material: await vibeFetch<TaskMaterial>(inputPath(session, x.id), auth) })))).then(items => { restoring.current.delete(key); update(key, old => ({ ...old, ...selectMaterials(old.items, edited.current.has(key) ? [] : items), busy: inFlight.current.has(key) })); }).catch(() => { restoring.current.delete(key); update(key, old => ({ ...old, busy: false, error: "Saved material could not be restored. Open Saved material to choose it again." })); });
  }, [key, session, token]);
  useEffect(() => {
    if (!session || !restored.current.has(key) || restoring.current.has(key) || current.busy) return;
    try { sessionStorage.setItem(`vibe-materials:${key}`, JSON.stringify(current.items.map(x => ({ id: x.material.id, usage: x.usage, page: x.page, quote: x.quote, acknowledged: x.acknowledged })))); } catch { /* In-memory drafts still work. */ }
  }, [session, key, current]);
  useEffect(() => {
    for (const { material } of current.items) if (["ready","failed","unreadable"].includes(material.status)) captureBuildEvent(WEB_EVENTS.VIBE_BUILD_INPUT_STATUS,{session_id:session,input_id:material.id,kind:material.kind,status:material.status},`${session}:${material.id}:${material.status}`);
  },[current.items,session]);
  const reading = current.items.some(x => x.material.status === "uploaded" || x.material.status === "extracting");
  useEffect(() => {
    if (!reading || !session) return;
    let live = true;
    let timer: ReturnType<typeof setTimeout>;
    async function poll() {
      try {
        const auth = await token();
        const values = await Promise.all(current.items.map(async x => x.material.status === "uploaded" || x.material.status === "extracting" ? { ...x, material: await vibeFetch<TaskMaterial>(inputPath(session, x.material.id), auth) } : x));
        if (live) update(key, old => ({ ...old, items: old.items.map(x => ({ ...x, material: values.find(v => v.material.id === x.material.id)?.material || x.material })), error: "" }));
      } catch { if (live) update(key, old => ({ ...old, error: "Reading status is unavailable. We’ll reconnect; your file is saved." })); }
      if (live) timer = setTimeout(poll, 2000);
    }
    timer = setTimeout(poll, 1000);
    return () => { live = false; clearTimeout(timer); };
  }, [reading, session, key, current.items, token]);
  async function add(value?: File | string) {
    if (!session || inFlight.current.has(key)) return;
    if (value instanceof File && value.size > 10_000_000) { update(key, old => ({ ...old, error: "Choose a PDF no larger than 10 MB." })); return; }
    const request = value === undefined ? pending.current.get(key) : { id: crypto.randomUUID(), value };
    if (!request) return;
    if (value !== undefined && (current.items.length >= 2 || typeof value !== "string" && current.items.some(x => x.material.kind === "pdf"))) { update(key, old => ({ ...old, error: "Use one PDF and optionally one pasted text input. Detach an input first." })); return; }
    pending.current.set(key, request); inFlight.current.add(key); edited.current.add(key);
    update(key, old => ({ ...old, busy: true, error: "" }));
    try {
      const material = await uploadMaterial(session, request.id, request.value, await token());
      update(key, old => ({ ...old, ...selectMaterials(old.items, [{ material, acknowledged: false, usage: "task_input" as const, page: 1, quote: "" }]) }));
      pending.current.delete(key);
    } catch (e) { update(key, old => ({ ...old, error: (e as Error).message })); }
    finally { inFlight.current.delete(key); update(key, old => ({ ...old, busy: false })); }
  }
  async function remove(id: string, erase = false) {
    edited.current.add(key);
    try {
      if (erase) await vibeFetch(inputPath(session, id), await token(), { method: "DELETE" });
      update(key, old => ({ ...old, items: old.items.filter(x => x.material.id !== id) }));
    } catch (e) { update(key, old => ({ ...old, error: (e as Error).message })); }
  }
  async function attach(id: string) {
    if (!session || inFlight.current.has(key)) return;
    edited.current.add(key); inFlight.current.add(key);
    update(key, old => ({ ...old, busy: true, error: "" }));
    try {
      const material = await vibeFetch<TaskMaterial>(inputPath(session, id), await token());
      update(key, old => ({ ...old, ...selectMaterials(old.items, [{ material, acknowledged: false, usage: "task_input" as const, page: 1, quote: "" }]) }));
    } catch (e) { update(key, old => ({ ...old, error: (e as Error).message })); }
    finally { inFlight.current.delete(key); update(key, old => ({ ...old, busy: restoring.current.has(key) })); }
  }
  return { ...current, canRetry: pending.current.has(key), bindings, adoptions, add, remove, attach, retry: () => add(),
    list: async () => vibeFetch<TaskMaterial[]>(inputPath(session), await token()),
    configure: (id: string, change: Partial<Pick<Selection, "usage" | "page" | "quote">>) => { edited.current.add(key); update(key, old => ({ ...old, items: old.items.map(x => x.material.id === id ? { ...x, ...change } : x) })); },
    blocked: current.busy || reading || current.items.some(x => x.material.status !== "ready" || x.material.warnings.length > 0 && !x.acknowledged || x.usage === "rules" && (!x.quote.trim() || !x.material.pages?.find(p => p.number === x.page)?.text.includes(x.quote))),
    acknowledge: (id: string, acknowledged: boolean) => update(key, old => ({ ...old, items: old.items.map(x => x.material.id === id ? { ...x, acknowledged } : x) })),
  };
}
