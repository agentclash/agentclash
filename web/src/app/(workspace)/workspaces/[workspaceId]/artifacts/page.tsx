import { ArtifactsClient } from "./artifacts-client";

export default async function ArtifactsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <ArtifactsClient workspaceId={workspaceId} />;
}
