import { PackBuilder } from "@/components/eval-packs/pack-builder";

export default async function EvalPackBuilderPage({
  params,
}: {
  params: Promise<{ workspaceId: string; draftId: string }>;
}) {
  const { workspaceId, draftId } = await params;
  return <PackBuilder workspaceId={workspaceId} draftId={draftId} />;
}
