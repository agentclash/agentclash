// Uploads, saved attachments and restoration share this policy at commit time.
export function selectMaterials<T extends { material: { id: string; kind: string } }>(current: readonly T[], incoming: readonly T[]): { items: T[]; error: string } {
  const items = [...current];
  let error = "";
  for (const value of incoming) {
    if (items.some(x => x.material.id === value.material.id)) continue;
    if (items.length >= 2 || value.material.kind === "pdf" && items.some(x => x.material.kind === "pdf")) {
      error = "Use one PDF and optionally one pasted text input. Detach an input first.";
      continue;
    }
    items.push(value);
  }
  return { items, error };
}
