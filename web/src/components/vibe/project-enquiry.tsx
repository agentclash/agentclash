"use client";

import { useEffect, useId, useRef, useState } from "react";
import { VibeError, vibeFetch, type Artifact, type Operation, type Session } from "@/lib/vibe";
import { useVibeConnection } from "@/lib/vibe-connection";
import { enquiryEmailLink, projectSummary, type EnquiryReceipt } from "@/lib/vibe-enquiries";
import { WEB_EVENTS } from "@/lib/analytics/events";
import { captureBuildEvent } from "@/lib/vibe-build-analytics";
import { VibeButton } from "./vibe-button";

export function ProjectEnquiry({ session, artifact, operation, primary = false }: { session: Session; artifact: Artifact; operation?: Operation; primary?: boolean }) {
  const { token, contact } = useVibeConnection();
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(() => ({ summary: projectSummary(session, artifact, operation), email: "", name: "", company: "" }));
  const source = useRef({ artifact_id: artifact.id, operation_id: operation?.id, revision: session.revision });
  const opened = useRef(false);
  function refresh() { source.current = { artifact_id: artifact.id, operation_id: operation?.id, revision: session.revision }; edit({ ...draft, summary: projectSummary(session,artifact,operation) }); }
  const request = useRef<{ session: string; body: string; client: string } | undefined>(undefined);
  const storageKey = `vibe-enquiry:${session.id}:${artifact.id}`;
  const sending = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [receipt, setReceipt] = useState<EnquiryReceipt>();
  const id = useId();
  function edit(next: typeof draft) {
    setDraft(next);
    try { sessionStorage.setItem(storageKey, JSON.stringify({ ...next, source: source.current })); }
    catch { setError("Your edits are available here, but this browser couldn't keep the draft after a reload."); }
  }
  useEffect(() => {
    // Persist the exact submitted body before sending, so a reload after a lost
    // acknowledgement recovers its receipt instead of creating another enquiry.
    try {
      const saved = sessionStorage.getItem(storageKey);
      if (!saved) return;
      const body = JSON.parse(saved);
      if (typeof body.summary !== "string" || typeof body.email !== "string" || typeof body.name !== "string" || typeof body.company !== "string" || body.source?.artifact_id !== artifact.id || !Number.isInteger(body.source.revision)) return;
      if (typeof body.client_id === "string") request.current = {session: session.id, body: saved, client: body.client_id};
      source.current = body.source;
      setDraft({summary:body.summary, email:body.email, name:body.name || "", company:body.company || ""});
      opened.current = true;
    } catch { /* A fresh form remains available if storage cannot be read. */ }
  }, [storageKey, session.id, artifact.id]);
  const eventBase = {session_id:session.id, artifact_id:artifact.id, operation_id:operation?.id};
  function received(value:EnquiryReceipt) { setReceipt(value); if(session.document.evaluation?.door==="build") captureBuildEvent(WEB_EVENTS.VIBE_BUILD_ENQUIRY_RECEIVED,{...eventBase,enquiry_id:value.id},value.id); }
  const link = enquiryEmailLink(contact?.email || "", draft.summary);
  async function send() {
    if (sending.current || receipt || !contact?.available) return;
    if (!request.current) {
      const client = crypto.randomUUID();
      const body = JSON.stringify({ ...draft, client_id: client, source: source.current });
      try { sessionStorage.setItem(storageKey, body); }
      catch { setError("Your browser couldn't save the enquiry for safe retry. Copy the summary and use direct email, or enable site storage."); return; }
      request.current = { session: session.id, client, body };
    }
    const captured = request.current;
    sending.current = true; setBusy(true); setError("");
    try { received(await vibeFetch<EnquiryReceipt>(`/sessions/${captured.session}/enquiries`, await token(), { method: "POST", body: captured.body })); }
    catch (e) {
      if (e instanceof VibeError && e.status && e.status < 500 && e.code !== "idempotency_conflict") { request.current=undefined; try { sessionStorage.removeItem(storageKey); } catch { /* The form still shows the validation error. */ } setError(e.message); }
      else setError(`${(e as Error).message} Retry uses the same enquiry; it won’t submit another one.`);
    }
    finally { sending.current = false; setBusy(false); }
  }
  async function status() {
    if (!request.current) return;
    try { received(await vibeFetch<EnquiryReceipt>(`/sessions/${request.current.session}/enquiries/${request.current.client}`, await token())); } catch { setError("Status is temporarily unavailable. Your original enquiry is preserved."); }
  }
  return <section className="vibe-project-enquiry">
    <VibeButton variant={primary ? "primary" : "quiet"} onClick={() => { if (!opened.current) { refresh(); opened.current=true; } if (!open && session.document.evaluation?.door==="build") captureBuildEvent(WEB_EVENTS.VIBE_BUILD_ENQUIRY_OPENED,eventBase); setOpen(!open); }}>Discuss this with AgentClash</VibeButton>
    {open && <form className="vibe-panel p-4 space-y-3" onSubmit={e => { e.preventDefault(); void send(); }}>
      <h3>Let’s discuss building this for your business</h3>
      <p className="text-sm vibe-muted">Review what you’ll share. Files, raw replies and chat history are not attached. Downloads and improvements stay available here.</p>
      <label className="block" htmlFor={`${id}-summary`}>Project summary</label>
      {!request.current && <VibeButton variant="quiet" onClick={refresh}>Refresh from this version</VibeButton>}
      <textarea id={`${id}-summary`} className="vibe-textarea w-full min-h-48" value={draft.summary} disabled={!!request.current} maxLength={12000} onChange={e => edit({ ...draft, summary: e.target.value })} required />
      <label className="block" htmlFor={`${id}-email`}>Your email</label><input id={`${id}-email`} className="vibe-textarea w-full" type="email" autoComplete="email" required maxLength={254} value={draft.email} disabled={!!request.current} onChange={e => edit({ ...draft, email: e.target.value })} />
      <div className="flex flex-wrap gap-3"><label>Name (optional)<input className="vibe-textarea block" autoComplete="name" maxLength={100} disabled={!!request.current} value={draft.name} onChange={e => edit({ ...draft, name: e.target.value })} /></label><label>Company (optional)<input className="vibe-textarea block" autoComplete="organization" maxLength={200} disabled={!!request.current} value={draft.company} onChange={e => edit({ ...draft, company: e.target.value })} /></label></div>
      {contact?.available ? <VibeButton variant="primary" type="submit" disabled={busy || !!receipt}>{busy ? "Saving enquiry…" : request.current && !receipt ? "Retry enquiry" : "Send enquiry"}</VibeButton> : <p role="status">Contact isn’t set up yet. You can copy your summary.</p>}
      {request.current && !receipt && !busy && <VibeButton variant="quiet" onClick={() => void status()}>Check original enquiry status</VibeButton>}
      {receipt && <div role="status"><p>{receipt.status === "provider_accepted" ? "Our email provider accepted the notification. This doesn’t confirm inbox delivery." : receipt.status === "needs_review" ? "Enquiry received. Its notification needs our team’s attention." : receipt.status === "cancelled" ? "Notification cancelled." : "Enquiry received. The team notification is queued."}</p><VibeButton variant="quiet" onClick={() => void status()}>Refresh status</VibeButton></div>}
      {contact?.email && <div>{link ? <a href={link} onClick={() => { if(session.document.evaluation?.door==="build") captureBuildEvent(WEB_EVENTS.VIBE_BUILD_EMAIL_DRAFT_OPENED,eventBase); }}>Prefer email? Email us directly</a> : <p>Prefer email? <a href={`mailto:${encodeURIComponent(contact.email)}`}>Email us directly</a>. Copy and paste your summary into the draft; it is too long to include in the link.</p>}<p className="text-xs vibe-muted">Opens your email app; you still choose whether to send.</p></div>}
      <VibeButton variant="quiet" onClick={() => void navigator.clipboard.writeText(draft.summary).then(() => setNotice("Summary copied")).catch(() => setError("Select and copy the summary above."))}>Copy summary</VibeButton>
      {contact?.email && <VibeButton variant="quiet" onClick={() => void navigator.clipboard.writeText(contact.email).then(() => setNotice("Email address copied")).catch(() => setError("Copy the email address from the link."))}>Copy email address</VibeButton>}
      {notice && <p role="status">{notice}</p>}{error && <p role="alert">{error}</p>}
    </form>}
  </section>;
}
