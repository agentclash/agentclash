import { LibraryClient } from "./library-client";

export default async function ChallengePackLibraryPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <LibraryClient workspaceId={workspaceId} />;
}
