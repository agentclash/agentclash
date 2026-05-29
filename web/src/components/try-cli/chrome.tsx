import Link from "next/link";

/**
 * Shared visual chrome for the Try CLI surface, matching the AgentClash landing
 * aesthetic: near-black canvas, a faint dotted grid that fades at the edges,
 * Instrument Serif wordmark.
 */

export function GridBackdrop({ className = "" }: { className?: string }) {
  return (
    <div
      aria-hidden
      className={`pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(circle_at_1px_1px,rgba(255,255,255,0.06)_1px,transparent_0)] [background-size:14px_14px] [mask-image:radial-gradient(ellipse_120%_80%_at_50%_-10%,#000_30%,transparent_75%)] ${className}`}
    />
  );
}

export function ClashMark({ className = "" }: { className?: string }) {
  // Solid fills with per-triangle opacity — no gradient <defs>/id, so the mark
  // is safe to render more than once on a page without id collisions.
  return (
    <svg viewBox="0 0 512 512" className={className} role="img" aria-label="AgentClash">
      <path d="M232 256 96 136v240z" fill="#fff" fillOpacity="0.85" />
      <path d="M280 256 416 136v240z" fill="#fff" fillOpacity="0.5" />
    </svg>
  );
}

export function Wordmark() {
  return (
    <Link href="/" className="inline-flex items-center gap-2.5 text-white/90">
      <ClashMark className="size-5" />
      <span className="font-[family-name:var(--font-display)] text-xl tracking-[-0.01em]">
        AgentClash
      </span>
    </Link>
  );
}
