"use client";
import { useEffect, useRef, useState } from "react";
import { useAccessToken, useAuth } from "@workos-inc/authkit-nextjs/components";
import Link from "next/link";
import { vibeFetch } from "@/lib/vibe";
import { pinVibeReturn } from "@/lib/vibe-keep";
import type { ProjectResultBinding } from "@/lib/vibe-result-evidence";
import { useVibeEvidenceOwner } from "@/lib/use-vibe-evidence-owner";

export function CanonicalProjectReturn({ runId, workspaceId }: { runId: string; workspaceId: string }) {
  const { user } = useAuth(), { getAccessToken } = useAccessToken();
  const tokenRef = useRef(getAccessToken);
  useEffect(() => { tokenRef.current = getAccessToken; }, [getAccessToken]);
  const actor = user?.id;
  const life = useVibeEvidenceOwner(user?.id, workspaceId, runId);
  const [state, setState] = useState<{ owner: object; rows: ProjectResultBinding[]; error?: string }>();
  const [attempt, retry] = useState(0);
  useEffect(() => {
    const current = life.capture();
    if (!actor) return;
    void (async () => {
      try {
        const token = await tokenRef.current();
        if (!current() || !token) return;
        const links = await vibeFetch<{ binding: ProjectResultBinding; url: string }[]>(`/canonical-results/run/${encodeURIComponent(runId)}/projects`, token);
        if (current()) setState({ owner: life.owner, rows: links.map(v => v.binding).filter(b => b.result_ref.kind === "run" && b.result_ref.id === runId && !!b.session_id && !!b.artifact_id) });
      } catch {
        if (current()) setState({ owner: life.owner, rows: [], error: "Project return links are unavailable." });
      }
    })();
  }, [life, runId, actor, attempt]);
  const active = state?.owner === life.owner ? state : undefined;
  if (!active?.rows.length && !active?.error) return null;
  return <aside aria-label="Originating Vibe projects" className="flex flex-wrap items-center gap-3 text-sm">
    {active?.rows.map(b => <Link className="underline" key={`${b.session_id}:${b.artifact_id}`} href={pinVibeReturn("/vibe-evals?view=results", b.session_id, b.artifact_id, `run:${runId}`)}>Return to Vibe project · {b.artifact_id.slice(0, 8)}</Link>)}
    {active?.error && <><span>{active.error}</span><button type="button" onClick={() => retry(v => v + 1)}>Retry project links</button></>}
  </aside>;
}
