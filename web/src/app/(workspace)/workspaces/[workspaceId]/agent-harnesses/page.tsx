import { AgentHarnessesClient } from "./agent-harnesses-client";

export default async function AgentHarnessesPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <AgentHarnessesClient workspaceId={workspaceId} />;
}
