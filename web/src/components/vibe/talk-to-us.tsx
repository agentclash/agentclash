"use client";

import { useId, useRef, useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { contactEmailLink } from "@/lib/vibe-contact";
import { VibeButton } from "./vibe-button";
import "./project-enquiry.css";

const SUBJECT = "Agents for our team";

// The agency path. It needs no project, so it writes an email in the visitor's own mail app:
// nothing is sent until they send it, and nothing is stored here.
export function TalkToUs({ open, onOpenChange, email }: { open: boolean; onOpenChange: (open: boolean) => void; email: string }) {
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
    <DialogContent className="vibe-workspace vibe-dialog vibe-talk-dialog" initialFocus={titleRef}>
      <div className="vibe-talk-heading">
        <p className="vibe-talk-eyebrow">We set it up for you</p>
        <DialogTitle ref={titleRef} tabIndex={-1}>What takes your team the most time?</DialogTitle>
        <DialogDescription>We build agents for that work, test them on your real cases and keep them reliable.</DialogDescription>
      </div>
      <div className="vibe-enquiry-form">
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
      </div>
    </DialogContent>
  </Dialog>;
}
