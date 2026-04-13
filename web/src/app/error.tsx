"use client";

import { useEffect } from "react";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Unhandled error:", error);
  }, [error]);

  return (
    <main className="min-h-screen flex flex-col items-center justify-center px-6 py-16 text-center">
      <p className="font-[family-name:var(--font-mono)] text-xs text-white/30 tracking-widest uppercase">
        Something went wrong
      </p>
      <h1 className="mt-3 font-[family-name:var(--font-display)] text-3xl sm:text-4xl tracking-[-0.02em]">
        Unexpected error
      </h1>
      <p className="mt-4 text-sm text-white/40 max-w-xs leading-relaxed">
        An error occurred while loading this page. Please try again.
      </p>
      <button
        onClick={reset}
        className="mt-8 text-xs text-white/40 hover:text-white/60 transition-colors underline underline-offset-4"
      >
        Try again
      </button>
    </main>
  );
}
