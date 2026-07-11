import { ToolEditClient } from "./tool-edit-client";

export default async function EditToolPage({
  params,
}: {
  params: Promise<{ workspaceId: string; toolId: string }>;
}) {
  const { workspaceId, toolId } = await params;
  return <ToolEditClient workspaceId={workspaceId} toolId={toolId} />;
}
