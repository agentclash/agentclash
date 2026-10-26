"use client";
import { useRef, useState } from "react";
import { vibeFetch, vibeBytes, type Session, type Artifact, type Operation } from "./vibe";
import {
  newExportJob,
  collectExport,
  exportDocument,
  exportFilename,
  downloadJSON,
  downloadBytes,
  bytesBase64,
  type ExportJob,
  type ExportPreparation,
  type ExportItem,
} from "./vibe-export";

export function useVibeExport(
  actor: string,
  session: Session | null,
  token: () => Promise<string | undefined>,
) {
  const owner = useRef({ actor, session: session?.id });
  if (owner.current.actor !== actor || owner.current.session !== session?.id)
    owner.current = { actor, session: session?.id };
  const generation = useRef(0);
  const [state, setState] = useState<{
    owner: object;
    job?: ExportJob;
    busy: boolean;
    error: string;
    open: boolean;
    intent?: { artifact: Artifact; operation?: Operation };
  }>();
  const renderedGeneration = generation.current;
  const active = state?.owner === owner.current ? state : undefined;
  function close() {
    generation.current++;
    setState(undefined);
  }
  async function start(
    artifact: Artifact,
    operation?: Operation,
    scope: "selected" | "history" = "selected",
  ) {
    if (!session) return;
    const captured = owner.current,
      attempt = ++generation.current;
    const current = () => captured === owner.current && attempt === generation.current;
    const intent = structuredClone({ artifact, operation });
    setState({ owner: captured, busy: true, error: "", open: true, intent });
    try {
      const auth = await token();
      if (!current()) return;
      const p = await vibeFetch<ExportPreparation>(`/sessions/${session.id}/export`, auth, {
        method: "POST",
        body: JSON.stringify({
          scope,
          artifact_id: artifact.id,
          ...(operation ? { result_id: operation.id } : {}),
        }),
      });
      if (!current()) return;
      if (
        p.session_id !== session.id ||
        p.artifact_id !== artifact.id ||
        p.selection.artifact_id !== artifact.id ||
        p.selection.scope !== scope ||
        p.selection.result_id !== operation?.id
      )
        throw new Error("Export preparation changed the selected identity.");
      const job = newExportJob(p);
      setState({ owner: captured, job, busy: true, error: "", open: true, intent });
      await fill(job, auth, current, () =>
        setState((old) => (current() && old ? { ...old, job } : old)),
      );
      if (current()) setState((old) => (old ? { ...old, busy: false } : old));
    } catch (e) {
      if (current())
        setState((old) => (old ? { ...old, busy: false, error: (e as Error).message } : old));
    }
  }
  async function fill(
    job: ExportJob,
    auth: string | undefined,
    current: () => boolean,
    progress: () => void,
  ) {
    await collectExport(
      job,
      async (item: ExportItem) => {
        const base = `/sessions/${job.preparation.session_id}`;
        if (item.kind === "case")
          return vibeFetch(
            `/operations/${item.operation_id}/case?key=${encodeURIComponent(item.case_key!)}&hash=${item.hash}`,
            auth,
          );
        if (item.kind === "original") {
          const bytes = await vibeBytes(`${base}/artifacts/${item.artifact_id}/original`, auth);
          const digest = await crypto.subtle.digest("SHA-256", new Uint8Array(bytes).buffer);
          const hash = Array.from(new Uint8Array(digest), (b) =>
            b.toString(16).padStart(2, "0"),
          ).join("");
          if (hash !== item.hash) throw new Error("Original file hash changed.");
          return { encoding: "base64", content: bytesBase64(bytes), content_sha256: hash };
        }
        return vibeFetch(
          `${base}/export/reviews/${item.id}?kind=${item.kind}&hash=${item.hash}`,
          auth,
        );
      },
      current,
      progress,
    );
  }
  async function retry() {
    if (!active?.job || active.busy) return;
    const captured = owner.current,
      attempt = generation.current,
      job = active.job;
    const current = () => captured === owner.current && attempt === generation.current;
    setState((old) => (old ? { ...old, busy: true } : old));
    try {
      const auth = await token();
      if (!current()) return;
      await fill(job, auth, current, () =>
        setState((old) => (current() && old ? { ...old, job } : old)),
      );
    } catch (e) {
      if (current()) setState((old) => (old ? { ...old, error: (e as Error).message } : old));
    } finally {
      if (current()) setState((old) => (old ? { ...old, busy: false } : old));
    }
  }
  function download(partial = false) {
    if (
      !active?.job ||
      active.busy ||
      active.owner !== owner.current ||
      renderedGeneration !== generation.current
    )
      return;
    downloadJSON(
      exportFilename(active.job.preparation, active.job.missing.size > 0),
      exportDocument(active.job, partial),
    );
  }
  async function original(artifact: Artifact) {
    if (!session || !artifact.original) return;
    const captured = owner.current;
    try {
      const auth = await token();
      if (captured !== owner.current) return;
      const bytes = await vibeBytes(
        `/sessions/${session.id}/artifacts/${artifact.id}/original`,
        auth,
      );
      if (captured !== owner.current) return;
      downloadBytes(artifact.original.filename, bytes, artifact.original.media_type);
    } catch (e) {
      if (captured === owner.current)
        setState({ owner: captured, open: true, busy: false, error: (e as Error).message });
    }
  }
  return { state: active, start, close, retry, download, original };
}
