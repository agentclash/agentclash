"use client";
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type SetStateAction } from "react";
import { VibeError, vibeFetch } from "./vibe";
export type CollectionStatus = "loading" | "ready" | "signed_out" | "forbidden" | "failed";

// Rows belong to an actor/workspace scope. Transient retry failures retain that
// scope's last success; access loss and scope changes immediately hide old rows.
export function useVibeCollection<T>(path: string, scope: string, token: () => Promise<string | undefined>, enabled: boolean, requireAuth: boolean) {
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<{ scope: string; attempt: number; rows: T[]; status: CollectionStatus }>({ scope, attempt, rows: [], status: "loading" });
  const currentScope = useRef(scope);
  useLayoutEffect(() => { currentScope.current = scope; }, [scope]);
  const retry = useCallback(() => setAttempt(value => value + 1), []);
  const setRows = useCallback((next: SetStateAction<T[]>) => setState(old => currentScope.current !== scope ? old : ({ scope, attempt, status: "ready", rows: typeof next === "function" ? next(old.scope === scope ? old.rows : []) : next })), [scope, attempt]);
  useEffect(() => {
    if (!enabled) return;
    let live = true;
    void token().then(async auth => {
      if (requireAuth && !auth) throw new VibeError("unauthorized", "Sign in to load saved work.", 401);
      return vibeFetch<T[]>(path, auth);
    }).then(rows => { if (live && currentScope.current === scope) setState({ scope, attempt, rows: Array.isArray(rows) ? rows : [], status: "ready" }); })
      .catch(error => {
        if (!live || currentScope.current !== scope) return;
        const status: CollectionStatus = error instanceof VibeError && error.status === 401 ? "signed_out" : error instanceof VibeError && error.status === 403 ? "forbidden" : "failed";
        setState(old => ({ scope, attempt, status, rows: status === "failed" && old.scope === scope ? old.rows : [] }));
      });
    return () => { live = false; };
  }, [path, scope, token, enabled, requireAuth, attempt]);
  return { rows: state.scope === scope ? state.rows : [], status: state.scope === scope && state.attempt === attempt ? state.status : "loading" as CollectionStatus, retry, setRows };
}
