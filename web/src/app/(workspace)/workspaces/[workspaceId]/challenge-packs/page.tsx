import { ChallengePacksClient } from "./challenge-packs-client";

export default async function ChallengePacksPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <ChallengePacksClient workspaceId={workspaceId} />;
}
