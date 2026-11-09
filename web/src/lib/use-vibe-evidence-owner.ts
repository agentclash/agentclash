"use client";
import { useCallback, useLayoutEffect, useMemo, useRef } from "react";
// Capture at the beginning of each request/effect. A committed identity change
// or StrictMode cleanup invalidates its lease; replay obtains a fresh lease.
export function useVibeEvidenceOwner(...identity: (string | undefined)[]) {
  const key = JSON.stringify(identity);
  const owner = useMemo(() => ({ key }), [key]);
  const active = useRef<{ owner: object } | undefined>(undefined);
  useLayoutEffect(() => {
    const lease = { owner }; active.current = lease;
    return () => { if (active.current === lease) active.current = undefined; };
  }, [owner]);
  const current = useCallback(() => active.current?.owner === owner, [owner]);
  const capture = useCallback(() => {
    const lease = active.current;
    return () => !!lease && lease.owner === owner && active.current === lease;
  }, [owner]);
  return useMemo(() => ({ owner, current, capture }), [owner, current, capture]);
}
