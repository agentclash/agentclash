"use client";
import { createContext, useContext, useState, type ReactNode } from "react";
import { dollars, type Artifact, type Models, type Operation, type RunQuote, type Session } from "@/lib/vibe";
import { VibeButton } from "./vibe-button";

type Price = { quote?: RunQuote; error?: string; credits?: boolean; refresh(): void; onCredits(): void };
type AdmittedQuote = { quote: RunQuote; sessionID: string; revision: number; selectionArtifact?: string; kind: string; requestModels: Models; baseline?: { id: string }; extra: { artifact_id?: string; evidence_set_id?: string; case_keys?: string[]; purpose?: string } };
const ImprovePrice = createContext<Price | undefined>(undefined);
export const useImprovePrice = () => useContext(ImprovePrice);
const scope = (session: string | undefined, revision: number | undefined, artifact: string | undefined, evidence: string | undefined, baseline: string | undefined, kind: string, models: Models) => JSON.stringify([session, revision, artifact, evidence, baseline, kind, models.assistant, models.target, models.evaluator]);

// Only the Run click requests a quote. This display may reuse that quote after consent closes.
export function ImproveRunProvider(p: { session?: Session | null; artifact?: Artifact; baseline?: Operation; models: Models; now: number; admitted?: AdmittedQuote; onCredits(): void; creditError?: string; onClearError?(): void; children: ReactNode }) {
  const [cached, setCached] = useState<AdmittedQuote>();
  if (p.admitted && p.admitted !== cached) setCached(p.admitted);
  const ready = p.session?.document.evaluation?.door === "test" && p.artifact && (p.artifact.kind === "test_suite" && p.artifact.agent_prompt.trim() || p.artifact.conversation_evaluation);
  const models = p.baseline ? { ...p.models, evaluator: p.baseline.models.evaluator } : p.models;
  const expected = scope(p.session?.id, p.session?.revision, p.artifact?.id, p.artifact?.conversation_evaluation?.evidence_set_id, p.baseline?.id, p.baseline ? "retest" : "check", models);
  const source = cached;
  const actual = source && scope(source.sessionID, source.revision, source.extra.artifact_id, source.extra.evidence_set_id, source.baseline?.id, source.kind, source.requestModels);
  const quote = source && actual === expected && source.selectionArtifact === p.artifact?.id && !source.extra.case_keys && !source.extra.purpose && Date.parse(source.quote.expires_at) > p.now && Number.isFinite(source.quote.max_cost_nano_usd) && source.quote.max_cost_nano_usd >= 0 ? source.quote : undefined;
  return <ImprovePrice.Provider value={ready ? { quote: p.creditError ? undefined : quote, error: p.creditError, credits: !!p.creditError, refresh: () => p.onClearError?.(), onCredits: p.onCredits } : undefined}>{p.children}</ImprovePrice.Provider>;
}
export function priceLabel(label: string, price?: Price) { return price?.quote ? `${label} · up to ${dollars(price.quote.max_cost_nano_usd)}` : label; }
export function ImproveRunNotice({ reason }: { reason?: string }) {
  const price = useImprovePrice();
  return <>{reason && <p role="status" className="text-sm vibe-muted">{reason}</p>}{price?.error && <p role="alert" className="text-sm">{price.error} Add credits to run these tests. <VibeButton variant="quiet" onClick={price.onCredits}>Credits</VibeButton><VibeButton variant="quiet" onClick={price.refresh}>Try again</VibeButton></p>}</>;
}
export function ImproveRunButton({ count, comparison, busy, blockedReason, onRun, primary = true }: { count: number; comparison?: boolean; busy?: boolean; blockedReason?: string; onRun(): void; primary?: boolean }) {
  const price = useImprovePrice();
  return <div className="space-y-2"><VibeButton variant={primary ? "primary" : "secondary"} disabled={busy || !!blockedReason || price?.credits} onClick={onRun}>{priceLabel(comparison ? "Test this change" : `Run ${count} ${count === 1 ? "test" : "tests"}`, price)}</VibeButton><ImproveRunNotice reason={blockedReason} /></div>;
}
