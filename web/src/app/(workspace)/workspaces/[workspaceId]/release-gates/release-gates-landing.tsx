"use client";

import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";

export function ReleaseGatesLanding({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const router = useRouter();

  return (
    <Button
      variant="outline"
      size="sm"
      className="mt-4"
      onClick={() => router.push(`/workspaces/${workspaceId}/runs`)}
    >
      Go to Runs
    </Button>
  );
}
