import { BuildsClient } from "./builds-client";

export default async function BuildsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <BuildsClient workspaceId={workspaceId} />;
}
