"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ArrowRight } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { ClashMark } from "@/components/marketing/clash-mark";

/**
 * Coming-soon gate for the public Agent Tryouts page.
 *
 * Shown on every visit (state is intentionally not persisted) while tryouts are
 * not yet open to the general public. Remove this component — and its render in
 * `page.tsx` — when tryouts go GA.
 */
export function ComingSoonDialog() {
  const router = useRouter();
  const [open, setOpen] = useState(true);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent showCloseButton className="sm:max-w-md">
        <div className="flex flex-col gap-4 py-1">
          <div className="flex items-center gap-2 text-white/70">
            <ClashMark className="size-6" />
            <span className="font-[family-name:var(--font-mono)] text-2xs uppercase tracking-[0.18em] text-white/40">
              Agent Tryouts
            </span>
          </div>

          <DialogTitle className="font-[family-name:var(--font-display)] text-2xl tracking-[-0.01em]">
            Coming soon
          </DialogTitle>

          <DialogDescription className="text-sm leading-relaxed text-white/55">
            Agent Tryouts isn&apos;t open to the general public just yet.
            We&apos;re putting the finishing touches on it so you can run a
            sandboxed agent on your own workflow and get a scored verdict before
            you ship. Check back shortly.
          </DialogDescription>

          <div className="mt-1 flex flex-col gap-2 sm:flex-row">
            <Button
              onClick={() => router.push("/benchmarks")}
              className="group inline-flex items-center gap-1.5"
            >
              Explore benchmarks
              <ArrowRight className="size-3.5 transition-transform group-hover:translate-x-0.5" />
            </Button>
            <Button variant="outline" onClick={() => router.push("/")}>
              Back to home
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
