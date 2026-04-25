import { ToolsClient } from "./tools-client";

export default async function ToolsPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <ToolsClient workspaceId={workspaceId} />;
}
