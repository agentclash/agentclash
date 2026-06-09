import { AgentTryoutsClient } from "./agent-tryouts-client";

export default async function AgentTryoutsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <AgentTryoutsClient workspaceId={workspaceId} />;
}
