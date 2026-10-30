"use client";
import { useState } from "react";
import { Copy } from "lucide-react";
import { VibeButton } from "./vibe-button";
export const exampleAgentDescription = `My agent helps customers with returns. Unopened items can be returned within 30 days. Opened items and purchases older than 30 days aren't eligible. It should ask for missing purchase age or item condition, and never claim it has processed a refund.`;
export const instructionsHelp = "Show me the exact instructions used by the AI agent in my app, and which model it uses. Include its existing rules. Do not rewrite them. I want to test those instructions in Vibe Evals.";
export const repliesPlaceholder = "Customer: Can I return an opened item?\nAgent: I can refund that.\n---\nCustomer: My order arrived yesterday.\nAgent: Is the item unopened?";
export function replyCount(source: string) {
  const text = source.trim();
  if (!text) return { conversations: 0, messages: 0 };
  if (/^[{[]/.test(text)) {
    try {
      const value = JSON.parse(text);
      const chats = Array.isArray(value) ? [{ messages: value }] : value.conversations || [{ messages: value.messages }];
      if (!Array.isArray(chats) || chats.some(chat => !Array.isArray(chat.messages))) return { conversations: 0, messages: 0 };
      return { conversations: chats.length, messages: chats.reduce((count, chat) => count + chat.messages.length, 0) };
    } catch { return { conversations: 0, messages: 0 }; }
  }
  const chats = text.split(/^---[\t ]*\r?$/m).filter(part => part.trim());
  const speaker = /^[\t ]*(?:\*\*|__)?(?:customer|user|human|agent|assistant|bot|system|tool)(?:\*\*|__)?[\t ]*:(?:\*\*|__)?[\t ]?/gim;
  return { conversations: chats.length, messages: chats.reduce((count, chat) => {
    const matches = [...chat.matchAll(speaker)];
    return count + Math.max(1, matches.length) + (matches.length && chat.slice(0, matches[0].index).trim() ? 1 : 0);
  }, 0) };
}
export function ReplyCount({ text }: { text: string }) {
  const count = replyCount(text);
  return <p className="vibe-improve-count" role="status">{count.conversations} {count.conversations === 1 ? "conversation" : "conversations"} · {count.messages} {count.messages === 1 ? "message" : "messages"}</p>;
}
export function InstructionsHelp() {
  const [copied, setCopied] = useState(false), [failed, setFailed] = useState(false);
  return <details className="vibe-improve-help"><summary>Where do I find these?</summary><p>Ask the coding tool you used to build your agent:</p><p className="vibe-improve-request">{instructionsHelp}</p>
    <VibeButton variant="quiet" onClick={async () => { try { await navigator.clipboard.writeText(instructionsHelp); setCopied(true); setFailed(false); } catch { setFailed(true); } }}><Copy aria-hidden="true" />{copied ? "Copy request · copied" : "Copy request"}</VibeButton>
    {failed && <p role="status">Select and copy the request above.</p>}
  </details>;
}
