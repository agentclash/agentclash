import { ToolBuilder } from "@/components/tools/tool-builder";
import { ToolStartChooser } from "@/components/tools/tool-start-chooser";
import { isBuilderToolType } from "@/components/tools/lib/definition";

export default async function NewToolPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string }>;
  searchParams: Promise<{ type?: string; start?: string }>;
}) {
  const { workspaceId } = await params;
  const { type, start } = await searchParams;

  // No (or unknown) type yet → let the user pick a plain-language starting point.
  if (!isBuilderToolType(type ?? "")) {
    return <ToolStartChooser workspaceId={workspaceId} />;
  }

  return (
    <ToolBuilder
      workspaceId={workspaceId}
      mode="create"
      toolType={type as "primitive" | "composed"}
      start={start}
    />
  );
}
