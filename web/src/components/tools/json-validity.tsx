"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  type ReactNode,
} from "react";

type ReportFn = (id: string, invalid: boolean) => void;

const JsonValidityContext = createContext<ReportFn | null>(null);

/**
 * Tracks JSON editors that currently hold unparseable text. `onChange` fires with
 * `true` whenever at least one editor is invalid, so the builder can block saving
 * stale/inconsistent definitions.
 */
export function JsonValidityProvider({
  onChange,
  children,
}: {
  onChange: (hasInvalid: boolean) => void;
  children: ReactNode;
}) {
  const invalidIds = useRef<Set<string>>(new Set());
  const report = useCallback<ReportFn>(
    (id, invalid) => {
      const had = invalidIds.current.size > 0;
      if (invalid) invalidIds.current.add(id);
      else invalidIds.current.delete(id);
      const has = invalidIds.current.size > 0;
      if (has !== had) onChange(has);
    },
    [onChange],
  );
  return (
    <JsonValidityContext.Provider value={report}>{children}</JsonValidityContext.Provider>
  );
}

/** Report this editor's parse validity to the nearest provider (no-op if none). */
export function useReportJsonValidity(id: string, invalid: boolean) {
  const report = useContext(JsonValidityContext);
  useEffect(() => {
    report?.(id, invalid);
    return () => report?.(id, false);
  }, [report, id, invalid]);
}
