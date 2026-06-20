import { ToolEditClient } from "./tool-edit-client";

export default async function EditToolPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string; toolId: string }>;
  searchParams: Promise<{ editor?: string }>;
}) {
  const { workspaceId, toolId } = await params;
  const { editor } = await searchParams;
  return <ToolEditClient workspaceId={workspaceId} toolId={toolId} editor={editor} />;
}
