"use client";

import { useRef, useState } from "react";
import type { Session } from "@/lib/vibe";
import type { ConversationAction } from "@/lib/vibe-conversation";

type Choice = Pick<ConversationAction, "kind" | "target_id" | "target_revision"> &
  Partial<Pick<ConversationAction, "option_ids" | "text">>;

export function ConversationActions({ session, busy, onAction, onReload }: {
  session: Session;
  busy: boolean;
  onAction: (action: ConversationAction) => Promise<void>;
  onReload: () => Promise<void>;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [selection, setSelection] = useState<{ question: string; ids: string[] }>({ question: "", ids: [] });
  const inFlight = useRef(false);
  const request = useRef<ConversationAction | null>(null);
  const state = session.document.conversation_state;
  if (state?.actions_version !== 1) return null;
  const proposal = state.proposal?.status === "proposed" && state.proposal.scope_id === state.brief.scope_id ? state.proposal : undefined;
  const question = state.pending_question?.status === "active" && state.pending_question.scope_id === state.brief.scope_id ? state.pending_question : undefined;
  const change = session.document.last_change;
  const canUndo = change?.scope_id === state.brief.scope_id && change.message_id === state.through_message_id && change.after_artifact_id === session.document.artifacts.at(-1)?.id;
  const disabled = busy || pending;
  const button = "inline-flex min-h-11 max-w-full items-center justify-center rounded-lg border border-builder-border [overflow-wrap:anywhere] px-3 py-2 text-left text-sm leading-5 transition-colors hover:bg-builder-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-builder-border-strong disabled:cursor-not-allowed disabled:opacity-50 motion-reduce:transition-none";

  async function send(choice?: Choice) {
    if (inFlight.current || busy || !state) return;
    if (choice) {
      request.current = {
        idempotency_key: crypto.randomUUID(), scope_id: state.brief.scope_id,
        session_revision: session.revision, option_ids: [], text: null, ...choice,
      };
    }
    if (!request.current) return;
    inFlight.current = true;
    setPending(true);
    setError("");
    try {
      await onAction(request.current);
      request.current = null;
    } catch (e) {
      setError(e instanceof Error ? e.message : "That choice could not be saved. Try again.");
    } finally {
      inFlight.current = false;
      setPending(false);
    }
  }

  return (
    <section aria-label="Conversation choices" aria-busy={pending} className="min-w-0 space-y-3 text-builder-fg">
      {proposal ? (
        <div className="space-y-3 rounded-xl border border-builder-border p-4">
          <p className="text-xs text-builder-fg-muted">Suggested checks</p>
          <ul className="list-disc space-y-1 break-words pl-5 text-sm leading-6">
            {proposal.facts.map(fact => <li key={fact.id}>{fact.text}</li>)}
          </ul>
          <div className="flex flex-wrap gap-2">
            <button type="button" className={button} disabled={disabled} onClick={() => void send({ kind: "adopt_proposal", target_id: proposal.id, target_revision: proposal.revision })}>Use these checks</button>
            <button type="button" className={button} disabled={disabled} onClick={() => void send({ kind: "reject_proposal", target_id: proposal.id, target_revision: proposal.revision })}>Leave these out</button>
          </div>
        </div>
      ) : question && (question.options.length > 0 || question.purpose === "offer_help") ? (
        <fieldset className="min-w-0 space-y-3">
          <legend className={session.document.messages.at(-1)?.id === question.origin_message_id ? "sr-only" : "mb-2 break-words text-sm leading-6"}>{question.text}</legend>
          <div className="flex flex-wrap gap-2">
            {(question.purpose === "offer_help" ? [] : question.options).map(option => question.max_selections === 1 ? (
              <button type="button" className={button} key={option.id} disabled={disabled} onClick={() => void send({ kind: "answer_question", target_id: question.id, target_revision: question.revision, option_ids: [option.id] })}>{option.label}</button>
            ) : (
              <label className={button} key={option.id}>
                <input type="checkbox" className="mr-2" disabled={disabled} checked={selection.question === question.id && selection.ids.includes(option.id)} onChange={e => {
                  const ids = selection.question === question.id ? selection.ids : [];
                  setSelection({ question: question.id, ids: e.target.checked ? [...ids, option.id] : ids.filter(id => id !== option.id) });
                }} />{option.label}
              </label>
            ))}
            {question.max_selections > 1 && <button type="button" className={button} disabled={disabled || selection.question !== question.id || selection.ids.length === 0 || selection.ids.length > question.max_selections} onClick={() => void send({ kind: "answer_question", target_id: question.id, target_revision: question.revision, option_ids: selection.ids })}>Use these answers</button>}
            {question.purpose === "offer_help" && <button type="button" className={button} disabled={disabled} onClick={() => void send({ kind: "dismiss_help", target_id: question.id, target_revision: question.revision })}>Skip this help</button>}
          </div>
        </fieldset>
      ) : null}
      {canUndo && change && <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm" role="status">
        <span className="min-w-0 break-words text-builder-fg-muted">{change.rule_ids?.length ? `${change.rule_ids.length} rule${change.rule_ids.length === 1 ? "" : "s"} changed.` : "Change saved."}</span>
        <button type="button" className={button} disabled={disabled} onClick={() => void send({ kind: "undo", target_id: change.id, target_revision: change.revision })}>Undo</button>
      </div>}
      {pending && <p role="status" className="text-sm text-builder-fg-muted">Saving your choice…</p>}
      {error && <div role="alert" className="space-y-2 text-sm">
        <p className="break-words">{error}</p>
        <div className="flex flex-wrap gap-2">
          <button type="button" className={button} disabled={disabled} onClick={() => void send()}>Try again</button>
          <button type="button" className={button} disabled={disabled} onClick={async () => { try { await onReload(); request.current = null; setError(""); } catch { setError("Could not refresh. Try again."); } }}>Refresh choices</button>
        </div>
      </div>}
    </section>
  );
}
