import { PlaygroundsClient } from "./playgrounds-client";

export default async function PlaygroundsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <PlaygroundsClient workspaceId={workspaceId} />;
}
