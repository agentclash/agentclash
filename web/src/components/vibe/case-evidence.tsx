"use client";

import { useEffect, useId, useRef, useState } from "react";
import { Check, ChevronRight, Copy, HelpCircle, X } from "lucide-react";
import { caseInput, type CaseResult, type EvidenceMessage } from "@/lib/vibe";
import { AgentReply, SafeMarkdown } from "./safe-markdown";
import { VibeButton } from "./vibe-button";
import {
  evidenceRole,
  expectedBehavior,
  findingCheck,
  fixPrompt,
  type EvidenceCheck,
  type EvidenceOrigin,
} from "./result-prompts";

export function CaseEvidence({
  summary,
  load,
  evidenceVersion,
  onImprove,
  busy,
  onDispute,
  defaultOpen = false,
  focused = false,
  allowFixPrompt = false,
  onFixCopied,
  origin,
  concise = false,
}: {
  summary: CaseResult;
  load: (key: string) => Promise<CaseResult>;
  evidenceVersion?: string;
  onImprove?: (result: CaseResult) => void;
  busy?: boolean;
  onDispute?: (rule: string, result: CaseResult) => void;
  defaultOpen?: boolean;
  focused?: boolean;
  allowFixPrompt?: boolean;
  onFixCopied?: () => void;
  origin?: EvidenceOrigin;
  concise?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const [refresh, setRefresh] = useState(0);
  const [settled, setSettled] = useState<{
    metadata: string;
    refresh: number;
    result?: CaseResult;
    error?: string;
  } | null>(null);
  // Bodies are redacted from snapshots. A persisted event can recover output
  // without changing verdicts, so include its version in the evidence cache key.
  // Identical SSE snapshots still reuse the cache; closed rows never fetch.
  const metadata = JSON.stringify([evidenceVersion, summary]);
  const loader = useRef(load);
  useEffect(() => {
    loader.current = load;
  }, [load]);
  useEffect(() => {
    if (!open) return;
    let current = true;
    void loader
      .current(summary.case_key)
      .then((result) => {
        if (
          result.case_key !== summary.case_key ||
          result.version !== summary.version ||
          !Array.isArray(result.checks) ||
          typeof result.output !== "string"
        )
          throw new Error(
            "Saved evidence could not be read for this result. Try loading it again.",
          );
        if (current) setSettled({ metadata, refresh, result });
      })
      .catch((e: Error) => {
        if (current) setSettled({ metadata, refresh, error: e.message });
      });
    return () => {
      current = false;
    };
  }, [open, metadata, summary.case_key, summary.version, refresh]);
  const current =
    settled?.metadata === metadata && settled.refresh === refresh
      ? settled
      : null;
  const evidence = current?.result;
  const error = current?.error;
  const loading = open && !current;
  const finding = evidence ? findingCheck(evidence) : undefined;
  const prompt =
    evidence && allowFixPrompt ? fixPrompt(evidence, origin) : null;
  const explanation =
    finding?.error?.message ||
    finding?.evidence ||
    "A detailed explanation was not recorded for this check.";
  const excerpt = concise && explanation.length > 240;
  const boundary = explanation.lastIndexOf(" ", 240);
  const visibleExplanation = excerpt
    ? explanation.slice(0, boundary > 180 ? boundary : 240) + "…"
    : explanation;

  return (
    <details
      className="group vibe-result-row"
      open={open}
      onToggle={(e) => setOpen(e.currentTarget.open)}
    >
      <summary className="flex cursor-pointer list-none items-center gap-3 text-sm">
        {summary.verdict === "PASS" ? (
          <Check size={16} aria-hidden="true" />
        ) : summary.verdict === "FAIL" ? (
          <X size={16} className="text-builder-warn" aria-hidden="true" />
        ) : (
          <HelpCircle
            size={16}
            className="text-builder-fg-muted"
            aria-hidden="true"
          />
        )}
        <span className="min-w-0 flex-1">
          {summary.title ||
            (concise && evidence?.input != null
              ? caseInput(evidence.input)
              : summary.case_key
                  .replace(/^case-/, concise ? "Test " : "Situation ")
                  .replaceAll("-", " "))}
        </span>
        <span className="text-xs text-builder-fg-muted">
          {summary.verdict === "PASS"
            ? "Passed"
            : summary.verdict === "FAIL"
              ? "Issue found"
              : summary.error?.code === "not_evaluated"
                ? "Not run"
                : "Could not determine"}
        </span>
        <ChevronRight
          size={16}
          className="vibe-chevron vibe-muted"
          aria-hidden="true"
        />
      </summary>
      <div className="space-y-4 border-t border-[var(--vibe-border)] p-5">
        {loading && (
          <p role="status" className="text-xs vibe-muted">
            Loading saved evidence…
          </p>
        )}
        {error && (
          <p role="alert" className="text-sm text-builder-warn">
            {error}
          </p>
        )}
        {evidence && (
          <>
            {evidence.error && (
              <p className="text-sm text-builder-warn">
                {evidence.error.message}
              </p>
            )}
            {finding && (
              <div>
                <p className="mb-1 text-xs vibe-muted">
                  {finding.verdict === "FAIL"
                    ? "What needs attention"
                    : finding.verdict === "PASS"
                      ? "Why this passed"
                      : "What is still unknown"}
                  {excerpt && " · excerpt"}
                </p>
                <SafeMarkdown>{visibleExplanation}</SafeMarkdown>
              </div>
            )}
            <PrimaryReply
              evidence={evidence}
              finding={finding}
              origin={origin}
            />
            {prompt && (
              <FixPromptCopy
                prompt={prompt}
                busy={busy}
                primary={focused}
                onCopied={onFixCopied}
              />
            )}
            <EvidenceDetails
              evidence={evidence}
              finding={finding}
              origin={origin}
              onDispute={onDispute}
              onImprove={onImprove}
              busy={busy}
            />
          </>
        )}
        {(error || summary.verdict === "UNKNOWN") && (
          <VibeButton
            variant="quiet"
            disabled={loading}
            onClick={() => setRefresh((value) => value + 1)}
          >
            Reload saved evidence
          </VibeButton>
        )}
      </div>
    </details>
  );
}

function PrimaryReply({
  evidence,
  finding,
  origin,
}: {
  evidence: CaseResult;
  finding?: EvidenceCheck;
  origin?: EvidenceOrigin;
}) {
  const cited = new Set(finding?.message_ids || []);
  const replies =
    evidence.messages?.filter((message) => message.role === "assistant") || [];
  const message =
    replies.findLast((reply) => cited.has(reply.id)) || replies.at(-1);
  const text = message?.content || evidence.output;
  if (!text) return null;
  // This is a verbatim excerpt for scanning. The saved reply remains complete in
  // See evidence and in the copied fix prompt; no evaluated material is changed.
  const excerpt = text.length > 280;
  const boundary = text.lastIndexOf(" ", 280);
  const quote = excerpt ? text.slice(0, boundary > 200 ? boundary : 280) : text;
  return (
    <blockquote className="border-l-2 border-[var(--vibe-border)] pl-4 text-sm">
      <p className="mb-1 text-xs vibe-muted">
        {origin === "prompt"
          ? "Generated reply"
          : origin === "provided_conversations"
            ? "App reply"
            : "Recorded reply"}
        {excerpt ? " · excerpt" : ""}
      </p>
      <p className="vibe-transcript">
        {quote}
        {excerpt && "…"}
      </p>
    </blockquote>
  );
}

function EvidenceDetails({
  evidence,
  finding,
  origin,
  onDispute,
  onImprove,
  busy,
}: {
  evidence: CaseResult;
  finding?: EvidenceCheck;
  origin?: EvidenceOrigin;
  onDispute?: (rule: string, result: CaseResult) => void;
  onImprove?: (result: CaseResult) => void;
  busy?: boolean;
}) {
  const id = useId();
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="space-y-3">
      <div className="-ml-3 flex flex-wrap items-center gap-1">
        <VibeButton
          variant="quiet"
          aria-expanded={expanded}
          aria-controls={id}
          onClick={() => setExpanded((value) => !value)}
        >
          {expanded ? "Hide evidence" : "See evidence"}
          <ChevronRight
            aria-hidden="true"
            className={expanded ? "rotate-90" : ""}
          />
        </VibeButton>
        {onDispute && finding && (
          <VibeButton
            variant="quiet"
            disabled={busy}
            onClick={() =>
              onDispute(expectedBehavior(evidence, finding), evidence)
            }
          >
            That’s not what I meant
          </VibeButton>
        )}
      </div>
      <div
        id={id}
        hidden={!expanded}
        className="space-y-6 border-t border-[var(--vibe-border)] pt-5"
      >
        {(evidence.expected || evidence.expectations?.length) && (
          <div className="text-sm">
            <p className="mb-2 text-xs vibe-muted">Expected behavior</p>
            <SafeMarkdown>{expectedBehavior(evidence, finding)}</SafeMarkdown>
          </div>
        )}
        <div aria-label="Supporting messages" className="space-y-3">
          <p className="text-xs vibe-muted">Supporting messages</p>
          <SupportingEvidence
            evidence={evidence}
            finding={finding}
            origin={origin}
          />
        </div>
        {evidence.messages?.length ? (
          <div>
            <p className="mb-3 text-xs vibe-muted">
              Full conversation · {evidence.messages.length} messages
            </p>
            <ol className="space-y-4">
              {evidence.messages.map((message, index) => (
                <li key={message.id}>
                  <p className="mb-1 text-xs vibe-muted">
                    {evidenceRole(message.role)} · message {index + 1}
                  </p>
                  <p className="vibe-transcript text-sm">{message.content}</p>
                </li>
              ))}
            </ol>
          </div>
        ) : (
          <div className="space-y-4">
            <div>
              <p className="mb-1 text-xs vibe-muted">
                Original message or task
              </p>
              <pre className="max-h-64 overflow-auto whitespace-pre-wrap break-words text-sm">
                {caseInput(evidence.input)}
              </pre>
            </div>
            {evidence.output && (
              <div>
                <p className="mb-1 text-xs vibe-muted">
                  {origin === "prompt"
                    ? "Complete generated reply"
                    : "Complete recorded reply"}
                </p>
                <AgentReply>{evidence.output}</AgentReply>
              </div>
            )}
          </div>
        )}
        <div className="space-y-5">
          <p className="text-xs vibe-muted">
            All checks · {evidence.checks.length}
          </p>
          {evidence.checks.map((check) => (
            <div key={check.key}>
              <p className="text-xs font-medium">
                {check.key === "behavior"
                  ? "Expected behavior"
                  : check.key === "has_answer"
                    ? "Reply received"
                    : evidence.expectations?.find(
                        (rule) => rule.id === check.key,
                      )?.statement || check.key}
                {" · "}
                {check.verdict === "PASS"
                  ? "Passed"
                  : check.verdict === "FAIL"
                    ? "Failed"
                    : "Could not determine"}
              </p>
              <SafeMarkdown>
                {check.error?.message || check.evidence}
              </SafeMarkdown>
              <CitedMessages evidence={evidence} check={check} />
              {onDispute && (
                <VibeButton
                  variant="quiet"
                  className="mt-2"
                  disabled={busy}
                  onClick={() =>
                    onDispute(expectedBehavior(evidence, check), evidence)
                  }
                >
                  That’s not our rule
                </VibeButton>
              )}
            </div>
          ))}
          {onImprove && evidence.verdict === "FAIL" && (
            <VibeButton disabled={busy} onClick={() => onImprove(evidence)}>
              Improve this behavior
            </VibeButton>
          )}
        </div>
      </div>
    </div>
  );
}

