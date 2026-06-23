import { PackBuilder } from "@/components/challenge-packs/pack-builder";

export default async function ChallengePackBuilderPage({
  params,
}: {
  params: Promise<{ workspaceId: string; draftId: string }>;
}) {
  const { workspaceId, draftId } = await params;
  return <PackBuilder workspaceId={workspaceId} draftId={draftId} />;
}
