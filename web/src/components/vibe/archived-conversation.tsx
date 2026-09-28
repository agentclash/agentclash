"use client";

import { useRef, useState, type ReactNode } from "react";
import type { Session, CaseResult } from "@/lib/vibe";
import { CaseEvidence } from "./case-evidence";
import { SafeMarkdown } from "./safe-markdown";
import { VibeButton } from "./vibe-button";

export function ArchivedConversation({ session, navigation, busy, error, onContinue, onExport, loadEvidence }: {
  session: Session;
  navigation: ReactNode;
  busy: boolean;
  error: string;
  onContinue: (artifactID: string, clientID: string) => Promise<void>;
  onExport: () => void;
  loadEvidence: (id: string, key: string) => Promise<CaseResult>;
}) {
  const packs = session.document.artifacts.filter(a => a.kind === "test_suite");
  const [selected, setSelected] = useState(packs.at(-1)?.id || "");
  const request = useRef<{ artifact: string; id: string } | null>(null);
  return <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
    <header className="flex items-center gap-3 border-b border-white/10 p-4">{navigation}<h1 className="font-medium">Saved conversation</h1></header>
    <div className="min-h-0 flex-1 overflow-y-auto">
      <div className="vibe-column space-y-6 py-8">
        <p className="vibe-muted">This earlier conversation is read-only. Keep its history here, or copy a version into V1 to continue.</p>
        {packs.length > 0 && <div className="space-y-3">
          <label className="block text-sm" htmlFor="archive-version">Version to continue</label>
          <select id="archive-version" value={selected} disabled={busy} className="w-full rounded-xl border border-white/15 bg-transparent p-3" onChange={e => setSelected(e.target.value)}>
            {packs.map((a, i) => <option key={a.id} value={a.id}>{a.title} · version {i + 1}</option>)}
          </select>
          <p className="text-sm vibe-muted">Copies these instructions and tests. Earlier scores stay here; the copy needs a new run. Recorded conversations and planning drafts can be exported.</p>
          <VibeButton variant="primary" disabled={busy || !selected} onClick={() => {
            if (request.current?.artifact !== selected) request.current = { artifact: selected, id: crypto.randomUUID() };
            void onContinue(selected, request.current.id);
          }}>{busy ? "Copying…" : "Continue in V1"}</VibeButton>
        </div>}
        <VibeButton onClick={onExport}>Export saved work</VibeButton>
        {error && <p role="alert">{error}</p>}
        {session.document.messages.map(message => <article key={message.id} className="border-t border-white/10 pt-5">
          <p className="mb-2 text-sm vibe-muted">{message.role === "user" ? "You" : "Vibe Evals"}</p>
          <SafeMarkdown>{message.content}</SafeMarkdown>
        </article>)}
        {session.operations.filter(o => o.scorecard).map(o => <details key={o.id} className="rounded-xl border border-white/10 p-4">
          <summary>Saved results · {o.scorecard?.passed} passed · {o.scorecard?.failed} failed</summary>
          <div className="mt-3">{o.results.map(result => <CaseEvidence key={result.case_key} summary={result} load={key => loadEvidence(o.id, key)} evidenceVersion={o.id} origin={o.source?.kind} />)}</div>
        </details>)}
      </div>
    </div>
  </section>;
}