function SupportingEvidence({
  evidence,
  finding,
  origin,
}: {
  evidence: CaseResult;
  finding?: EvidenceCheck;
  origin?: EvidenceOrigin;
}) {
  if (evidence.messages?.length) {
    const ids = new Set(finding?.message_ids || []);
    const cited = evidence.messages.filter((message) => ids.has(message.id));
    return (
      <>
        {cited.map((message) => (
          <MessageQuote
            key={message.id}
            message={message}
            index={evidence.messages!.findIndex((m) => m.id === message.id)}
          />
        ))}
      </>
    );
  }
  if (!evidence.output) return null;
  return (
    <div>
      <p className="mb-2 text-xs vibe-muted">
        {origin === "prompt"
          ? "Reply generated for this test"
          : "Recorded reply"}
      </p>
      <div className="max-h-64 overflow-auto border-l-2 border-[var(--vibe-border)] pl-4">
        <AgentReply>{evidence.output}</AgentReply>
      </div>
    </div>
  );
}

function MessageQuote({
  message,
  index,
}: {
  message: EvidenceMessage;
  index: number;
}) {
  return (
    <blockquote className="border-l-2 border-[var(--vibe-border)] pl-4 text-sm">
      <p className="mb-1 text-xs vibe-muted">
        {evidenceRole(message.role)} · message {index + 1}
      </p>
      <p className="vibe-transcript max-h-48 overflow-auto">
        {message.content}
      </p>
    </blockquote>
  );
}

