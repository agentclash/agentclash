"use client";

import type { RuleCoverage } from "@/lib/vibe";
import { VibeButton } from "./vibe-button";

export function CoverageNote({ rows, busy, onSuggest }: {
  rows: RuleCoverage[];
  busy: boolean;
  onSuggest: (rule: string) => void;
}) {
  if (!rows.length) return null;
  const gap = rows.find(row => !row.case_keys.length);
  return (
    <details className="mt-5 text-sm vibe-muted">
      <summary className="w-fit cursor-pointer rounded-sm focus-visible:outline-2 focus-visible:outline-offset-4">Which rules have examples?</summary>
      <p className="mt-3">These links show what the examples cover. They don’t prove your agent handles every situation.</p>
      <ul className="mt-3 space-y-2">
        {rows.map(row => <li key={row.rule_id} className="flex flex-wrap justify-between gap-x-4 gap-y-1">
          <span className="min-w-0 break-words">{row.statement}</span>
          <span className="text-xs">{row.case_keys.length ? `${row.case_keys.length} linked ${row.case_keys.length === 1 ? "example" : "examples"}` : "No linked example yet"}</span>
        </li>)}
      </ul>
      {gap && <div className="mt-4">
        <p>One thing to explore next: {gap.statement}</p>
        <VibeButton variant="quiet" className="mt-2" disabled={busy} onClick={() => onSuggest(gap.statement)}>Draft a request for one more test</VibeButton>
      </div>}
    </details>
  );
}
