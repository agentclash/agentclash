"use client";

import { useRef, type ReactNode } from "react";
import { ArrowUp } from "lucide-react";
import { dollars, type Model, type Session } from "@/lib/vibe";
import { AgentReply } from "./safe-markdown";
import { VibeButton } from "./vibe-button";
import { sendOnEnter } from "./composer-keyboard";
import { useComposerAutosize } from "./use-composer-autosize";

export function PrototypeTrial({ materialInput, hasMaterial = false, inline, dock = false, executionAvailable = true, title, version, busy, text, onText, onSend, onBack, onNew, thread, onThread, history, messages, example, model }: {
  materialInput?: ReactNode;
  hasMaterial?: boolean;
  inline: boolean;
  dock?: boolean;
  executionAvailable?: boolean;
  title?: string;
  version?: number;
  busy: boolean;
  text: string;
  onText: (text: string) => void;
  onSend: () => void;
  onBack: () => void;
  onNew: () => void;
  thread: string;
  onThread: (id: string) => void;
  history: { id: string; legacy: boolean }[];
  messages: Session["document"]["messages"];
  example?: string;
  model?: Model;
}) {
  const input = useRef<HTMLTextAreaElement>(null);
  useComposerAutosize(input, text, true, true, thread);
  return <section className={dock ? "vibe-prototype-trial vibe-trial-dock" : "vibe-prototype-trial"} aria-label="Try your prototype">
    {dock ? <div className="vibe-trial-recipient"><p>Trying <strong>{title || "your prototype"}</strong> · v{version}</p>
      <div className="flex flex-wrap items-center gap-2"><VibeButton variant="quiet" disabled={busy} onClick={onNew}>New conversation</VibeButton><VibeButton variant="quiet" onClick={onBack}>← Back to Vibe Evals</VibeButton></div></div> : <div className="vibe-trial-heading"><h3>{inline ? "Try your prototype" : "Try a message"}</h3>
      <VibeButton variant="quiet" disabled={busy} onClick={onNew}>New conversation</VibeButton></div>}
    {!dock && <p className="vibe-build-note">{inline ? "Messages here go to your prototype. To change how it works, return to Vibe Evals." : "Talking to the agent from these instructions. Text only; live tools are not connected."}</p>}
    {!inline && !dock && <VibeButton variant="quiet" onClick={onBack}>← Back to Vibe Evals</VibeButton>}
    {history.length > 0 && <label className="vibe-trial-history">Conversation
      <select aria-label="Trial conversation" disabled={busy} value={history.some(t => t.id === thread) ? thread : ""} onChange={event => onThread(event.target.value)}>
        <option value="">New conversation</option>{history.map((t, i) => <option key={t.id} value={t.id}>{t.legacy ? "Earlier trial" : "Conversation"} {i + 1}</option>)}
      </select></label>}
    {!dock && <div className="vibe-trial-messages" role="log" aria-label={inline ? "Prototype conversation" : "Trial conversation"} aria-live="polite">
      {messages.map(m => <div key={m.id} className={`vibe-message ${m.role === "user" ? "vibe-message-user" : ""}`}><p className="vibe-evidence-label">{m.role === "user" ? "You" : "Your prototype"}</p><AgentReply>{m.content}</AgentReply></div>)}
    </div>}
    {!dock && !messages.length && example && <VibeButton variant="quiet" disabled={busy} onClick={() => onText(example)}>Use an example message</VibeButton>}
    {thread.startsWith("legacy:") ? <p className="vibe-build-note">Start a new conversation to try follow-ups. This earlier trial used one message.</p> : <>
      {materialInput}
      <form className="vibe-composer vibe-composer-compact vibe-chat-composer" onSubmit={event => { event.preventDefault(); if (executionAvailable && !busy && (text.trim() || hasMaterial)) onSend(); }}>
        <label className="sr-only" htmlFor="vibe-trial-message">Message your agent</label>
        <textarea id="vibe-trial-message" ref={input} rows={1} aria-label="Message your agent" placeholder="Give your prototype a task…" value={text} onChange={event => onText(event.target.value)} onKeyDown={event => sendOnEnter(event, () => { if (executionAvailable && !busy && (text.trim() || hasMaterial)) onSend(); })} />
        <VibeButton type="submit" variant="primary" aria-label={inline ? "Send to prototype" : "Send to agent"} disabled={!executionAvailable || busy || (!text.trim() && !hasMaterial)}>{dock && <span>Try it</span>}<ArrowUp /></VibeButton>
      </form>
      {!dock && <p className="vibe-build-note">This generates a reply; it does not run or change your tests.</p>}
      {model && <details className="vibe-trial-cost"><summary>{model.name} · {model.input_nano_per_token || model.output_nano_per_token ? "Uses credits · model rates" : "Free model"}</summary>
        <p>Per million tokens: {dollars(model.input_nano_per_token * 1_000_000)} input, {dollars(model.output_nano_per_token * 1_000_000)} output. Cost depends on the conversation length. Sending uses your existing preview allowance.</p>
      </details>}
    </>}
  </section>;
}
