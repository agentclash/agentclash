import { RunsPageClient } from "./runs-page-client";

export default async function RunsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <RunsPageClient workspaceId={workspaceId} />;
}
