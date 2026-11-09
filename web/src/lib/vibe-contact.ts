// The AgentClash team address. The API can set another address (VIBE_ENQUIRY_EMAIL). Without
// one, people can still email the team, so the contact call to action never disappears.
export const VIBE_CONTACT_EMAIL = "hello@tryagentclash.com";

export type VibeContact = { email: string; available: boolean; direct: boolean };

/** The contact that the UI shows: the API form when it is available, and always an email address. */
export function vibeContact(contact?: { email?: string; available?: boolean; direct?: boolean } | null): VibeContact {
  return { direct: contact?.direct === true && contact?.available === true, available: !!contact?.available, email: contact?.email?.trim() || VIBE_CONTACT_EMAIL };
}

// Mail apps and browsers drop very long mailto links, so a long body falls back to the address only.
const MAX_MAILTO = 1800;

/** A mailto link with a subject and a body, or undefined when the link would be too long. */
export function contactEmailLink(email: string, subject: string, body = "") {
  if (!email) return undefined;
  const query = [`subject=${encodeURIComponent(subject)}`, ...(body ? [`body=${encodeURIComponent(body)}`] : [])].join("&");
  const href = `mailto:${encodeURIComponent(email)}?${query}`;
  return href.length <= MAX_MAILTO ? href : undefined;
}