function CitedMessages({
  evidence,
  check,
}: {
  evidence: CaseResult;
  check: EvidenceCheck;
}) {
  const ids = new Set(check.message_ids || []);
  return (
    <>
      {evidence.messages
        ?.filter((message) => ids.has(message.id))
        .map((message) => (
          <div className="mt-3" key={message.id}>
            <MessageQuote
              message={message}
              index={evidence.messages!.findIndex((m) => m.id === message.id)}
            />
          </div>
        ))}
    </>
  );
}

function FixPromptCopy({
  prompt,
  busy,
  primary,
  onCopied,
}: {
  prompt: string;
  busy?: boolean;
  primary: boolean;
  onCopied?: () => void;
}) {
  const [copying, setCopying] = useState(false);
  const [receipt, setReceipt] = useState<{ prompt: string; copied: boolean }>();
  const copied = receipt?.prompt === prompt && receipt.copied;
  const failed = receipt?.prompt === prompt && !receipt.copied;
  const copy = async () => {
    setCopying(true);
    try {
      await navigator.clipboard.writeText(prompt);
      setReceipt({ prompt, copied: true });
      onCopied?.();
    } catch {
      setReceipt({ prompt, copied: false });
    } finally {
      setCopying(false);
    }
  };
  return (
    <div className="space-y-2">
      <VibeButton
        variant={primary && !copied ? "primary" : "secondary"}
        disabled={busy || copying}
        loading={copying}
        onClick={() => void copy()}
      >
        <Copy aria-hidden="true" />
        {copied ? "Copy fix prompt again" : "Copy fix prompt"}
      </VibeButton>
      <p className="text-xs vibe-muted" role={copied ? "status" : undefined}>
        {copied
          ? "Copied. Paste it into your coding tool, then bring back the new answer."
          : "Paste this into your coding tool."}
      </p>
      {failed && (
        <div className="space-y-2">
          <p role="status" className="text-sm vibe-muted">
            Copy is unavailable in this browser. Select the prompt below to copy
            it.
          </p>
          <textarea
            aria-label="Fix prompt to copy"
            className="vibe-textarea min-h-48"
            readOnly
            value={prompt}
            onFocus={(event) => event.currentTarget.select()}
            onCopy={onCopied}
          />
        </div>
      )}
    </div>
  );
}
