import Link from "next/link";
import type { ReactNode } from "react";

export function ChangelogShell({
  children,
  backHref = "/changelog",
  backLabel = "All releases",
}: {
  children: ReactNode;
  backHref?: string;
  backLabel?: string;
}) {
  return (
    <main className="min-h-screen px-6 py-14 sm:py-16">
      <div className="mx-auto w-full max-w-2xl">{children}</div>
      <div className="mx-auto mt-12 w-full max-w-2xl">
        <Link
          href={backHref}
          className="text-xs text-white/35 transition-colors hover:text-white/55"
        >
          &larr; {backLabel}
        </Link>
      </div>
    </main>
  );
}
