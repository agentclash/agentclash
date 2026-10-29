"use client";

import { useRef, useState } from "react";
import { vibeFetch, type Artifact, type Session } from "@/lib/vibe";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";
import { VibeButton } from "./vibe-button";

type Review = { review_hash: string; partial: boolean; applied: boolean; changes: { path: string; kind: string; before: unknown; after: unknown }[] };

export function ReimportReview({ actor, session, artifact, token, disabled, onApplied }: {
  actor: string; session: Session; artifact: Artifact; disabled: boolean;
  token: () => Promise<string | undefined>;
  onApplied: (session: Session) => void;
}) {
  const life = useVibeEvidenceOwner(actor, session.id, artifact.id, String(session.revision));
  const [state, setState] = useState<{ owner: object; content: string; review?: Review; busy: boolean; error?: string }>();
  const input = useRef<HTMLInputElement>(null);
  const active = state?.owner === life.owner ? state : undefined;
  async function review(file?: File) {
    if (!file || disabled || active?.busy) return;
    const current = life.capture();
    setState({ owner: life.owner, content: "", busy: true });
    try {
      if (file.size > (session.anonymous ? 256 * 1024 : 1024 * 1024)) throw new Error("This file exceeds the import size limit.");
      const content = await file.text();
      const auth = await token();
      if (!current()) return;
      const result = await vibeFetch<Review>(`/sessions/${session.id}/reimport`, auth, {
        method: "POST", body: JSON.stringify({ revision: session.revision, artifact_id: artifact.id, content }),
      });
      if (current()) setState({ owner: life.owner, content, review: result, busy: false });
    } catch (error) {
      if (current()) setState({ owner: life.owner, content: "", busy: false, error: (error as Error).message });
    } finally { if (input.current) input.current.value = ""; }
  }
  async function apply() {
    if (!active?.review || active.busy || disabled) return;
    const current = life.capture(), selected = active;
    setState({ ...selected, busy: true, error: undefined });
    try {
      const auth = await token();
      if (!current()) return;
      await vibeFetch(`/sessions/${session.id}/reimport`, auth, { method: "POST", body: JSON.stringify({ revision: session.revision, artifact_id: artifact.id, content: selected.content, apply: true, review_hash: selected.review!.review_hash }) });
      const updated = await vibeFetch<Session>(`/sessions/${session.id}`, auth);
      if (current()) { setState(undefined); onApplied(updated); }
    } catch (error) {
      if (current()) setState({ ...selected, busy: false, error: (error as Error).message });
    }
  }
  return <section aria-label="Review reimport" className="space-y-3">
    <h3>Review an updated export</h3>
    <p className="text-sm vibe-muted">Compare with version {artifact.id.slice(0, 8)}. Creating a new version keeps earlier tests and grades. Publish it separately, then choose a completed run as your CI baseline.</p>
    <input ref={input} type="file" accept=".json" aria-label="Updated evaluation export" disabled={disabled || active?.busy} onChange={event => void review(event.target.files?.[0])} className="w-full min-w-0" />
    {active?.busy && <p role="status">Preparing this exact version…</p>}
    {active?.error && <p role="alert">{active.error}</p>}
    {active?.review && <>
      {active.review.partial && <p>Partial evidence: missing items stay missing. Imported scores are historical claims, not new execution or a complete baseline.</p>}
      <p>{active.review.changes.length ? `${active.review.changes.length} semantic changes` : "No semantic changes to tests or instructions."}</p>
      <ul className="space-y-3">{active.review.changes.map(change => <li key={change.path} className="min-w-0 rounded-[var(--vibe-radius-sm)] border border-current/20 p-2 text-sm [overflow-wrap:anywhere]">
        <p><strong>{change.kind}</strong> · {change.path}</p>
        <p>Before</p><pre className="whitespace-pre-wrap break-words">{JSON.stringify(change.before, null, 2)}</pre>
        <p>After</p><pre className="whitespace-pre-wrap break-words">{JSON.stringify(change.after, null, 2)}</pre>
      </li>)}</ul>
      <div className="flex flex-wrap gap-2">
        <VibeButton disabled={disabled || active.busy} onClick={() => void apply()}>Accept changes as a new version</VibeButton>
        <VibeButton disabled={active.busy} variant="quiet" onClick={() => { setState(undefined); input.current?.focus(); }}>Cancel reimport</VibeButton>
      </div>
    </>}
  </section>;
}
