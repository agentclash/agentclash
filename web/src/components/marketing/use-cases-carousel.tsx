"use client";

import { useState } from "react";
import {
  type LucideIcon,
  Activity,
  BookOpen,
  Code2,
  Workflow,
} from "lucide-react";
import { SpectraNoise } from "./spectra-noise";

export type UseCase = {
  label: string;
  brief: string;
  captures: string;
  hueShift: number;
};

const ICONS: LucideIcon[] = [Code2, BookOpen, Activity, Workflow];

export function UseCasesCarousel({ items }: { items: UseCase[] }) {
  // Index of the currently-open card on desktop (hover/focus). Defaults to
  // the first card so the section never reads as fully collapsed.
  const [active, setActive] = useState(0);

  return (
    <>
      {/* Desktop: horizontal accordion. One card expands, the others stay as
          slim spines showing only a number + glyph. */}
      <div
        className="hidden gap-3 sm:flex"
        onMouseLeave={() => setActive(0)}
      >
        {items.map((u, i) => {
          const Icon = ICONS[i % ICONS.length];
          const open = active === i;
          return (
            <button
              key={u.label}
              type="button"
              onMouseEnter={() => setActive(i)}
              onFocus={() => setActive(i)}
              onClick={() => setActive(i)}
              aria-expanded={open}
              className={`group relative flex h-[420px] shrink-0 cursor-pointer flex-col overflow-hidden rounded-2xl glass-card glass-shine text-left transition-[flex] duration-500 ease-out ${
                open ? "flex-[3.2]" : "flex-[1]"
              }`}
              style={{ backgroundColor: "rgba(255,255,255,0.04)" }}
            >
              <SpectraNoise
                hueShift={u.hueShift}
                speed={0.4}
                warpAmount={0.3}
                noiseIntensity={0.06}
                resolutionScale={0.55}
                className="absolute inset-0"
              />
              <div
                aria-hidden
                className="absolute inset-0"
                style={{
                  background:
                    "linear-gradient(180deg, rgba(6,6,6,0.10) 0%, rgba(6,6,6,0.55) 55%, rgba(6,6,6,0.92) 100%)",
                }}
              />

              <span className="relative z-10 px-5 pt-5 text-base font-medium tabular-nums text-white/85">
                {String(i + 1).padStart(2, "0")}
              </span>

              <div className="relative z-10 mt-auto flex flex-col gap-4 p-5">
                <div
                  className={`transition-opacity duration-300 ${
                    open ? "opacity-100 delay-150" : "pointer-events-none opacity-0"
                  }`}
                >
                  <p className="max-w-[44ch] text-[15px] leading-[1.55] text-white">
                    {u.brief}
                  </p>
                  <p className="mt-3 max-w-[44ch] text-[13px] leading-[1.55] text-white/55">
                    {u.captures}
                  </p>
                </div>

                <div className="flex items-center gap-2.5 text-white/85">
                  <Icon className="size-[18px] shrink-0" strokeWidth={1.5} />
                  <span
                    className={`whitespace-nowrap text-[15px] font-medium transition-opacity duration-300 ${
                      open ? "opacity-100 delay-150" : "opacity-0"
                    }`}
                  >
                    {u.label}
                  </span>
                </div>
              </div>
            </button>
          );
        })}
      </div>

      {/* Mobile: stacked cards, each tap-to-expand. Sized for one-handed
          phone use; no horizontal scroll. */}
      <div className="flex flex-col gap-3 sm:hidden">
        {items.map((u, i) => (
          <MobileCard key={u.label} useCase={u} index={i} icon={ICONS[i % ICONS.length]} />
        ))}
      </div>
    </>
  );
}

function MobileCard({
  useCase,
  index,
  icon: Icon,
}: {
  useCase: UseCase;
  index: number;
  icon: LucideIcon;
}) {
  const [open, setOpen] = useState(index === 0);

  return (
    <article
      className="relative overflow-hidden rounded-2xl glass-card glass-shine"
      style={{ backgroundColor: "rgba(255,255,255,0.04)" }}
    >
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className="relative flex w-full items-center gap-3 p-4 text-left"
      >
        <div className="absolute inset-0 -z-0">
          <SpectraNoise
            hueShift={useCase.hueShift}
            speed={0.4}
            warpAmount={0.3}
            noiseIntensity={0.06}
            resolutionScale={0.5}
          />
          <div
            aria-hidden
            className="absolute inset-0"
            style={{
              background:
                "linear-gradient(90deg, rgba(6,6,6,0.55) 0%, rgba(6,6,6,0.85) 100%)",
            }}
          />
        </div>

        <span className="relative z-10 text-sm tabular-nums text-white/70">
          {String(index + 1).padStart(2, "0")}
        </span>
        <Icon className="relative z-10 size-[18px] text-white/85" strokeWidth={1.5} />
        <span className="relative z-10 flex-1 text-[15px] font-medium text-white">
          {useCase.label}
        </span>
        <span
          aria-hidden
          className={`relative z-10 text-white/55 transition-transform duration-300 ${
            open ? "rotate-45" : ""
          }`}
        >
          +
        </span>
      </button>

      <div
        className={`grid transition-[grid-template-rows] duration-300 ease-out ${
          open ? "grid-rows-[1fr]" : "grid-rows-[0fr]"
        }`}
      >
        <div className="overflow-hidden">
          <div className="border-t border-white/10 p-4">
            <p className="text-[14px] leading-[1.55] text-white/90">
              {useCase.brief}
            </p>
            <p className="mt-3 text-[12.5px] leading-[1.55] text-white/55">
              {useCase.captures}
            </p>
          </div>
        </div>
      </div>
    </article>
  );
}
