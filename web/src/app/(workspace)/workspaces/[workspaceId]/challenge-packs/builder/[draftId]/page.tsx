import { PackBuilder } from "@/components/challenge-packs/pack-builder";

export default async function ChallengePackBuilderPage({
  params, searchParams,
}: {
  searchParams: Promise<{return_to?:string;return_title?:string}>;
  params: Promise<{ workspaceId: string; draftId: string }>;
}) {
  const { workspaceId, draftId } = await params;
  const query=await searchParams;
  return <PackBuilder workspaceId={workspaceId} draftId={draftId} returnTo={query.return_to} returnTitle={query.return_title} />;
}
