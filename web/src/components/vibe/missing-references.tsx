"use client";

import { useRef, useState } from "react";
import type { useVibeInputs } from "@/lib/use-vibe-inputs";
import { vibeFetch, type Artifact, type Session } from "@/lib/vibe";
import { VibeButton } from "./vibe-button";

// Upload/selection remains in the existing material controller. This view only
// binds a selected file to a missing requirement; it never runs the agent.
export function MissingReferences({session, artifact, inputs, busy, token, onBoundSession}: {
  session: Session; artifact: Artifact; inputs: ReturnType<typeof useVibeInputs>; busy: boolean;
  token: () => Promise<string | null | undefined>; onBoundSession: (session: Session, artifactID: string) => void;
}) {
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const pending = useRef(false);
  const missing = artifact.missing_references || [];
  if (!missing.length) return null;
  async function bind(key: string, inputID: string) {
    if (pending.current) return;
    const selected = inputs.items.find(item => item.material.id === inputID);
    if (!selected) return;
    pending.current = true; setWorking(true); setError("");
    try {
      const incoming = await vibeFetch<Session>(`/sessions/${session.id}/references`, await token(), {
        method: "POST", body: JSON.stringify({revision: session.revision, artifact_id: artifact.id, key,
          input: {input_id: inputID, content_hash: selected.material.content_hash, usage: "reference", accept_partial: selected.acknowledged}}),
      });
      onBoundSession(incoming, incoming.document.active_artifact_id || artifact.id);
    } catch (issue) {
      // A lost response can still have published the revision. Read the same
      // project before offering a retry; never create a second binding blindly.
      try {
        const current = await vibeFetch<Session>(`/sessions/${session.id}`, await token());
        const attached = current.document.artifacts.find(version => version.parent_id === artifact.id &&
          !version.missing_references?.some(reference => reference.key === key) &&
          version.reference_inputs?.some(binding => binding.input_id === inputID));
        if (attached) { onBoundSession(current, attached.id); return; }
      } catch { /* Keep the original error and definition for a later retry. */ }
      setError(issue instanceof Error ? issue.message : "Could not attach the reference. Your imported tests are preserved.");
    } finally {pending.current = false; setWorking(false);}
  }
  return <section className="vibe-panel space-y-3 p-4 text-sm" aria-label="Missing references">
    <p><strong>Attach the original references before trying this agent.</strong></p>
    <p className="vibe-muted">The download includes instructions and tests, without private files. Add the original file or text below, then attach it here. Nothing runs during this step.</p>
    {missing.map(reference => {
      const matching = inputs.items.filter(item => item.material.status === "ready" && item.material.content_hash === reference.content_hash && (!item.material.warnings.length || item.acknowledged));
      return <div key={reference.key} className="flex flex-wrap items-center gap-2">
        <span>{reference.name || reference.key}{reference.format ? ` · ${reference.format.toUpperCase()}` : ""}</span>
        {matching.length ? matching.map(item => <VibeButton key={item.material.id} disabled={busy || working || inputs.busy} onClick={() => void bind(reference.key, item.material.id)}>Attach {item.material.name}</VibeButton>) : <span className="vibe-muted">Waiting for matching material</span>}
      </div>;
    })}
    {working && <p role="status">Attaching reference…</p>}
    {error && <p role="alert" className="text-builder-warn">{error}</p>}
    <p className="vibe-muted">A different document changes the agent’s requirements. It cannot count as an improvement against the original checks.</p>
  </section>;
}
