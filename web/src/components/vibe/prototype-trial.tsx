"use client";

import { useRef, type ReactNode } from "react";
import { VibeDataUseNotice } from "./vibe-data-use";
import { friendlyModelName } from "./model-name";
import { ArrowUp } from "lucide-react";
import { dollars, type Model, type Session } from "@/lib/vibe";
import { AgentReply } from "./safe-markdown";
import { TaskOutput } from "./task-output";
import { VibeButton } from "./vibe-button";
import { sendOnEnter } from "./composer-keyboard";
import { useComposerAutosize } from "./use-composer-autosize";

export function PrototypeTrial({ workspace = false, sessionID, error, sendError, notice, materialInput, hasMaterial = false, inline, dock = false, title, version, busy, inferenceBlocked = false, text, onText, onSend, onBack, onNew, thread, onThread, history, messages, example, model }: {
  workspace?: boolean;
  sessionID?: string;
  error?: string;
  sendError?: string;
  notice?: ReactNode;
  materialInput?: ReactNode;
  hasMaterial?: boolean;
  inline: boolean;
  dock?: boolean;
  title?: string;
  version?: number;
  busy: boolean;
  inferenceBlocked?: boolean;
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
  const shownError = error || sendError;
  useComposerAutosize(input, text, true, true, thread);
  return <section className={dock ? "vibe-prototype-trial vibe-trial-dock" : "vibe-prototype-trial"} aria-label="Try your agent">
    {dock && <VibeButton variant="quiet" disabled={busy} onClick={onNew}>New conversation</VibeButton>}
    {dock ? <div className="vibe-trial-recipient"><p>Trying <strong>{title || "your agent"}</strong> · v{version}</p>
      <VibeButton variant="quiet" onClick={onBack}>← Back to Vibe Evals</VibeButton></div> : <div className="vibe-trial-heading"><h3>{inline ? "Try your agent" : "Try a message"}</h3>
      <VibeButton variant="quiet" disabled={busy} onClick={onNew}>New conversation</VibeButton></div>}
    {!dock && <p className="vibe-build-note">{workspace ? "Messages here go to this agent. Change instructions above or discuss changes with Guide." : inline ? "Messages here go to your agent. To change how it works, return to Vibe Evals." : "Talking to the agent from these instructions. Text only; live tools are not connected."}</p>}
    {!workspace && !inline && !dock && <VibeButton variant="quiet" onClick={onBack}>← Back to Vibe Evals</VibeButton>}
    {history.length > 0 && <label className="vibe-trial-history">Conversation
      <select aria-label="Trial conversation" disabled={busy} value={history.some(t => t.id === thread) ? thread : ""} onChange={event => onThread(event.target.value)}>
        <option value="">New conversation</option>{history.map((t, i) => <option key={t.id} value={t.id}>{t.legacy ? "Earlier trial" : "Conversation"} {i + 1}</option>)}
      </select></label>}
    {<div className="vibe-trial-messages" role="log" aria-label={inline ? "Agent conversation" : "Trial conversation"} aria-live="polite">
      {messages.map(m => <div key={m.id} className={`vibe-message ${m.role === "user" ? "vibe-message-user" : ""}`}><p className="vibe-evidence-label">{m.role === "user" ? "You" : "Your agent"}</p>{workspace && m.role === "assistant" ? <TaskOutput text={m.content} sessionID={sessionID} materials={m.materials} /> : <AgentReply>{m.content}</AgentReply>}</div>)}
    </div>}
    {!dock && !messages.length && example && <VibeButton variant="quiet" disabled={busy} onClick={() => onText(example)}>Use a test message</VibeButton>}
    {thread.startsWith("legacy:") ? <p className="vibe-build-note">Start a new conversation to try follow-ups. This earlier trial used one message.</p> : <>
      {materialInput || <VibeDataUseNotice />}
      {shownError && <p id="vibe-trial-error" role="alert">{shownError}</p>}{notice}
      <form className="vibe-composer vibe-composer-compact vibe-chat-composer" onSubmit={event => { event.preventDefault(); if (!busy && !inferenceBlocked && !error && (text.trim() || hasMaterial)) onSend(); }}>
        <label className="sr-only" htmlFor="vibe-trial-message">Message your agent</label>
        <textarea id="vibe-trial-message" aria-describedby={shownError ? "vibe-trial-error" : undefined} ref={input} rows={1} aria-label="Message your agent" placeholder="Give your agent a task…" value={text} onChange={event => onText(event.target.value)} onKeyDown={event => sendOnEnter(event, () => { if (!busy && !inferenceBlocked && !error && (text.trim() || hasMaterial)) onSend(); })} />
        <VibeButton type="submit" variant={workspace && inline ? "secondary" : "primary"} aria-label={dock ? "Try it" : "Send to agent"} disabled={busy || inferenceBlocked || !!error || (!text.trim() && !hasMaterial)}>{dock && <span>Try it</span>}<ArrowUp /></VibeButton>
      </form>
      {!dock && <p className="vibe-build-note">This generates a reply; it does not run or change your tests.</p>}
      {model && <details className="vibe-trial-cost"><summary>{friendlyModelName(model, true)} · {model.input_nano_per_token || model.output_nano_per_token ? "Uses credits · model rates" : "Free model"}</summary>
        <p>Per million tokens: {dollars(model.input_nano_per_token * 1_000_000)} input, {dollars(model.output_nano_per_token * 1_000_000)} output. Cost depends on the conversation length. Sending uses your existing preview allowance.</p>
      </details>}
    </>}
  </section>;
}
