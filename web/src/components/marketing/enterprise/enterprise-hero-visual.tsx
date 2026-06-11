import { ShieldCheck } from "lucide-react";

const gateRows = [
  { agent: "baseline-gpt-4.1", score: "94", status: "pass", delta: "baseline" },
  { agent: "candidate-claude", score: "91", status: "pass", delta: "+2 cost" },
  { agent: "vendor-preview", score: "78", status: "fail", delta: "-12 correctness" },
];

export function EnterpriseHeroVisual() {
  return (
    <div className="relative overflow-hidden rounded-xl border border-white/[0.1] bg-[#0a0a0a] p-1 shadow-[0_24px_80px_rgba(0,0,0,0.45)]">
      <div className="rounded-[10px] border border-white/[0.06] bg-gradient-to-b from-white/[0.04] to-transparent px-5 py-4">
        <div className="flex items-center justify-between gap-4 border-b border-white/[0.06] pb-4">
          <div>
            <p className="font-mono text-[10px] uppercase tracking-[0.16em] text-white/40">
              Release gate preview
            </p>
            <p className="mt-1 text-sm font-medium text-white/90">
              refund-recovery-v3
            </p>
          </div>
          <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-400/25 bg-amber-400/10 px-2.5 py-1 font-mono text-[10px] uppercase tracking-[0.12em] text-amber-200">
            <ShieldCheck className="size-3" />
            Block candidate
          </span>
        </div>
        <div className="mt-4 space-y-2">
          {gateRows.map((row) => (
            <div
              key={row.agent}
              className="flex items-center justify-between gap-3 rounded-md border border-white/[0.06] bg-black/40 px-3 py-2.5"
            >
              <div className="min-w-0">
                <p className="truncate font-mono text-[11px] text-white/75">
                  {row.agent}
                </p>
                <p className="mt-0.5 text-[10px] text-white/35">{row.delta}</p>
              </div>
              <div className="flex items-center gap-3 shrink-0">
                <span className="font-mono text-sm tabular-nums text-white/80">
                  {row.score}
                </span>
                <span
                  className={`rounded px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-[0.1em] ${
                    row.status === "pass"
                      ? "bg-emerald-400/10 text-emerald-300"
                      : "bg-red-400/10 text-red-300"
                  }`}
                >
                  {row.status}
                </span>
              </div>
            </div>
          ))}
        </div>
        <p className="mt-4 text-[11px] leading-relaxed text-white/40">
          Scorecard, replay, and policy checks in one gate verdict your team can
          export.
        </p>
      </div>
    </div>
  );
}
