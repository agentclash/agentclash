import type { Artifact, CaseResult, Models, ReferenceRequirement, Session } from "./vibe";

// Session snapshots intentionally omit full case bodies. Exports must retrieve
// the authorized evidence instead of silently exporting only the scorecard.
export async function exportRuns(session: Session, load: (id: string, key: string) => Promise<CaseResult>) {
  return Promise.all(session.operations.map(async operation => ({
    ...operation,
    results: await Promise.all(operation.results.map(result => load(operation.id, result.case_key))),
  })));
}

export function downloadJSON(name: string, value: unknown) {
  const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], { type: "application/json" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

// Definition downloads carry requirements, not private input IDs or evidence.
export function exportAgent(artifact: Artifact, models: Models, materials: import("./vibe-inputs").TaskMaterial[] = []) {
  const references: ReferenceRequirement[] = [...(artifact.missing_references || [])];
  for (const binding of artifact.reference_inputs || []) {
    let number = 1;
    while (references.some(reference => reference.key === `reference-${number}`)) number++;
    const material = materials.find(item => item.id === binding.input_id && item.content_hash === binding.content_hash);
    references.push({key: `reference-${number}`, content_hash: binding.content_hash, usage: "reference",
      ...(material ? {name:material.name, format:material.kind, page_count:material.page_count} : {})});
  }
  downloadJSON("agentclash-agent.json", {
    format: "agentclash-vibe-v2", title: artifact.title,
    agent_prompt: artifact.agent_prompt, evaluation: artifact.blueprint,
    input_contract: artifact.input_contract, required_capabilities: artifact.required_capabilities,
    scope_note: artifact.scope_note, sample: artifact.sample, references, models,
  });
}
