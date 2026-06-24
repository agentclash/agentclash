import { ArrowDown, ArrowRight } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

function DiagramFrame({
  caption,
  children,
}: {
  caption?: ReactNode;
  children: ReactNode;
}) {
  return (
    <figure className="not-prose my-10 overflow-x-auto rounded-2xl border border-white/[0.08] bg-black/20 p-6">
      <figcaption className="sr-only">{caption ?? "Diagram"}</figcaption>
      <div className="min-w-0">{children}</div>
    </figure>
  );
}

function Pill({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex max-w-[18rem] items-center justify-center rounded-xl border px-3 py-1.5 text-center text-sm font-medium leading-snug text-white/88",
        "border-white/[0.12] bg-white/[0.04]",
        className,
      )}
    >
      {children}
    </span>
  );
}

function ArrowR({ className }: { className?: string }) {
  return (
    <ArrowRight
      className={cn("size-[18px] shrink-0 text-white/30", className)}
      aria-hidden
    />
  );
}

function ArrowD({ className }: { className?: string }) {
  return (
    <ArrowDown
      className={cn("size-[18px] shrink-0 text-white/30", className)}
      aria-hidden
    />
  );
}

function RowArrowRow({ pills }: { pills: readonly string[] }) {
  return (
    <div className="flex flex-wrap items-center justify-center gap-x-3 gap-y-3 md:flex-nowrap md:justify-start">
      {pills.map((label, i) => (
        <span key={`${label}-${i}`} className="flex shrink-0 items-center gap-x-3">
          {i > 0 ? <ArrowR /> : null}
          <Pill>{label}</Pill>
        </span>
      ))}
    </div>
  );
}

export function DiagramOrchestrationRuntimeSplit() {
  return (
    <DiagramFrame caption="API request into Temporal workflows and worker responsibilities">
      <div className="flex flex-col gap-6">
        <RowArrowRow pills={["Browser or CLI", "API server", "Temporal", "Worker"]} />
        <div className="flex flex-col gap-6 border-t border-white/[0.08] pt-6">
          <p className="text-center text-xs font-semibold uppercase tracking-wider text-white/35 md:text-left">
            Worker surfaces
          </p>
          <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-3 md:flex-nowrap md:justify-start">
            <Pill>Provider router</Pill>
            <ArrowR />
            <Pill>Sandbox provider</Pill>
            <ArrowR />
            <Pill>Run event recorder</Pill>
          </div>
        </div>
      </div>
    </DiagramFrame>
  );
}

export function DiagramSandboxBoundary() {
  return (
    <DiagramFrame caption="Sandbox execution path inside worker activities">
      <RowArrowRow
        pills={[
          "API server",
          "Temporal workflow",
          "Worker activity",
          "Sandbox provider",
          "Agent execution",
          "Replay events and artifacts",
        ]}
      />
    </DiagramFrame>
  );
}

export function DiagramFrontendRouteSplit() {
  return (
    <DiagramFrame caption="Next.js App Router high-level segmentation">
      <div className="mx-auto flex max-w-xl flex-col items-center gap-5">
        <Pill>App Router root</Pill>
        <ArrowD />
        <div className="grid w-full grid-cols-1 gap-3 sm:grid-cols-2">
          <Pill>Public pages</Pill>
          <Pill>Auth routes</Pill>
          <Pill>Authenticated workspace and org routes</Pill>
          <Pill>Docs routes</Pill>
        </div>
      </div>
    </DiagramFrame>
  );
}

export function DiagramWorkspaceDataModel() {
  return (
    <DiagramFrame caption="Core relational edges between workspace entities">
      <div className="mx-auto flex max-w-lg flex-col items-center gap-4">
        <Pill>Workspace</Pill>
        <ArrowD />
        <div className="flex w-full flex-wrap items-center justify-center gap-6 md:justify-between md:gap-16">
          <Pill>Deployment</Pill>
          <span className="text-xs uppercase tracking-[0.2em] text-zinc-600">and</span>
          <Pill>Eval pack</Pill>
        </div>
        <ArrowD />
        <Pill className="ring-[1px] ring-white/20">Run</Pill>
        <ArrowD />
        <div className="flex w-full max-w-xl flex-wrap justify-center gap-3 md:gap-6">
          <Pill>Replay events</Pill>
          <Pill>Artifacts</Pill>
          <Pill>Scorecards</Pill>
        </div>
      </div>
    </DiagramFrame>
  );
}

