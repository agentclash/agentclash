import type { ReportContent } from "@/lib/report-snapshots";
// Only the immutable copy is accepted here. Neither recipient surface reads a source.
export function ReportSnapshotView({ content }: {
    content: ReportContent;
}) {
    return <article aria-label="Selected report" className="min-w-0 space-y-5">
    <header><h2 className="text-lg font-semibold">Selected report</h2><p>{content.status} · {content.origin === "imported" ? "Imported SDK claims" : content.origin}</p>
      <p className="text-sm text-muted-foreground">An independent workspace copy. Original grades are unchanged. Source deletion does not remove this copy or its public links.</p>
      <p>{content.omitted_cases} cases omitted</p></header>
    {content.cases.map(c => <section key={c.entry_id} className="space-y-3 rounded-lg border p-4">
      <h3 className="font-medium">{c.label} · {c.status}</h3><p>{c.capture_assurance}</p>
      <details><summary>Original source capture</summary><p className="text-sm">Capture describes the source, not this selected subset.</p>
        {Object.entries(c.capture_scope).map(([dimension, capture]) => <p key={dimension}>{dimension}: {capture.scope} · {capture.instrumentation}</p>)}
      </details><p className="text-sm text-muted-foreground">{c.omitted_entries} entries omitted</p>
      {c.entries.map(e => <section key={e.entry_id} className="space-y-2 border-t pt-3"><h4 className="font-medium">{e.kind === "grade" ? "Original grade · " : ""}{e.label}</h4>
        {e.facts.map((fact, i) => <p key={i} className="break-words [overflow-wrap:anywhere]">{fact.label}: <span className="font-mono">{fact.value}</span></p>)}
        {e.text_selected && <p className="text-sm text-muted-foreground">Source text explicitly included</p>}
        {e.text.map((fact, i) => <div key={i}><p className="text-sm font-medium">{fact.label}</p><pre className="whitespace-pre-wrap break-words text-sm [overflow-wrap:anywhere]">{fact.value}</pre></div>)}
        {e.facts.length === 0 && e.text.length === 0 && <p className="text-sm text-muted-foreground">No text or scalar facts selected.</p>}
      </section>)}
    </section>)}
    <details><summary>Selection and source identity</summary>{content.omissions.map((note, i) => <p key={i} className="text-sm">{note}</p>)}{content.fingerprints && <div><p>Retained platform fingerprints (unknown where absent)</p>{Object.entries(content.fingerprints).map(([key, value]) => <p key={key} className="break-all text-xs">{key}: {value || "unknown"}</p>)}</div>}<p className="break-all text-xs">Evidence hash: {content.evidence_hash}</p><p className="break-all text-xs">Source hash: {content.source_hash}</p></details>
  </article>;
}
