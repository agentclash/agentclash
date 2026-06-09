import { TryoutDetailClient } from "./tryout-detail-client";

export default async function AgentTryoutDetailPage({
  params,
}: {
  params: Promise<{ workspaceId: string; tryoutId: string }>;
}) {
  const { workspaceId, tryoutId } = await params;
  return <TryoutDetailClient workspaceId={workspaceId} tryoutId={tryoutId} />;
}
