import type { Artifact, Operation, Session } from "./vibe";

export type EnquiryReceipt = { id: string; status: "received" | "sending" | "provider_accepted" | "needs_review" | "cancelled"; created_at: string };
export function projectSummary(session: Session, artifact: Artifact, operation?: Operation) {
  const rules = artifact.sample_basis?.rules || session.document.policies?.find(p => p.id === artifact.policy_id)?.rules || [];
  const score = operation?.scorecard;
  return [
    `Project: ${artifact.title}`,
    ...(artifact.kind === "task_brief" ? [artifact.summary || "", "Unexecuted brief. No prototype or checks have run."] : []),
    artifact.sample ? "Sample demonstration. These assumptions are not verified business policy." : "Based on the supplied requirements.",
    rules.length ? `Rules used:\n${rules.map(r => `• ${r.statement}`).join("\n")}` : "Business-specific rules may still be unspecified.",
    artifact.scope_note || "Runs here using supplied text. Business systems are not connected.",
    ...(artifact.required_capabilities?.some(c => integrationLabels[c]) ? [`Requested capabilities still to connect: ${artifact.required_capabilities.filter(c => integrationLabels[c]).map(c => integrationLabels[c]).join(", ")}.`] : []),
    score ? `Checks: ${score.passed} passed, ${score.failed} need attention, ${score.unknown} unassessed. These checks do not establish reliability outside the tried situations.` : "No completed check report is included.",
    "I'd like to discuss building and setting this up for my business, including the required integrations, deployment and ongoing operation.",
  ].join("\n\n");
}
const integrationLabels: Record<string, string> = {live_search:"live web research",audio_transcription:"audio transcription",database_execution:"database access and query execution",business_action:"business-system actions"};
export function enquiryEmailLink(email: string, summary: string) {
  if (!email) return undefined;
  const href = `mailto:${encodeURIComponent(email)}?subject=${encodeURIComponent("AgentClash project enquiry")}&body=${encodeURIComponent(summary)}`;
  return href.length <= 1800 ? href : undefined;
}
