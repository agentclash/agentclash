import { ToolBuilder } from "@/components/tools/tool-builder";
import { ToolStartChooser } from "@/components/tools/tool-start-chooser";
import { ToolLibraryGallery } from "@/components/tools/tool-library-gallery";
import { isBuilderToolType } from "@/components/tools/lib/definition";

export default async function NewToolPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string }>;
  searchParams: Promise<{ editor?: string; type?: string; start?: string }>;
}) {
  const { workspaceId } = await params;
  const { editor, type, start } = await searchParams;

  // Classic form editor. Needs a type — show the chooser first.
  if (editor === "form") {
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

  // Default: the tool library — pick a ready-made tool, or choose to build one.
  return <ToolLibraryGallery workspaceId={workspaceId} />;
}
