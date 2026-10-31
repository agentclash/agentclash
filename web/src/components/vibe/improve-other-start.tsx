"use client";
import { useEffect, useRef, useState } from "react";
import type { Session } from "@/lib/vibe";
import { projectTitle } from "@/lib/vibe-project";
import { VibeButton } from "./vibe-button";
import { VibeDataUseNotice } from "./vibe-data-use";
import { sendOnEnter } from "./composer-keyboard";

export type ImproveSource = "chat" | "live_app" | "sdk_report";
export function ImproveOtherStart(p: { source: ImproveSource; projects: Session[]; workspace: string; loading: boolean; failed: boolean; busy: boolean; draft: string; onDraft(text: string): void; onSend(): void; onProject(id: string): void; onRetry(): void; onBack(): void; blockedReason?: string; error?: string }) {
  const heading = useRef<HTMLHeadingElement>(null);
  const [project, setProject] = useState("");
  useEffect(() => { heading.current?.focus(); }, []);
  const projects = p.projects.filter(row => row.workspace_id === p.workspace && row.document.artifacts.some(artifact => artifact.kind === "test_suite"));
  const chat = p.source === "chat";
  const send = () => { if (!p.busy && !p.blockedReason && p.draft.trim()) p.onSend(); };
  return <section className="vibe-start-page vibe-improve-start" onKeyDown={event => { if (event.key === "Escape" && !p.busy) p.onBack(); }}>
    <header><VibeButton variant="quiet" disabled={p.busy} onClick={p.onBack}>Back</VibeButton></header>
    <h1 ref={heading} tabIndex={-1}>{chat ? "Describe your agent" : p.source === "live_app" ? "Connect a live app" : "Import an SDK report"}</h1>
    {chat ? <form className="vibe-improve-form" onSubmit={event => { event.preventDefault(); send(); }}>
      <label>Message Guide<textarea aria-label="Message Guide" className="vibe-textarea" rows={4} value={p.draft} disabled={p.busy} onChange={event => p.onDraft(event.target.value)} onKeyDown={event => sendOnEnter(event, send)} /></label>
      <p className="vibe-muted">Describe its job and rules. Guide helps you choose what to test.</p>
      <VibeButton type="submit" aria-label="Send message" variant="primary" loading={p.busy} disabled={p.busy || !!p.blockedReason || !p.draft.trim()}>Send message</VibeButton>
      {p.blockedReason && <p role="status">{p.blockedReason}</p>}
      <VibeDataUseNotice />
    </form> : <div className="vibe-improve-form">
      <p>{p.source === "live_app" ? "Choose a project with tests. Keep and publish the tests before connecting your app." : "Choose a project with tests to bind the report to the right test version."}</p>
      {p.loading ? <p role="status">Loading projects…</p> : p.failed ? <><p role="alert">Couldn’t load your projects.</p><VibeButton onClick={p.onRetry}>Retry projects</VibeButton></> : <>
        <label>Project with tests<select aria-label="Project with tests" disabled={p.busy} value={project} onChange={event => setProject(event.target.value)}><option value="">Choose a project</option>{projects.map(row => <option value={row.id} key={row.id}>{projectTitle(row)}</option>)}</select></label>
        {!projects.length && <p>No projects with tests in this workspace yet. Go back to write or import tests first.</p>}
        <VibeButton variant="primary" disabled={p.busy || !projects.some(row => row.id === project)} loading={p.busy} onClick={() => p.onProject(project)}>Open project</VibeButton>
      </>}
    </div>}
    {p.error && <p role="alert">{p.error}</p>}
  </section>;
}
