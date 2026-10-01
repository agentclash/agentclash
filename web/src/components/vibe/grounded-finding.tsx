import type { CaseResult } from "@/lib/vibe";
import { expectedBehavior, type EvidenceCheck } from "./result-prompts";

export function verifiedFinding(result: CaseResult, check?: EvidenceCheck) {
  const finding = check?.finding;
  if (
    check?.evidence_version !== 1 || check.error || !finding ||
    typeof finding.missing !== "string"
  ) return null;
  const replies = result.messages?.length
    ? new Map(result.messages.filter(message => message.role === "assistant")
      .map(message => [message.id, message.content]))
    : new Map([["output", result.output]]);
  if (
    !Array.isArray(finding.quotes) || finding.quotes.length > 4 ||
    !Array.isArray(finding.covered_message_ids)
  ) return null;
  if (finding.quotes.some(quote =>
    !quote || typeof quote.text !== "string" || !quote.text.trim() ||
    [...quote.text].length > 800 || !replies.get(quote.message_id)?.includes(quote.text)
  )) return null;
  if (
    finding.kind === "observed" && check.verdict !== "UNKNOWN" &&
    finding.quotes.length && !finding.missing && !finding.covered_message_ids.length
  ) return finding;
  if (
    finding.kind === "missing_behavior" && check.verdict === "FAIL" &&
    finding.missing.trim() && [...finding.missing].length <= 500 && replies.size > 0 &&
    finding.covered_message_ids.length === replies.size &&
    new Set(finding.covered_message_ids).size === replies.size &&
    finding.covered_message_ids.every(id => replies.get(id)?.trim())
  ) return finding;
  return null;
}

export function GroundedFinding({ result, check }: {
  result: CaseResult;
  check?: EvidenceCheck;
}) {
  const finding = verifiedFinding(result, check);
  if (!finding) return null;
  return (
    <section aria-label="Evidence for this grade" className="space-y-3 text-sm">
      <div>
        <p className="mb-1 text-xs vibe-muted">What should happen</p>
        <p className="whitespace-pre-wrap break-words">{expectedBehavior(result, check)}</p>
      </div>
      {finding.kind === "missing_behavior" && (
        <div>
          <p className="mb-1 text-xs vibe-muted">Missing from the replies</p>
          <p className="whitespace-pre-wrap break-words">{finding.missing}</p>
          <p className="mt-1 text-xs vibe-muted">
            The judge checked the complete replies for this behavior. Open the full evidence to review its conclusion.
          </p>
        </div>
      )}
      {finding.quotes.map((quote, index) => (
        <blockquote key={`${quote.message_id}:${index}`} className="border-l-2 border-[var(--vibe-border)] pl-4">
          <p className="mb-1 text-xs vibe-muted">Quoted from the reply</p>
          <p className="vibe-transcript">{quote.text}</p>
        </blockquote>
      ))}
    </section>
  );
}
