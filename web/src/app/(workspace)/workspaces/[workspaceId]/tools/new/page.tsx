import { ToolBuilder } from "@/components/tools/tool-builder";
import { ToolCanvasBuilder } from "@/components/tools/canvas-builder";
import { ToolStartChooser } from "@/components/tools/tool-start-chooser";
import { ToolLibraryGallery } from "@/components/tools/tool-library-gallery";
import { isBuilderToolType, presetDefinition } from "@/components/tools/lib/definition";

export default async function NewToolPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string }>;
  searchParams: Promise<{ editor?: string; type?: string; start?: string; build?: string }>;
}) {
  const { workspaceId } = await params;
  const { editor, type, start, build } = await searchParams;

  // Classic form editor (fallback). Needs a type — show the chooser first.
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

  // Build-your-own: the unified canvas builder. A quick-start preselects a node.
  if (build === "canvas") {
    const initialDefinition = start ? presetDefinition("primitive", start) : undefined;
    return (
      <ToolCanvasBuilder
        workspaceId={workspaceId}
        mode="create"
        initialName=""
        initialDefinition={initialDefinition}
        formEditorHref={`/workspaces/${workspaceId}/tools/new?editor=form`}
      />
    );
  }

  // Default: the tool library — pick a ready-made tool, or choose to build one.
  return <ToolLibraryGallery workspaceId={workspaceId} />;
}
