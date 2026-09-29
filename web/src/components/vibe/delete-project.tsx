"use client";

import { useRef, useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { vibeFetch, type Session } from "@/lib/vibe";
import { useVibeConnection } from "@/lib/vibe-connection";
import { VibeButton } from "./vibe-button";

export function DeleteProject({ session, onClose, onDeleted }: { session: Session; onClose: () => void; onDeleted: (id: string) => void }) {
  const { token } = useVibeConnection();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [receipt, setReceipt] = useState<string>();
  const sending = useRef(false);
  async function erase() {
    if (sending.current) return;
    sending.current = true; setBusy(true); setError("");
    try {
      const result = await vibeFetch<{ status: string }>(`/sessions/${session.id}?revision=${session.revision}`, await token(), { method: "DELETE" });
      setReceipt(result.status);
      // Only local drafts belonging to this project are erased.
      try { for (const storage of [sessionStorage, localStorage]) {
        for (const key of Object.keys(storage)) {
          if (key === `vibe-build-drafts:${session.id}` || key.startsWith(`vibe-materials:${session.id}:`) || key.startsWith(`vibe-enquiry:${session.id}:`)) storage.removeItem(key);
        }
      } } catch { /* Server deletion succeeded even if local storage is blocked. */ }
      onDeleted(session.id);
    } catch (e) { setError(`${(e as Error).message} Retrying this deletion will not affect another project.`); }
    finally { sending.current = false; setBusy(false); }
  }
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose(); }}>
    <DialogContent className="vibe-workspace sm:max-w-lg">
      <DialogTitle>{receipt ? "Project removed" : "Delete project and its data?"}</DialogTitle>
      <DialogDescription>{receipt
        ? "New access and runs are blocked. Stored files and remaining content are being removed."
        : "This removes this project’s files, conversation, versions and results, stops pending work and cancels unsent enquiries. It does not delete other projects."}</DialogDescription>
      {!receipt && <p className="text-sm vibe-muted">Separate workspace saves and downloaded copies remain. An email already sent or in flight cannot be recalled. Minimal cost records remain for billing; provider retention also applies.</p>}
      {error && <p role="alert">{error}</p>}
      <div className="flex flex-wrap gap-2">{receipt ? <VibeButton onClick={onClose}>Done</VibeButton> : <>
        <VibeButton disabled={busy} onClick={() => void erase()}>{busy ? "Deleting…" : "Delete project permanently"}</VibeButton>
        <VibeButton variant="quiet" disabled={busy} onClick={onClose}>Cancel</VibeButton>
      </>}</div>
    </DialogContent>
  </Dialog>;
}
