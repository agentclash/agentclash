"use client";
import { useRef, useState } from "react";
import { useAccessToken, useAuth } from "@workos-inc/authkit-nextjs/components";
import { safeVibeReturn } from "@/lib/vibe-keep";
import { vibeBytes, vibeFetch } from "@/lib/vibe";
import { downloadBytes } from "@/lib/vibe-export";
import { Button } from "@/components/ui/button";
export function VibeReturn({
  path,
  draft,
  title,
}: { path?: string; draft: string; title?: string }) {
  const safe = safeVibeReturn(path),
    { user } = useAuth(),
    { getAccessToken } = useAccessToken();
  const owner = useRef({ user: user?.id, draft });
  if (owner.current.user !== user?.id || owner.current.draft !== draft)
    owner.current = { user: user?.id, draft };
  const [state, setState] = useState<{ owner: object; error: string; busy: boolean }>();
  const active = state?.owner === owner.current ? state : undefined;
  const busy = !!active?.busy,
    error = active?.error;
  async function original() {
    const captured = owner.current;
    setState({ owner: captured, busy: true, error: "" });
    try {
      const auth = await getAccessToken();
      if (captured !== owner.current || !auth) return;
      const metadata = await vibeFetch<{ filename: string; media_type: string }>(
        `/saved-originals/${draft}?metadata=1`,
        auth,
      );
      if (captured !== owner.current) return;
      const bytes = await vibeBytes(`/saved-originals/${draft}`, auth);
      if (captured !== owner.current) return;
      downloadBytes(metadata.filename, bytes, metadata.media_type);
    } catch (e) {
      if (captured === owner.current)
        setState({ owner: captured, busy: false, error: (e as Error).message });
    } finally {
      if (captured === owner.current) setState((old) => (old ? { ...old, busy: false } : old));
    }
  }
  if (!safe) return null;
  return (
    <aside className="mb-4 flex flex-wrap items-center gap-3 text-sm">
      <a href={safe} className="underline break-words">
        Return to {title || "your Vibe project"}
      </a>
      <Button variant="outline" disabled={busy} onClick={() => void original()}>
        Download retained original
      </Button>
      <p>
        Editing this workspace copy does not change the original Vibe version. Saving and returning
        do not deploy or run it.
      </p>
      {error && <p role="alert">{error}</p>}
    </aside>
  );
}
