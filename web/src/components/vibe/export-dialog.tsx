"use client";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { VibeButton } from "./vibe-button";
import { unresolvedCase } from "@/lib/vibe-result-evidence";
import type { useVibeExport } from "@/lib/use-vibe-export";
export function ExportDialog({ controller: c }: { controller: ReturnType<typeof useVibeExport> }) {
  const state = c.state,
    p = state?.job?.preparation,
    missing = state?.job?.missing;
  return (
    <Dialog
      open={!!state?.open}
      onOpenChange={(open) => {
        if (!open) c.close();
      }}
    >
      <DialogContent className="vibe-workspace vibe-dialog">
        <DialogTitle className="[overflow-wrap:anywhere]">
          Export {p?.title || "selected evidence"}
        </DialogTitle>
        <DialogDescription>
          The chosen version and result stay fixed while this copy is prepared. Review the captured
          evidence before downloading.
        </DialogDescription>
        <div className="vibe-dialog-body space-y-3">
          {p && (
            <>
              <p>
                Version {p.artifact_id.slice(0, 8)} ·{" "}
                {p.selection.result_id ? `Run ${p.selection.result_id.slice(0, 8)}` : "Definition"}{" "}
                · {p.selection.scope === "history" ? "Full history" : "Selected evidence"}
              </p>
              <p>
                Captured {new Date(p.captured_at).toLocaleString()}.{" "}
                {p.runs
                  .map(
                    (run) =>
                      `${run.result_scope?.case_keys.length ?? "Unknown"} selected cases; ${run.scorecard?.failed || 0} failed, ${run.results.filter((result) => unresolvedCase({ ...result, checks: result.checks || [] })).length} unresolved saved cases`,
                  )
                  .join("; ")}
              </p>
              <p className="vibe-muted">
                Failed and unresolved counts can overlap. Missing bodies remain listed separately
                below.
              </p>
              {p.omissions.map((text) => (
                <p key={text} className="vibe-muted">
                  {text}
                </p>
              ))}
              <p role="status">
                {state!.job!.values.size} of {p.items.length} evidence items prepared
                {state?.busy ? "…" : "."}
              </p>
            </>
          )}
          {state?.error && <p role="alert">{state.error}</p>}
          {!!missing?.size && !state?.busy && (
            <>
              <p>
                Some chosen evidence is missing. A partial export names every missing item and
                retains the original totals.
              </p>
              <ul>
                {Array.from(missing).map(([key, reason]) => (
                  <li className="break-words" key={key}>
                    {key}: {reason}
                  </li>
                ))}
              </ul>
              <VibeButton onClick={() => void c.retry()}>Retry missing evidence</VibeButton>
              <VibeButton onClick={() => c.download(true)}>
                Confirm and download partial export
              </VibeButton>
            </>
          )}
          {p && !state?.busy && !missing?.size && (
            <VibeButton onClick={() => c.download()}>Download exact export</VibeButton>
          )}
          {state?.intent && (
            <>
              <VibeButton
                disabled={state.busy}
                onClick={() =>
                  void c.start(state.intent!.artifact, state.intent!.operation, "selected")
                }
              >
                Reconcile and prepare selected evidence again
              </VibeButton>
              <VibeButton
                disabled={state.busy}
                onClick={() =>
                  void c.start(state.intent!.artifact, state.intent!.operation, "history")
                }
              >
                Prepare full project history
              </VibeButton>
            </>
          )}
          <VibeButton variant="quiet" onClick={c.close}>
            Close
          </VibeButton>
        </div>
      </DialogContent>
    </Dialog>
  );
}
