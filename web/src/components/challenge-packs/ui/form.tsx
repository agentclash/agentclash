"use client";

// Quiet-utilitarian form primitives for the challenge-pack builder: warm,
// near-monochrome, mono-forward, with the accent (--builder-warn) reserved for
// errors only. Kept separate from components/tools/field.tsx so the (shared)
// tool builder is unaffected by the builder's bespoke styling.

import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export const controlClass =
  "block w-full rounded-md border border-builder-border bg-builder-surface px-3 py-2 text-sm text-builder-fg transition-colors placeholder:text-builder-fg-faint hover:border-builder-border-strong focus:border-builder-fg-muted focus:bg-builder-surface-hover focus:outline-none disabled:cursor-not-allowed disabled:opacity-50";

export const monoControlClass = cn(
  controlClass,
  "font-[family-name:var(--font-mono)] text-xs leading-relaxed",
);

/** Section title + one-line description that opens every editor. */
export function EditorHeader({
  title,
  description,
}: {
  title: string;
  description?: ReactNode;
}) {
  return (
    <div className="space-y-1">
      <h2 className="text-[0.95rem] font-medium tracking-tight text-builder-fg">{title}</h2>
      {description && (
        <p className="text-sm leading-relaxed text-builder-fg-muted">{description}</p>
      )}
    </div>
  );
}

/** Stacked label/control — use for textareas and anything multi-line. */
export function Field({
  label,
  hint,
  error,
  htmlFor,
  children,
  className,
}: {
  label?: ReactNode;
  hint?: ReactNode;
  error?: string;
  htmlFor?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={className}>
      {label && (
        <label
          htmlFor={htmlFor}
          className="mb-1.5 block text-xs font-medium lowercase text-builder-fg-subtle"
        >
          {label}
        </label>
      )}
      {hint && <p className="mb-1.5 text-xs text-builder-fg-faint">{hint}</p>}
      {children}
      {error && <p className="mt-1.5 text-xs text-builder-warn">{error}</p>}
    </div>
  );
}

/**
 * Aligned `label … control` row — the signature compact, scannable look for
 * single-line fields. Collapses to a stacked layout on narrow viewports.
 */
export function FieldRow({
  label,
  hint,
  error,
  htmlFor,
  children,
  className,
}: {
  label?: ReactNode;
  hint?: ReactNode;
  error?: string;
  htmlFor?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "grid grid-cols-1 gap-x-4 gap-y-1.5 sm:grid-cols-[7.5rem_1fr] sm:items-start",
        className,
      )}
    >
      {label && (
        <label
          htmlFor={htmlFor}
          className="text-xs font-medium lowercase text-builder-fg-subtle sm:pt-2.5"
        >
          {label}
        </label>
      )}
      <div className="min-w-0">
        {children}
        {hint && <p className="mt-1.5 text-xs text-builder-fg-faint">{hint}</p>}
        {error && <p className="mt-1.5 text-xs text-builder-warn">{error}</p>}
      </div>
    </div>
  );
}
