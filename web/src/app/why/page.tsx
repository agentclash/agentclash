import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { ClashMark } from "@/components/marketing/clash-mark";
import { whyMetadata } from "./metadata";

export const metadata = whyMetadata;

export default function WhyWeBuiltThisPage() {
  return (
    <main className="min-h-screen bg-[#060606] text-white">
      <header className="px-5 sm:px-12 py-5 sm:py-6 border-b border-white/[0.06]">
        <div className="mx-auto flex max-w-[1440px] items-center justify-between">
          <Link
            href="/"
            className="inline-flex items-center gap-2.5 text-white/90"
          >
            <ClashMark className="size-6" />
            <span className="font-[family-name:var(--font-display)] text-xl tracking-[-0.01em]">
              AgentClash
            </span>
          </Link>
          <Link
            href="/"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs text-white/55 hover:text-white/85 transition-colors"
          >
            <ArrowLeft className="size-3.5" />
            Back
          </Link>
        </div>
      </header>

      <section className="px-8 sm:px-12 py-32 sm:py-48">
        <div className="mx-auto max-w-[1440px]">
          <p className="font-mono text-[0.66rem] uppercase tracking-[0.28em] text-white/45">
            Why we built this
          </p>

          <h1 className="mt-6 font-[family-name:var(--font-display)] font-normal tracking-[-0.03em] leading-[1.02] text-[clamp(2.5rem,6vw,5.5rem)] max-w-[22ch]">
            We got tired of being lied to.
          </h1>

          <p className="mt-24 font-[family-name:var(--font-display)] font-normal tracking-[-0.02em] leading-[1.1] text-[clamp(1.875rem,4.2vw,3.5rem)] text-white/90 max-w-[30ch]">
            It passed every eval we had. It failed in week one.
          </p>

          <p className="mt-16 font-[family-name:var(--font-display)] font-normal tracking-[-0.02em] leading-[1.15] text-[clamp(1.625rem,3.2vw,2.75rem)] text-white/60 max-w-[34ch]">
            None of the benchmarks had touched our task.
          </p>

          <p className="mt-24 font-[family-name:var(--font-display)] font-normal tracking-[-0.025em] leading-[1.05] text-[clamp(2.125rem,5vw,4.25rem)] text-white/95 max-w-[30ch]">
            The only eval you can trust is the one you ran yourself —
            your task, every model, at the same time.
          </p>

          <p className="mt-16 font-[family-name:var(--font-display)] font-normal tracking-[-0.02em] leading-[1.15] text-[clamp(1.625rem,3.2vw,2.75rem)] text-white/90 max-w-[24ch]">
            AgentClash is that eval.
          </p>

          <p className="mt-24 max-w-[56ch] text-[15px] leading-[1.7] text-white/50">
            Pick your task the way your product actually runs it. Six
            models race, live, on the same inputs with the same tools.
            Scored on what matters in production — correctness, cost,
            latency, behaviour under pressure. When one fails, the failing
            trace becomes a test. Every mistake ratchets the eval tighter.
          </p>

          <p className="mt-20 font-[family-name:var(--font-display)] font-normal tracking-[-0.025em] leading-[1.05] text-[clamp(1.875rem,4.5vw,3.5rem)] text-white/95 max-w-[26ch]">
            Your task. Your models. Your scoreboard.
          </p>

          <div className="mt-24 flex flex-col sm:flex-row gap-3">
            <Link
              href="/auth/login"
              className="inline-flex items-center justify-center gap-2 rounded-md bg-white px-7 py-3 text-sm font-medium text-[#060606] hover:bg-white/90 transition-colors"
            >
              Start your first race
            </Link>
            <Link
              href="/"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-white/15 bg-white/[0.04] px-7 py-3 text-sm font-medium text-white/80 hover:text-white hover:border-white/30 transition-colors"
            >
              Back to home
            </Link>
          </div>
        </div>
      </section>
    </main>
  );
}
