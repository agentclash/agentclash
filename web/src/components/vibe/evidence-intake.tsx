"use client";

import { useEffect, useRef, useState } from "react";
import { ArrowRight, FileText, Paperclip } from "lucide-react";
import type { EvidenceSet } from "@/lib/vibe";
import { VibeButton } from "./vibe-button";

export const exampleBrief =
  "Our support agent explains returns. Unopened items can be returned within 30 days. Opened items and older purchases are ineligible. It should ask only for missing purchase age or item condition, remember details across follow-ups, and never claim to process refunds.";
export const exampleChats = `Customer: I bought it 10 days ago. Can I return it?
Agent: Is the item opened or unopened?
Customer: Unopened.
Agent: It is eligible for return. What is your order number and why are you returning it?
---
Customer: I bought it 45 days ago and it is unopened. Can I return it?
Agent: It is outside the 30-day return window, so it is not eligible.
---
Customer: I want to return an item.
Agent: How many days ago did you buy it, and is it opened or unopened?`;

export function EvidenceIntake({
  evidence,
  busy,
  comparing,
  autoPrepare = false,
  onAttach,
  onPrepare,
  onCancel,
}: {
  evidence?: EvidenceSet;
  busy: boolean;
  comparing?: boolean;
  autoPrepare?: boolean;
  onAttach: (input: {
    content?: string;
    label?: string;
    parent_id?: string;
    roles?: Record<string, string>;
  }) => Promise<boolean>;
  onPrepare: (evidenceID?: string) => void;
  onCancel: () => void;
}) {
  const [text, setText] = useState("");
  const [label, setLabel] = useState("Pasted conversations");
  const [roles, setRoles] = useState<Record<string, string>>({});
  const [editing, setEditing] = useState(!evidence);
  const [issue, setIssue] = useState("");
  const [attaching, setAttaching] = useState(false);
  const [preparation, setPreparation] = useState<{
    previousID?: string;
    ready: boolean;
  } | null>(null);
  const file = useRef<HTMLInputElement>(null);
  const attachmentInFlight = useRef(false);
  const prepared = useRef<string | undefined>(undefined);
  const missing = evidence?.conversations.some((c) =>
    c.messages.some((m) => m.role === "unknown" && !roles[m.id]),
  );
  const corrections = Object.keys(roles).length > 0;
  const persistedSpeakersKnown = evidence?.conversations.every((conversation) =>
    conversation.messages.every((message) => message.role !== "unknown"),
  );
  useEffect(() => {
    if (
      !autoPrepare ||
      !preparation?.ready ||
      busy ||
      attaching ||
      corrections ||
      !evidence ||
      !persistedSpeakersKnown ||
      evidence.id === preparation.previousID ||
      prepared.current !== undefined
    )
      return;
    // Only an explicit successful submission can arm this handoff. Persisted
    // evidence and speaker corrections must arrive before starting the check.
    prepared.current = evidence.id;
    onPrepare(evidence.id);
  }, [
    autoPrepare,
    preparation,
    busy,
    attaching,
    corrections,
    evidence,
    persistedSpeakersKnown,
    onPrepare,
  ]);
  const attach = async (input: Parameters<typeof onAttach>[0]) => {
    if (busy || attachmentInFlight.current) return;
    attachmentInFlight.current = true;
    setAttaching(true);
    setIssue("");
    prepared.current = undefined;
    if (autoPrepare) setPreparation({ previousID: evidence?.id, ready: false });
    try {
      if (await onAttach(input)) {
        setEditing(false);
        setRoles({});
        if (autoPrepare)
          setPreparation({ previousID: evidence?.id, ready: true });
      } else {
        setPreparation(null);
      }
    } catch (error) {
      setPreparation(null);
      setIssue(
        error instanceof Error
          ? error.message
          : "Couldn’t add this answer. Please try again.",
      );
    } finally {
      attachmentInFlight.current = false;
      setAttaching(false);
    }
  };
  const add = () => {
    if (text.trim()) void attach({ content: text, label });
  };
  const working = busy || attaching;
  return (
    <section
      aria-label={comparing ? "Check the new answer" : "Add conversations"}
      className="space-y-5"
    >
      <div>
        <h2 className="text-xl font-semibold tracking-tight">
          {comparing ? "Check the new answer" : "Add an answer from your app"}
        </h2>
        <p className="vibe-muted mt-2 text-sm">
          {comparing
            ? "Paste the same question and your app’s new answer. Keep any follow-up questions in the same order so the comparison stays fair."
            : "Include what you asked and what it said, including any follow-ups."}
        </p>
      </div>
      {editing ? (
        <>
          <div className="vibe-composer">
            <label className="sr-only" htmlFor="vibe-evidence">
              Conversations to check
            </label>
            <textarea
              id="vibe-evidence"
              value={text}
              readOnly={working}
              onChange={(e) => setText(e.target.value)}
              placeholder={
                "User: The question you asked…\nAssistant: Your app’s answer…"
              }
              style={{ minHeight: 180 }}
            />
            <div className="vibe-composer-actions">
              <VibeButton
                variant="quiet"
                disabled={working}
                onClick={() => file.current?.click()}
              >
                <Paperclip />
                Attach a file
              </VibeButton>
              <VibeButton
                variant="primary"
                disabled={working || !text.trim()}
                onClick={add}
              >
                {autoPrepare ? "Check new answer" : "Review messages"}
                <ArrowRight />
              </VibeButton>
            </div>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-2 text-xs vibe-muted">
            <span>Include both sides of the conversation.</span>
            {!comparing && (
              <VibeButton
                variant="quiet"
                disabled={working}
                onClick={() => {
                  setText(exampleChats);
                  setLabel("Sample conversations");
                }}
              >
                Use sample chats
              </VibeButton>
            )}
          </div>
          <details className="text-sm vibe-muted">
            <summary className="cursor-pointer">Supported formats</summary>
            <p className="mt-2">
              Paste User: / Assistant: labels, or attach .txt, .md or JSON. JSON
              accepts a messages array with role and content, or a conversations
              array containing messages.
            </p>
          </details>
          <input
            ref={file}
            type="file"
            accept=".txt,.md,.json"
            hidden
            aria-label="Chat file"
            onChange={async (e) => {
              const f = e.target.files?.[0];
              if (!f) return;
              setIssue("");
              try {
                if (f.size > 1024 * 1024)
                  throw new Error("Choose a file under 1 MB.");
                setText(await f.text());
                setLabel(f.name);
              } catch (e) {
                setIssue((e as Error).message);
              } finally {
                if (file.current) file.current.value = "";
              }
            }}
          />
        </>
      ) : (
        evidence && (
          <>
            <p className="flex items-center gap-2 text-sm vibe-muted">
              <FileText size={16} />
              {evidence.label} · {evidence.conversations.length}{" "}
              {evidence.conversations.length === 1
                ? "conversation"
                : "conversations"}
            </p>
            <div className="vibe-panel">
              {evidence.conversations.map((c) => (
                <details
                  key={c.key}
                  open={
                    c.messages.some((m) => m.role === "unknown") || undefined
                  }
                >
                  <summary>
                    {c.title}
                    <span className="ml-2 text-xs vibe-muted">
                      {c.messages.length} messages
                    </span>
                  </summary>
                  <ol className="space-y-4 border-t border-[var(--vibe-border)] p-5">
                    {c.messages.map((m, index) => (
                      <li key={m.id}>
                        <label className="flex items-center gap-3 text-xs vibe-muted">
                          Message {index + 1}
                          <select
                            aria-label={`Speaker for ${m.id}`}
                            value={roles[m.id] || m.role}
                            disabled={working}
                            onChange={(e) =>
                              setRoles((old) => ({
                                ...old,
                                [m.id]: e.target.value,
                              }))
                            }
                            className="rounded-md border border-[var(--vibe-border)] bg-transparent p-2 text-sm"
                          >
                            <option value="unknown" disabled>
                              Identify speaker
                            </option>
                            <option value="user">User</option>
                            <option value="assistant">AI</option>
                            <option value="system">System context</option>
                            <option value="tool">Tool output</option>
                          </select>
                        </label>
                        <p className="vibe-transcript mt-2 text-sm">
                          {m.content}
                        </p>
                      </li>
                    ))}
                  </ol>
                </details>
              ))}
            </div>
            {missing && (
              <p className="text-sm text-builder-warn">
                Identify the highlighted speakers before continuing. To split a
                turn, add speaker labels to the original text.
              </p>
            )}
            <div className="flex flex-wrap justify-between gap-3">
              <VibeButton
                variant="quiet"
                disabled={working}
                onClick={() => {
                  setText(evidence.raw);
                  setLabel(evidence.label);
                  setEditing(true);
                  setPreparation(null);
                }}
              >
                Edit source
              </VibeButton>
              <VibeButton
                variant="primary"
                disabled={working || !!missing}
                onClick={async () => {
                  if (corrections) {
                    await attach({ parent_id: evidence.id, roles });
                    return;
                  }
                  onPrepare(evidence.id);
                }}
              >
                {corrections
                  ? autoPrepare
                    ? "Save speakers and check"
                    : "Save speaker corrections"
                  : comparing
                    ? "Check new answer"
                    : "Prepare the check"}
                <ArrowRight />
              </VibeButton>
            </div>
          </>
        )
      )}
      {issue && (
        <p role="alert" className="text-sm text-builder-warn">
          {issue}
        </p>
      )}
      <VibeButton
        variant="quiet"
        disabled={working}
        onClick={() => {
          setPreparation(null);
          onCancel();
        }}
      >
        {comparing ? "Back to the finding" : "Back to conversation"}
      </VibeButton>
    </section>
  );
}
