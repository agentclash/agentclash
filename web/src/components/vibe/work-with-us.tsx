"use client";

import { useId, type ReactNode } from "react";
import { useVibeConnection } from "@/lib/vibe-connection";
import { VIBE_CONTACT_EMAIL } from "@/lib/vibe-contact";
import "./project-enquiry.css";

// The agency offer under a finished result: after people see value, not before. The action
// (the enquiry button) comes from the caller; the team address is always in view under it.
export function WorkWithUs({ children }: { children: ReactNode }) {
  const id = useId();
  const email = useVibeConnection().contact?.email || VIBE_CONTACT_EMAIL;
  return <div className="vibe-work-with-us" role="group" aria-labelledby={id}>
    <div className="vibe-work-with-us-copy">
      <p className="vibe-talk-eyebrow">Work with us</p>
      <h3 id={id}>Want this agent live for your customers?</h3>
      <p>We connect it to your systems and keep it tested.</p>
    </div>
    <div className="vibe-work-with-us-actions">
      {children}
      <p className="vibe-work-with-us-email">or email <a href={`mailto:${email}`}>{email}</a></p>
    </div>
  </div>;
}
