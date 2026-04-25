import { RuntimeProfilesClient } from "./runtime-profiles-client";

export default async function RuntimeProfilesPage({
  params,
}: {
  params: Promise<{ workspaceId: string }>;
}) {
  const { workspaceId } = await params;
  return <RuntimeProfilesClient workspaceId={workspaceId} />;
}
