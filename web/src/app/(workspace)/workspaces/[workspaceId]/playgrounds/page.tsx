import { withAuth } from "@workos-inc/authkit-nextjs";
import { redirect } from "next/navigation";
import { createApiClient } from "@/lib/api/client";
import type { Playground } from "@/lib/api/types";
import { PlaygroundsClient } from "./playgrounds-client";

export default async function PlaygroundsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { accessToken } = await withAuth();
  if (!accessToken) redirect("/auth/login");

  const { workspaceId } = await params;
  const api = createApiClient(accessToken);
  const { items: playgrounds } = await api.get<{ items: Playground[] }>(
    `/v1/workspaces/${workspaceId}/playgrounds`,
  );

  return (
    <PlaygroundsClient workspaceId={workspaceId} initialPlaygrounds={playgrounds} />
  );
}