export function DiagramAgentsToRun() {
  return (
    <DiagramFrame caption="Builds and runtime credentials join the same deployment boundary">
      <div className="mx-auto flex max-w-2xl flex-col gap-10">
        <div className="grid gap-10 md:grid-cols-2 md:gap-14">
          <div className="flex flex-col items-center gap-3 md:items-start">
            <Pill>Agent build</Pill>
            <ArrowD />
            <Pill>Ready build version</Pill>
            <ArrowD className="md:hidden" />
          </div>
          <div className="flex flex-col items-center gap-2 md:items-start">
            <Pill>Runtime profile</Pill>
            <Pill>Provider account</Pill>
            <Pill>Model alias</Pill>
          </div>
        </div>
        <div className="flex flex-col items-center gap-2">
          <div className="hidden h-[2rem] border-l border-dashed border-white/20 md:block" aria-hidden />
          <p className="text-center text-2xs uppercase tracking-[0.18em] text-white/35">
            feed deployment
          </p>
          <ArrowD />
          <Pill className="ring-[1px] ring-white/20">Deployment</Pill>
          <ArrowD />
          <Pill>Run</Pill>
        </div>
        <p className="text-center text-xs leading-relaxed text-white/35">
          Ready build versions and runtime configuration rows converge on the same Deployment—the
          object runs once your eval pack binds through run submission.
        </p>
      </div>
    </DiagramFrame>
  );
}

export function DiagramArtifactFlow() {
  return (
    <DiagramFrame caption="Workspace uploads feed pack assets and runs; replay and scoring consume both">
      <div className="mx-auto flex max-w-xl flex-col items-center gap-8 md:max-w-none">
        <Pill className="max-w-none">Workspace artifact upload</Pill>
        <div className="flex w-full flex-col gap-10 md:flex-row md:justify-center md:gap-24">
          <div className="flex flex-col items-center gap-3 md:flex-1">
            <ArrowD />
            <Pill>Eval pack assets</Pill>
            <ArrowD />
            <Pill>Scoring evidence</Pill>
          </div>
          <div className="flex flex-col items-center gap-3 md:flex-1">
            <ArrowD />
            <Pill>Run</Pill>
            <ArrowD />
            <Pill>Replay events</Pill>
            <ArrowD />
            <Pill>Failure review</Pill>
          </div>
        </div>
      </div>
    </DiagramFrame>
  );
}

export function DiagramReplayVsScorecards() {
  return (
    <DiagramFrame caption="Canonical events branching into granular replay versus aggregate scores">
      <div className="mx-auto flex max-w-2xl flex-col gap-14 md:flex-row md:items-start md:justify-between md:gap-10">
        <div className="flex flex-1 flex-col items-center gap-3 md:items-start">
          <Pill>Canonical events</Pill>
          <ArrowD />
          <Pill>Replay timeline</Pill>
          <ArrowD />
          <Pill>Run detail view</Pill>
        </div>
        <div className="hidden w-px self-stretch bg-white/[0.08] md:block md:mx-10" aria-hidden />
        <div className="flex flex-1 flex-col items-center gap-3 md:items-start">
          <span className="text-2xs font-semibold uppercase tracking-[0.2em] text-white/45 md:-mt-[1rem] md:pb-10">
            also
          </span>
          <Pill className="-mt-[0.5rem] md:mt-0">Canonical events</Pill>
          <ArrowD />
          <Pill>Scorecard</Pill>
          <ArrowD />
          <Pill>Compare and ranking views</Pill>
        </div>
      </div>
    </DiagramFrame>
  );
}

