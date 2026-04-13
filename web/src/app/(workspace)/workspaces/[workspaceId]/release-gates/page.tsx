import { ShieldCheck } from "lucide-react";
import { ReleaseGatesLanding } from "./release-gates-landing";

export default async function ReleaseGatesPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;

  return (
    <div>
      <h1 className="text-lg font-semibold tracking-tight mb-4">
        Release Gates
      </h1>
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <div className="mb-4 text-muted-foreground">
          <ShieldCheck className="size-10" />
        </div>
        <h3 className="text-sm font-medium text-foreground">
          Automated pass/fail decisions
        </h3>
        <p className="mt-1 text-sm text-muted-foreground max-w-sm">
          Release gates evaluate whether a candidate agent is ready to ship by
          checking metric thresholds against a baseline. Compare two runs to
          evaluate a release gate.
        </p>
        <ReleaseGatesLanding workspaceId={workspaceId} />
      </div>
    </div>
  );
}
