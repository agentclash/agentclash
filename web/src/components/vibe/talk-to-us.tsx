"use client";

import { useEffect, useId, useRef, useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { useVibeConnection } from "@/lib/vibe-connection";
import { vibeFetch } from "@/lib/vibe";
import { recoverRequest } from "@/lib/vibe-request-recovery";
import { contactEmailLink, vibeContact } from "@/lib/vibe-contact";
import { VibeButton } from "./vibe-button";
import "./project-enquiry.css";

const SUBJECT = "Agents for our team";

// Feature detection keeps the mail-app flow available with older API deployments.
export function TalkToUs({ open, onOpenChange, email }: { open: boolean; onOpenChange: (open: boolean) => void; email: string }) {
  const supportsDirect = vibeContact(useVibeConnection().contact).direct;
  const [mode, setMode] = useState({ open, direct: supportsDirect });
  // Capture capability on each opening; late config must not replace the form.
  if (mode.open !== open) setMode({ open, direct: open ? supportsDirect : mode.direct });
  const direct = mode.direct;
  const id = useId();
  const titleRef = useRef<HTMLHeadingElement>(null);
  const [work, setWork] = useState("");
  const [company, setCompany] = useState("");
  const [notice, setNotice] = useState("");
  const body = [work.trim(), company.trim() && `Company: ${company.trim()}`].filter(Boolean).join("\n\n");
  const full = contactEmailLink(email, SUBJECT, body);
  const plain = contactEmailLink(email, SUBJECT);
  const copy = async (text: string, done: string) => {
    try { await navigator.clipboard.writeText(text); setNotice(done); }
    catch { setNotice("Copy is not available here. Select the text and copy it yourself."); }
  };
  return <Dialog open={open} onOpenChange={next => { if (!next) setNotice(""); onOpenChange(next); }}>
    <DialogContent className="vibe-workspace vibe-dialog vibe-talk-dialog" data-direct={direct || undefined} initialFocus={titleRef}>
      <div className="vibe-talk-heading">
        <p className="vibe-talk-eyebrow">We set it up for you</p>
        <DialogTitle ref={titleRef} tabIndex={-1}>What takes your team the most time?</DialogTitle>
        <DialogDescription>We build agents for that work, test them on your real cases and keep them reliable.</DialogDescription>
      </div>
      {direct ? (open && <DirectContact email={email} work={work} setWork={setWork} company={company} setCompany={setCompany} onClose={() => onOpenChange(false)} />) : <div className="vibe-enquiry-form">
        <div className="vibe-enquiry-field">
          <label htmlFor={`${id}-work`}>What work should agents take on?</label>
          <textarea id={`${id}-work`} className="vibe-textarea" rows={4} maxLength={2000} value={work} onChange={event => setWork(event.target.value)} placeholder="For example: we answer about 200 return emails a day and copy each order into our order system by hand." />
        </div>
        <div className="vibe-enquiry-field">
          <label htmlFor={`${id}-company`}>Company (optional)</label>
          <input id={`${id}-company`} className="vibe-textarea" autoComplete="organization" maxLength={200} value={company} onChange={event => setCompany(event.target.value)} />
        </div>
        <a className="vibe-button vibe-button-brass vibe-talk-send" href={full || plain}>Email our team</a>
        <p id={`${id}-note`} className="vibe-talk-note">{full ? "Opens your email app with this text. You choose when to send it." : "Your text is too long for an email link. Copy it, then paste it into the email."}</p>
        {!full && <VibeButton variant="quiet" onClick={() => void copy(body, "Text copied")}>Copy your text</VibeButton>}
        <div className="vibe-talk-direct">
          <span>Or write to</span>
          <a href={`mailto:${email}`}>{email}</a>
          <VibeButton variant="quiet" onClick={() => void copy(email, "Email address copied")}>Copy</VibeButton>
          <span role="status" className="vibe-talk-status">{notice}</span>
        </div>
      </div>}
    </DialogContent>
  </Dialog>;
}

function DirectContact({ email, onClose, work, setWork, company, setCompany }: { email: string; onClose: () => void; work: string; setWork: (value: string) => void; company: string; setCompany: (value: string) => void }) {
  const { token } = useVibeConnection();
  const id = useId();
  const [clientID] = useState(() => crypto.randomUUID());
  const [replyEmail, setReplyEmail] = useState("");
  const [website, setWebsite] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [sentTo, setSentTo] = useState("");
  const receiptRef = useRef<HTMLParagraphElement>(null);
  const sending = useRef(false);
  const request = useRef<{ body: string; email: string; uncertain: boolean }>(undefined);
  useEffect(() => { if (sentTo) receiptRef.current?.focus(); }, [sentTo]);

  async function send(form: HTMLFormElement) {
    if (sending.current || sentTo) return;
    if (!request.current) {
      if (!work.trim()) { setError("Describe the work before sending."); return; }
      if (!replyEmail.trim() || !form.querySelector<HTMLInputElement>('input[type="email"]')!.validity.valid) { setError("Enter a valid email address before sending."); return; }
      if (new TextEncoder().encode(work.trim()).length > 4000 || new TextEncoder().encode(company).length > 200) {
        setError("Keep the work description within 4,000 bytes and the company within 200 bytes."); return;
      }
      request.current = { body: JSON.stringify({ client_id: clientID, email: replyEmail.trim(), work: work.trim(), company, website }), email: replyEmail.trim(), uncertain: false };
    }
    const captured = request.current;
    sending.current = true; setBusy(true); setError("");
    try {
      await vibeFetch("/enquiries", await token(), { method: "POST", body: captured.body });
      setSentTo(captured.email);
    } catch (e) {
      if (recoverRequest(captured, e) === "rejected") request.current = undefined;
      setError(e instanceof Error ? e.message : "Could not send your enquiry. Try again or email us directly.");
    } finally { sending.current = false; setBusy(false); }
  }

  return <div className="vibe-enquiry-form vibe-direct-contact">
    {sentTo ? <>
      <p ref={receiptRef} tabIndex={-1} role="status" className="vibe-enquiry-receipt">Sent. We will reply to {sentTo}.</p>
      <VibeButton onClick={onClose}>Close</VibeButton>
    </> : <form className="vibe-enquiry-form vibe-contact-form" noValidate onSubmit={event => { event.preventDefault(); void send(event.currentTarget); }}>
      <div className="vibe-enquiry-field">
        <label htmlFor={`${id}-work`}>What work should agents take on?</label>
        <textarea id={`${id}-work`} className="vibe-textarea" rows={4} required maxLength={4000} value={work} disabled={busy || !!request.current} onChange={event => setWork(event.target.value)} placeholder="For example: we answer about 200 return emails a day and copy each order into our order system by hand." />
      </div>
      <div className="vibe-talk-fields">
        <div className="vibe-enquiry-field">
          <label htmlFor={`${id}-email`}>Work email</label>
          <input id={`${id}-email`} className="vibe-textarea" type="email" autoComplete="email" required maxLength={254} value={replyEmail} disabled={busy || !!request.current} onChange={event => setReplyEmail(event.target.value)} placeholder="you@company.com" />
        </div>
        <div className="vibe-enquiry-field">
          <label htmlFor={`${id}-company`}>Company (optional)</label>
          <input id={`${id}-company`} className="vibe-textarea" autoComplete="organization" maxLength={200} value={company} disabled={busy || !!request.current} onChange={event => setCompany(event.target.value)} />
        </div>
      </div>
      <input className="vibe-contact-honeypot" name="website" tabIndex={-1} autoComplete="off" aria-hidden="true" value={website} onChange={event => setWebsite(event.target.value)} />
      <VibeButton type="submit" className="vibe-button-brass vibe-talk-send" disabled={busy} loading={busy}>Send to our team</VibeButton>
      {error && <p role="alert" className="vibe-enquiry-error">{error}</p>}
    </form>}
    <div className="vibe-talk-direct"><span>Or write to</span><a href={`mailto:${email}`}>{email}</a></div>
  </div>;
}
