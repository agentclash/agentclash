import { ToolBuilder } from "@/components/tools/tool-builder";
import { isBuilderToolType } from "@/components/tools/lib/definition";

export default async function NewToolPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string }>;
  searchParams: Promise<{ type?: string }>;
}) {
  const { workspaceId } = await params;
  const { type } = await searchParams;
  const toolType = isBuilderToolType(type ?? "") ? (type as "primitive" | "composed") : "primitive";

  return <ToolBuilder workspaceId={workspaceId} mode="create" toolType={toolType} />;
}