export function DiagramEvidenceClosingLoop() {
  return (
    <DiagramFrame caption="Execution evidence branching into qualitative and quantitative rails">
      <div className="mx-auto grid max-w-5xl gap-16 md:grid-cols-[1fr_18px_1fr] md:items-start md:gap-12 md:justify-items-start">
        <div className="flex flex-col gap-10">
          <RowArrowRow pills={["Execution", "Canonical events"]} />
          <RowArrowRow pills={["Replay views"]} />
          <RowArrowRow pills={["Reviewer understanding"]} />
        </div>
        <div className="hidden min-h-[300px] w-px shrink-0 self-stretch bg-white/[0.08] md:block md:rounded-full" aria-hidden />
        <div className="flex flex-col gap-10">
          <div className="flex flex-wrap gap-x-3 gap-y-3">
            <Pill>Canonical events</Pill>
            <ArrowR />
            <Pill>Artifacts and logs</Pill>
          </div>
          <div className="flex flex-wrap gap-x-3 gap-y-3">
            <Pill>Canonical events</Pill>
            <ArrowR />
            <Pill>Scorecards</Pill>
          </div>
          <div className="rounded-2xl border border-dashed border-white/[0.08] bg-black/35 p-6">
            <p className="mb-6 text-2xs font-semibold uppercase tracking-wider text-zinc-400">
              Convergent outcomes
            </p>
            <div className="flex flex-col gap-8">
              <div className="flex flex-wrap gap-x-3 gap-y-3">
                <Pill>Reviewer understanding</Pill>
              </div>
              <div className="flex flex-wrap gap-x-3 gap-y-3">
                <Pill>Comparison decisions</Pill>
              </div>
              <ArrowD />
              <RowArrowRow pills={["Future eval-pack improvements"]} />
            </div>
            <p className="mt-6 text-xs leading-relaxed text-white/35">
              Review and comparison tighten the authored packs you ship next, turning observed drifts into explicit benchmark updates.
            </p>
          </div>
        </div>
      </div>
    </DiagramFrame>
  );
}

export function DiagramCodebaseTourShortcuts() {
  return (
    <DiagramFrame caption="Thin slice from web and CLI typed surfaces toward worker-hosted execution">
      <div className="flex flex-col gap-10 md:gap-12">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-6">
          <Pill className="text-white/80">web/</Pill>
          <ArrowR />
          <Pill>backend/internal/api</Pill>
        </div>

        <div className="-mx-2 border-y border-white/14 py-6 md:border-0 md:py-0">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-6">
            <Pill className="text-white/80">CLI</Pill>
            <ArrowR />
            <Pill>backend/internal/api</Pill>
          </div>
        </div>

        <RowArrowRow pills={["backend/internal/api", "Temporal workflows"]} />
        <RowArrowRow pills={["Temporal workflows", "backend/internal/worker"]} />

        <div className="flex flex-col gap-6 border-t border-white/[0.08] pt-8">
          <p className="text-xs font-semibold uppercase tracking-wider text-white/35">
            Worker tails
          </p>
          <div className="flex flex-wrap gap-x-3 gap-y-4">
            <Pill>Sandbox provider</Pill>
            <ArrowR />
            <Pill>Replay events and artifacts</Pill>
          </div>
        </div>
      </div>
    </DiagramFrame>
  );
}

export function DiagramEvalPackBundleShape() {
  return (
    <DiagramFrame caption="Pack versioning ties policy, sandbox, scoring, and cases into runs">
      <div className="mx-auto grid max-w-5xl gap-16">
        <div className="grid gap-x-14 gap-y-12 md:grid-cols-[1fr_auto_1fr] md:items-start">
          <div className="flex flex-col gap-14">
            <div className="flex flex-col gap-14 md:flex-row md:justify-between">
              <div className="flex flex-col items-start gap-2">
                <Pill>Pack metadata</Pill>
              </div>
              <ArrowR className="hidden shrink-0 self-center rotate-[-12deg] md:inline" />
              <div className="flex flex-col items-start gap-2">
                <Pill className="ring-[1px] ring-white/20">Version</Pill>
                <ArrowD />
                <div className="mt-8 flex flex-col gap-3">
                  <Pill>Evaluation spec</Pill>
                  <Pill>Sandbox</Pill>
                  <Pill>Tool policy</Pill>
                  <Pill>Version assets</Pill>
                </div>
              </div>
            </div>
          </div>
          <div className="mx-auto hidden h-px min-h-[18rem] min-w-[2px] self-stretch bg-white/[0.08] md:block md:rounded-full" aria-hidden />
          <div className="flex flex-col items-start gap-3">
            <Pill>Challenges</Pill>
            <ArrowD />
            <Pill>Input sets</Pill>
            <ArrowD />
            <Pill>Cases</Pill>
          </div>
        </div>

        <div className="flex flex-col items-center gap-6 border-t border-white/[0.08] pt-12 md:flex-row md:flex-wrap md:justify-center md:gap-10">
          <RowArrowRow pills={["Cases", "Run execution"]} />
          <span className="hidden text-2xs uppercase tracking-[0.2em] text-zinc-600 md:inline">
            —
          </span>
          <div className="flex flex-col items-center gap-2">
            <Pill>Evaluation spec</Pill>
            <ArrowD />
            <Pill className="ring-[1px] ring-white/20">Scorecards</Pill>
          </div>
        </div>
      </div>
    </DiagramFrame>
  );
}
