"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { vibeFetch, watchVibe, VibeError, type Session } from "./vibe";

export const sessionAccessLost = "This browser can’t access the saved session. Your unsent message is still here.";
export function isSessionAccessError(error: unknown) {
  return error instanceof VibeError && [401, 403, 404].includes(error.status || 0);
}
function reconcileSession(current: Session | null, incoming: Session, activeID?: string): Session | null {
  if (incoming.id !== activeID) return current;
  if (current?.id === incoming.id && (current.revision > incoming.revision || (current.event_cursor || 0) > (incoming.event_cursor || 0))) return current;
  return incoming;
}

// All network snapshots enter here. Selection is explicit; a response cannot
// select its own context, and HTTP and streaming share the same ordering rule.
export function useVibeSession(id: string | null, token: () => Promise<string | undefined>, authLoading: boolean, attempt: number) {
  const [session, render] = useState<Session | null>(null);
  const snapshot = useRef<Session | null>(null);
  const activeSessionRef = useRef<string | undefined>(id || undefined);
  const [loadingSession, setLoading] = useState(!!id);
  const [connection, setConnection] = useState("");
  const [loadError, setLoadError] = useState("");
  const accept = useCallback((incoming: Session) => {
    const next = reconcileSession(snapshot.current, incoming, activeSessionRef.current);
    if (next !== incoming) return false;
    snapshot.current = next; render(next); return true;
  }, []);
  const select = useCallback((next?: string) => {
    activeSessionRef.current = next;
    if (snapshot.current?.id !== next) { snapshot.current = null; render(null); }
    setConnection(""); setLoadError("");
  }, []);
  const reload = useCallback(async (next = activeSessionRef.current) => {
    if (!next) return;
    const incoming = await vibeFetch<Session>(`/sessions/${next}`, await token());
    return accept(incoming) ? incoming : snapshot.current?.id === next ? snapshot.current : undefined;
  }, [token, accept]);
  useEffect(() => {
    select(id || undefined);
    if (!id || authLoading) { setLoading(!!id); return; }
    let live = true;
    setLoading(true);
    void (async () => {
      const auth = await token();
      try {
        let incoming: Session;
        try { incoming = await vibeFetch<Session>(`/sessions/${id}`, auth); }
        catch (error) {
          if (!auth || !(error instanceof VibeError) || error.code !== "not_found") throw error;
          incoming = await vibeFetch<Session>(`/sessions/${id}/claim`, auth, { method: "POST", body: "{}" });
        }
        if (live) accept(incoming);
      } catch (error) {
        if (live) {
          if (isSessionAccessError(error)) setConnection(sessionAccessLost);
          else setLoadError((error as Error).message);
        }
      } finally { if (live) setLoading(false); }
    })();
    return () => { live = false; };
  }, [id, token, authLoading, attempt, select, accept]);
  useEffect(() => {
    if (!session?.id || authLoading) return;
    const sessionID = session.id;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    let claimed = false;
    const connect = async () => {
      const auth = await token();
      try {
        await watchVibe(sessionID, auth, controller.signal, incoming => {
          if (!controller.signal.aborted && accept(incoming)) setConnection("");
        });
      } catch (error) {
        if (controller.signal.aborted || activeSessionRef.current !== sessionID) return;
        if (auth && error instanceof VibeError && error.code === "not_found" && !claimed) {
          claimed = true;
          try {
            const incoming = await vibeFetch<Session>(`/sessions/${sessionID}/claim`, auth, { method: "POST", body: "{}" });
            if (!controller.signal.aborted && accept(incoming)) { setConnection(""); timer = setTimeout(connect, 0); }
            return;
          } catch { /* Membership or ownership loss stays inaccessible. */ }
        }
        if (isSessionAccessError(error)) { setConnection(sessionAccessLost); return; }
        setConnection("Reconnecting to saved progress…");
      }
      if (!controller.signal.aborted) timer = setTimeout(connect, 2000);
    };
    void connect();
    return () => { controller.abort(); clearTimeout(timer); };
  }, [session?.id, token, authLoading, attempt, accept]);
  return { session, accept, select, reload, activeSessionRef, loadingSession, connection, setConnection, loadError };
}
