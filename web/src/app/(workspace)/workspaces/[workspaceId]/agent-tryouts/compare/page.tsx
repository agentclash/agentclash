import { Suspense } from "react";

import { CompareTryoutsClient } from "./compare-tryouts-client";

export default async function CompareAgentTryoutsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return (
    // useSearchParams in the client requires a Suspense boundary at build time.
    <Suspense>
      <CompareTryoutsClient workspaceId={workspaceId} />
    </Suspense>
  );
}
