import { KnowledgeSourcesClient } from "./knowledge-sources-client";

export default async function KnowledgeSourcesPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <KnowledgeSourcesClient workspaceId={workspaceId} />;
}
